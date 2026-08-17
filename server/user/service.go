package user

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"unicode/utf8"

	"smallgo/server/auth"
	"smallgo/server/database"
	"smallgo/server/sysconfig"
	"smallgo/server/utils"

	"gorm.io/gorm"
)

// Sentinel errors — handlers map these to the documented API error codes
// (1001 / 1003 in AGENTS.md). The error message IS the user-facing text.
var (
	ErrRegisterDisabled = errors.New("注册功能已关闭")
	ErrUserExists       = errors.New("用户名已存在")
	ErrPasswordTooShort = errors.New("密码长度不能少于6位")
	ErrPasswordTooLong  = errors.New("密码长度不能超过72字节")
	ErrLastAdmin        = errors.New("不能降级、禁用或删除最后一个管理员")
	ErrUserNotFound     = errors.New("用户不存在")
	ErrFnOSNotBound     = errors.New("此飞牛 NAS 账号尚未绑定应用账号")
	ErrFnOSAlreadyBound = errors.New("此飞牛 NAS 账号已绑定其他应用账号")
)

// FnOSIdentity is the trusted identity context added by the fnOS unified
// gateway. Gateway tokens are deliberately neither persisted nor exposed.
type FnOSIdentity struct {
	UserID   uint
	Username string
	IsAdmin  bool
}

func validatePassword(password string) error {
	if utf8.RuneCountInString(password) < 6 {
		return ErrPasswordTooShort
	}
	if len(password) > 72 {
		return ErrPasswordTooLong
	}
	return nil
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

// verifyStoredPassword accepts both the current plaintext-over-TLS protocol
// and the previous browser-side SHA-256 protocol. A successful legacy match is
// migrated to bcrypt(raw) by callers that can safely update the row.
func verifyStoredPassword(stored, raw, legacyMD5 string) (bool, bool) {
	if auth.VerifyPassword(stored, raw) {
		return true, auth.IsLegacyHash(stored)
	}
	for _, candidate := range []string{sha256Hex(raw), legacyMD5} {
		if candidate != "" && candidate != raw && auth.VerifyPassword(stored, candidate) {
			return true, true
		}
	}
	return false, false
}

func Register(db *gorm.DB, username string, password string, jwtSecret string) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := db.Transaction(func(tx *gorm.DB) error {
		var userCount int64
		if err := tx.Model(&database.User{}).Count(&userCount).Error; err != nil {
			return err
		}

		if userCount > 0 {
			allowRegister, err := sysconfig.GetConfig(tx, "allow_register", 0)
			if err != nil {
				return err
			}
			if allowRegister != "true" {
				return ErrRegisterDisabled
			}
		}

		var existing database.User
		if err := tx.Where("username = ?", username).First(&existing).Error; err == nil {
			return ErrUserExists
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		if err := validatePassword(password); err != nil {
			return err
		}
		hashedPassword, err := auth.HashPassword(password)
		if err != nil {
			return err
		}

		role := "user"
		if userCount == 0 {
			role = "admin"
		}

		user := database.User{
			Username:    username,
			Password:    hashedPassword,
			Role:        role,
			Status:      1,
			APIKey:      auth.GenerateAPIKey(),
			AuthVersion: 1,
		}
		if err := tx.Create(&user).Error; err != nil {
			return err
		}

		token, err := auth.GenerateToken(user.ID, user.Username, user.Role, user.AuthVersion, jwtSecret)
		if err != nil {
			return err
		}

		result = map[string]interface{}{
			"token": token,
			"user": map[string]interface{}{
				"id":       user.ID,
				"username": user.Username,
				"role":     user.Role,
			},
		}
		return nil
	})
	return result, err
}

func Login(db *gorm.DB, username string, password string, passwordMd5 string, jwtSecret string) (map[string]interface{}, error) {
	var user database.User
	if err := db.Where("username = ?", username).First(&user).Error; err != nil {
		return nil, fmt.Errorf("用户名或密码错误")
	}

	if user.Status != 1 {
		// Generic error — masking disabled accounts as "wrong credentials"
		// prevents username enumeration via response inspection.
		return nil, fmt.Errorf("用户名或密码错误")
	}

	verified, needsRehash := verifyStoredPassword(user.Password, password, passwordMd5)
	if !verified {
		return nil, fmt.Errorf("用户名或密码错误")
	}
	if user.AuthVersion == 0 {
		user.AuthVersion = 1
		if err := db.Model(&user).Update("auth_version", user.AuthVersion).Error; err != nil {
			return nil, err
		}
	}
	if needsRehash {
		hashedPassword, err := auth.HashPassword(password)
		if err != nil {
			return nil, err
		}
		if err := db.Model(&user).Update("password", hashedPassword).Error; err != nil {
			return nil, err
		}
	}

	token, err := auth.GenerateToken(user.ID, user.Username, user.Role, user.AuthVersion, jwtSecret)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"token": token,
		"user": map[string]interface{}{
			"id":       user.ID,
			"username": user.Username,
			"role":     user.Role,
		},
	}, nil
}

// LoginWithFnOS signs in the application account bound to the gateway's
// NAS-local UID. The identity must only come from the fnOS Unix-socket gateway.
func LoginWithFnOS(db *gorm.DB, identity FnOSIdentity, jwtSecret string) (map[string]interface{}, error) {
	var user database.User
	if err := db.Where("fn_os_user_id = ?", identity.UserID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFnOSNotBound
		}
		return nil, err
	}
	if user.Status != 1 {
		return nil, fmt.Errorf("账号已停用")
	}
	if user.AuthVersion == 0 {
		user.AuthVersion = 1
		if err := db.Model(&user).Update("auth_version", user.AuthVersion).Error; err != nil {
			return nil, err
		}
	}
	if user.FnOSUsername != identity.Username {
		if err := db.Model(&user).Update("fn_os_username", identity.Username).Error; err != nil {
			return nil, err
		}
	}
	return loginResult(user, jwtSecret)
}

// BindFnOSAccount either creates a new application account or verifies an
// existing account, then atomically binds it to the current fnOS UID.
func BindFnOSAccount(db *gorm.DB, identity FnOSIdentity, username, password, mode, jwtSecret string) (map[string]interface{}, error) {
	if identity.UserID == 0 || identity.Username == "" {
		return nil, fmt.Errorf("飞牛登录信息无效")
	}

	var result map[string]interface{}
	err := db.Transaction(func(tx *gorm.DB) error {
		var linked database.User
		if err := tx.Where("fn_os_user_id = ?", identity.UserID).First(&linked).Error; err == nil {
			return ErrFnOSAlreadyBound
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		var user database.User
		switch mode {
		case "register":
			var userCount int64
			if err := tx.Model(&database.User{}).Count(&userCount).Error; err != nil {
				return err
			}
			if userCount > 0 {
				allowRegister, err := sysconfig.GetConfig(tx, "allow_register", 0)
				if err != nil {
					return err
				}
				if allowRegister != "true" {
					return ErrRegisterDisabled
				}
			}
			if err := validatePassword(password); err != nil {
				return err
			}
			if err := tx.Where("username = ?", username).First(&user).Error; err == nil {
				return ErrUserExists
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			hashedPassword, err := auth.HashPassword(password)
			if err != nil {
				return err
			}
			role := "user"
			if userCount == 0 {
				role = "admin"
			}
			user = database.User{Username: username, Password: hashedPassword, Role: role, Status: 1, APIKey: auth.GenerateAPIKey(), AuthVersion: 1}
			if err := tx.Create(&user).Error; err != nil {
				return err
			}
		case "bind":
			if err := tx.Where("username = ?", username).First(&user).Error; err != nil || user.Status != 1 {
				return fmt.Errorf("用户名或密码错误")
			}
			verified, needsRehash := verifyStoredPassword(user.Password, password, "")
			if !verified {
				return fmt.Errorf("用户名或密码错误")
			}
			if needsRehash {
				hashedPassword, err := auth.HashPassword(password)
				if err != nil {
					return err
				}
				if err := tx.Model(&user).Update("password", hashedPassword).Error; err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("无效的绑定方式")
		}

		if user.FnOSUserID != nil && *user.FnOSUserID != identity.UserID {
			return fmt.Errorf("该应用账号已绑定其他飞牛 NAS 账号")
		}
		updates := map[string]interface{}{
			"fn_os_user_id":  identity.UserID,
			"fn_os_username": identity.Username,
		}
		if user.AuthVersion == 0 {
			user.AuthVersion = 1
			updates["auth_version"] = user.AuthVersion
		}
		if err := tx.Model(&user).Updates(updates).Error; err != nil {
			return err
		}
		user.FnOSUserID = &identity.UserID
		user.FnOSUsername = identity.Username
		var err error
		result, err = loginResult(user, jwtSecret)
		return err
	})
	return result, err
}

func loginResult(user database.User, jwtSecret string) (map[string]interface{}, error) {
	token, err := auth.GenerateToken(user.ID, user.Username, user.Role, user.AuthVersion, jwtSecret)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"token": token,
		"user":  map[string]interface{}{"id": user.ID, "username": user.Username, "role": user.Role},
	}, nil
}

func ChangePassword(db *gorm.DB, userID uint, oldPassword string, oldPasswordMd5 string, newPassword string) error {
	if err := validatePassword(newPassword); err != nil {
		return err
	}

	var user database.User
	if err := db.First(&user, userID).Error; err != nil {
		return fmt.Errorf("用户不存在")
	}

	verified, _ := verifyStoredPassword(user.Password, oldPassword, oldPasswordMd5)
	if !verified {
		return fmt.Errorf("原密码不正确")
	}
	hashedPassword, err := auth.HashPassword(newPassword)
	if err != nil {
		return err
	}
	nextVersion := user.AuthVersion + 1
	return db.Model(&user).Updates(map[string]interface{}{
		"password":     hashedPassword,
		"auth_version": nextVersion,
	}).Error
}

// UserResponse is the admin user-list payload. APIKey is intentionally
// omitted — bearer tokens must not be enumerable via the list endpoint.
// Owners read their own key via GET /api/auth/me.
type UserResponse struct {
	ID        uint   `json:"id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	Status    int    `json:"status"`
	CreatedAt string `json:"created_at"`
}

func GetAllUsers(db *gorm.DB, page int, pageSize int, search string) ([]UserResponse, int64, error) {
	page, pageSize = utils.NormalizePage(page, pageSize)

	query := db.Model(&database.User{})
	if search != "" {
		query = query.Where("username LIKE ?", "%"+search+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var users []database.User
	if err := query.Order("id ASC").Offset(utils.Offset(page, pageSize)).Limit(pageSize).Find(&users).Error; err != nil {
		return nil, 0, err
	}

	userResponses := make([]UserResponse, 0, len(users))
	for _, u := range users {
		userResponses = append(userResponses, UserResponse{
			ID:        u.ID,
			Username:  u.Username,
			Role:      u.Role,
			Status:    u.Status,
			CreatedAt: u.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return userResponses, total, nil
}

// UpdateUser updates only the fields an admin is allowed to change (role and
// status). Sensitive columns (password, api_key, username) are intentionally
// not updatable here to avoid mass-assignment; use the dedicated endpoints.
func UpdateUser(db *gorm.DB, userID uint, role *string, status *int) error {
	updates := map[string]interface{}{}
	if role != nil {
		if *role != "admin" && *role != "user" {
			return fmt.Errorf("无效的角色")
		}
		updates["role"] = *role
	}
	if status != nil {
		if *status != 0 && *status != 1 {
			return fmt.Errorf("无效的状态")
		}
		updates["status"] = *status
	}
	if len(updates) == 0 {
		return fmt.Errorf("请提供要更新的角色或状态")
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var target database.User
		if err := tx.First(&target, userID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrUserNotFound
			}
			return err
		}
		if err := ensureAdminRemains(tx, target, role, status); err != nil {
			return err
		}
		return tx.Model(&target).Updates(updates).Error
	})
}

func ensureAdminRemains(db *gorm.DB, target database.User, newRole *string, newStatus *int) error {
	wasActiveAdmin := target.Role == "admin" && target.Status == 1
	if !wasActiveAdmin {
		return nil
	}
	stillAdmin := newRole == nil || *newRole == "admin"
	stillActive := newStatus == nil || *newStatus == 1
	if stillAdmin && stillActive {
		return nil
	}
	var count int64
	if err := db.Model(&database.User{}).Where("role = ? AND status = ?", "admin", 1).Count(&count).Error; err != nil {
		return err
	}
	if count <= 1 {
		return ErrLastAdmin
	}
	return nil
}

func ToggleUserStatus(db *gorm.DB, userID uint) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var target database.User
		if err := tx.First(&target, userID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrUserNotFound
			}
			return err
		}
		newStatus := 1
		if target.Status == 1 {
			newStatus = 0
		}
		if err := ensureAdminRemains(tx, target, nil, &newStatus); err != nil {
			return err
		}
		return tx.Model(&target).Update("status", newStatus).Error
	})
}

func DeleteUser(db *gorm.DB, userID uint) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var target database.User
		if err := tx.First(&target, userID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrUserNotFound
			}
			return err
		}
		disabled := 0
		if err := ensureAdminRemains(tx, target, nil, &disabled); err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&database.SecurityQuestion{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&database.SystemConfig{}).Error; err != nil {
			return err
		}
		return tx.Delete(&target).Error
	})
}

func ResetPassword(db *gorm.DB, userID uint, newPassword string) error {
	if err := validatePassword(newPassword); err != nil {
		return err
	}
	hashedPassword, err := auth.HashPassword(newPassword)
	if err != nil {
		return err
	}
	result := db.Model(&database.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"password":     hashedPassword,
		"auth_version": gorm.Expr("auth_version + 1"),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
}

func ResetAdminPassword(db *gorm.DB) error {
	var user database.User
	if err := db.Where("role = ?", "admin").First(&user).Error; err != nil {
		return fmt.Errorf("未找到管理员账号")
	}

	defaultPassword := "admin123"
	hashedPassword, err := auth.HashPassword(defaultPassword)
	if err != nil {
		return err
	}
	if err := db.Model(&user).Updates(map[string]interface{}{
		"password":     hashedPassword,
		"auth_version": gorm.Expr("auth_version + 1"),
	}).Error; err != nil {
		return err
	}

	fmt.Printf("Admin password has been reset to: %s\n", defaultPassword)
	return nil
}

func CheckAuth(db *gorm.DB, userID uint, requireLogin string) map[string]interface{} {
	result := map[string]interface{}{
		"authenticated": false,
		"require_login": requireLogin,
		"user":          nil,
	}

	if userID > 0 {
		var user database.User
		if err := db.First(&user, userID).Error; err == nil && user.Status == 1 {
			result["authenticated"] = true
			result["user"] = map[string]interface{}{
				"id":       user.ID,
				"username": user.Username,
				"role":     user.Role,
			}
		}
	}

	return result
}

func GetCurrentUser(db *gorm.DB, userID uint) (map[string]interface{}, error) {
	var user database.User
	if err := db.First(&user, userID).Error; err != nil {
		return nil, err
	}

	var sq database.SecurityQuestion
	hasQuestions := db.Where("user_id = ?", userID).First(&sq).Error == nil

	return map[string]interface{}{
		"id":                     user.ID,
		"username":               user.Username,
		"role":                   user.Role,
		"api_key":                user.APIKey,
		"has_security_questions": hasQuestions,
	}, nil
}

func RegenerateAPIKey(db *gorm.DB, userID uint) (string, error) {
	newKey := auth.GenerateAPIKey()
	if err := db.Model(&database.User{}).Where("id = ?", userID).Update("api_key", newKey).Error; err != nil {
		return "", err
	}
	return newKey, nil
}

func getJWTSecret(db *gorm.DB) string {
	var config database.SystemConfig
	if err := db.Where("user_id = ? AND key = ?", 0, "jwt_secret").First(&config).Error; err != nil {
		return ""
	}
	return config.Value
}
