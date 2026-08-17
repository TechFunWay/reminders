package user

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"smallgo/server/audit"
	"smallgo/server/database"
	"smallgo/server/response"
	"smallgo/server/sysconfig"
	"smallgo/server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type fnOSGatewayContextKey struct{}

// MarkFnOSGateway marks a request connection that arrived through the fnOS
// Unix-socket gateway. TCP clients can send the same header names, so headers
// are only trusted when this marker was applied by server.Start's ConnContext.
func MarkFnOSGateway(ctx context.Context) context.Context {
	return context.WithValue(ctx, fnOSGatewayContextKey{}, true)
}

func parseUserID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		response.ErrorBadRequest(c, "无效的用户 ID")
		return 0, false
	}
	return uint(id), true
}

func handleSetupRequired(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var count int64
		if err := db.Model(&database.User{}).Count(&count).Error; err != nil {
			response.ErrorInternal(c, "检查初始化状态失败")
			return
		}
		response.Success(c, map[string]interface{}{
			"setup_required": count == 0,
		})
	}
}

func handleRegister(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Username string `json:"username" binding:"required"`
			Password string `json:"password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.ErrorBadRequest(c, "请输入用户名和密码")
			return
		}

		secret := getJWTSecret(db)
		result, err := Register(db, req.Username, req.Password, secret)
		if err != nil {
			switch {
			case errors.Is(err, ErrRegisterDisabled):
				response.Error(c, http.StatusForbidden, response.CodeRegisterDisabled, err.Error())
			case errors.Is(err, ErrUserExists):
				response.Error(c, http.StatusConflict, response.CodeUserExists, err.Error())
			case errors.Is(err, ErrPasswordTooShort), errors.Is(err, ErrPasswordTooLong):
				response.ErrorBadRequest(c, err.Error())
			default:
				response.ErrorInternal(c, "注册失败")
			}
			return
		}

		response.Success(c, result)
	}
}

func handleLogin(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Username    string `json:"username" binding:"required"`
			Password    string `json:"password" binding:"required"`
			PasswordMd5 string `json:"password_md5"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.ErrorBadRequest(c, "请输入用户名和密码")
			return
		}

		secret := getJWTSecret(db)
		result, err := Login(db, req.Username, req.Password, req.PasswordMd5, secret)
		if err != nil {
			response.Error(c, http.StatusUnauthorized, response.CodeInvalidCredentials, err.Error())
			return
		}

		// Populate context so the audit entry is attributed to the logged-in user.
		if u, ok := result["user"].(map[string]interface{}); ok {
			if uid, ok := u["id"].(uint); ok {
				c.Set("userID", uid)
				c.Set("username", req.Username)
			}
		}
		audit.Log(db, c, "login", "user", c.GetUint("userID"), "用户登录")

		response.Success(c, result)
	}
}

func fnOSIdentity(c *gin.Context) (FnOSIdentity, bool) {
	if c.Request.Context().Value(fnOSGatewayContextKey{}) != true {
		response.ErrorUnauthorized(c, "请从飞牛桌面中的应用入口使用一键登录")
		return FnOSIdentity{}, false
	}
	uid, err := strconv.ParseUint(c.GetHeader("X-Trim-Userid"), 10, 32)
	username := c.GetHeader("X-Trim-Username")
	if err != nil || uid == 0 || username == "" {
		response.ErrorUnauthorized(c, "未获取到飞牛 NAS 登录信息")
		return FnOSIdentity{}, false
	}
	return FnOSIdentity{
		UserID:   uint(uid),
		Username: username,
		IsAdmin:  c.GetHeader("X-Trim-Isadmin") == "true",
	}, true
}

func setLoginAuditContext(c *gin.Context, result map[string]interface{}) {
	user, ok := result["user"].(map[string]interface{})
	if !ok {
		return
	}
	if uid, ok := user["id"].(uint); ok {
		c.Set("userID", uid)
	}
	if username, ok := user["username"].(string); ok {
		c.Set("username", username)
	}
}

func handleFnOSIdentity() gin.HandlerFunc {
	return func(c *gin.Context) {
		identity, ok := fnOSIdentity(c)
		if !ok {
			return
		}
		response.Success(c, map[string]interface{}{
			"fnos_username": identity.Username,
		})
	}
}

func handleFnOSLogin(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		identity, ok := fnOSIdentity(c)
		if !ok {
			return
		}
		result, err := LoginWithFnOS(db, identity, getJWTSecret(db))
		if errors.Is(err, ErrFnOSNotBound) {
			var accountCount int64
			if countErr := db.Model(&database.User{}).Count(&accountCount).Error; countErr != nil {
				response.ErrorInternal(c, "检查应用账号状态失败")
				return
			}
			var matchingAccounts int64
			if countErr := db.Model(&database.User{}).Where("username = ?", identity.Username).Count(&matchingAccounts).Error; countErr != nil {
				response.ErrorInternal(c, "检查飞牛账号绑定状态失败")
				return
			}
			suggestedMode := "register"
			if accountCount > 0 {
				suggestedMode = "bind"
			}
			suggestedUsername := ""
			if matchingAccounts > 0 {
				suggestedUsername = identity.Username
			}
			response.Success(c, map[string]interface{}{
				"binding_required":   true,
				"fnos_username":      identity.Username,
				"has_accounts":       accountCount > 0,
				"suggested_mode":     suggestedMode,
				"suggested_username": suggestedUsername,
			})
			return
		}
		if err != nil {
			response.Error(c, http.StatusUnauthorized, response.CodeInvalidCredentials, "飞牛一键登录失败")
			return
		}
		setLoginAuditContext(c, result)
		audit.Log(db, c, "fnos_login", "user", c.GetUint("userID"), "飞牛 NAS 一键登录")
		response.Success(c, result)
	}
}

func handleFnOSBind(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		identity, ok := fnOSIdentity(c)
		if !ok {
			return
		}
		var req struct {
			Mode     string `json:"mode" binding:"required"`
			Username string `json:"username" binding:"required"`
			Password string `json:"password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.ErrorBadRequest(c, "请输入应用账号和密码")
			return
		}
		result, err := BindFnOSAccount(db, identity, req.Username, req.Password, req.Mode, getJWTSecret(db))
		if err != nil {
			switch {
			case errors.Is(err, ErrRegisterDisabled):
				response.Error(c, http.StatusForbidden, response.CodeRegisterDisabled, err.Error())
			case errors.Is(err, ErrUserExists), errors.Is(err, ErrFnOSAlreadyBound):
				response.Error(c, http.StatusConflict, response.CodeUserExists, err.Error())
			case errors.Is(err, ErrPasswordTooShort), errors.Is(err, ErrPasswordTooLong):
				response.ErrorBadRequest(c, err.Error())
			default:
				response.ErrorBadRequest(c, err.Error())
			}
			return
		}
		setLoginAuditContext(c, result)
		audit.Log(db, c, "fnos_bind", "user", c.GetUint("userID"), "绑定飞牛 NAS 账号")
		response.Success(c, result)
	}
}

func handleCheckAuth(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		requireLogin, _ := sysconfig.GetConfig(db, "require_login", 0)

		userID, _ := c.Get("userID")
		uid, _ := userID.(uint)

		result := CheckAuth(db, uid, requireLogin)
		response.Success(c, result)
	}
}

func handleGetCurrentUser(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("userID")
		uid, _ := userID.(uint)

		result, err := GetCurrentUser(db, uid)
		if err != nil {
			response.ErrorInternal(c, "获取用户信息失败")
			return
		}

		response.Success(c, result)
	}
}

func handleChangePassword(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			OldPassword    string `json:"old_password" binding:"required"`
			OldPasswordMd5 string `json:"old_password_md5"`
			NewPassword    string `json:"new_password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.ErrorBadRequest(c, "请输入旧密码和新密码")
			return
		}

		userID, _ := c.Get("userID")
		uid, _ := userID.(uint)

		if err := ChangePassword(db, uid, req.OldPassword, req.OldPasswordMd5, req.NewPassword); err != nil {
			response.ErrorBadRequest(c, err.Error())
			return
		}

		response.Success(c, nil)
	}
}

func handleRegenerateAPIKey(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("userID")
		uid, _ := userID.(uint)

		newKey, err := RegenerateAPIKey(db, uid)
		if err != nil {
			response.ErrorInternal(c, "重新生成 API Key 失败")
			return
		}

		response.Success(c, map[string]string{"api_key": newKey})
	}
}

func handleGetAllUsers(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		page := utils.Atoi(c.Query("page"), 1)
		pageSize := utils.Atoi(c.Query("pageSize"), utils.DefaultPageSize)
		search := c.Query("search")

		users, total, err := GetAllUsers(db, page, pageSize, search)
		if err != nil {
			response.ErrorInternal(c, "获取用户列表失败")
			return
		}

		page, pageSize = utils.NormalizePage(page, pageSize)
		response.SuccessPage(c, users, total, page, pageSize)
	}
}

func handleUpdateUser(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Role   *string `json:"role"`
			Status *int    `json:"status"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.ErrorBadRequest(c, "请求无效")
			return
		}

		userID, ok := parseUserID(c)
		if !ok {
			return
		}

		if err := UpdateUser(db, userID, req.Role, req.Status); err != nil {
			if errors.Is(err, ErrUserNotFound) {
				response.Error(c, http.StatusNotFound, response.CodeNotFound, err.Error())
			} else {
				response.ErrorBadRequest(c, err.Error())
			}
			return
		}

		audit.Log(db, c, "user_update", "user", userID, "管理员修改用户信息")
		response.Success(c, nil)
	}
}

func handleDeleteUser(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := parseUserID(c)
		if !ok {
			return
		}
		if userID == c.GetUint("userID") {
			response.ErrorBadRequest(c, "不能删除自己的账号")
			return
		}
		if err := DeleteUser(db, userID); err != nil {
			switch {
			case errors.Is(err, ErrUserNotFound):
				response.Error(c, http.StatusNotFound, response.CodeNotFound, err.Error())
			case errors.Is(err, ErrLastAdmin):
				response.ErrorBadRequest(c, err.Error())
			default:
				response.ErrorInternal(c, "删除用户失败")
			}
			return
		}

		audit.Log(db, c, "user_delete", "user", userID, "管理员删除用户")
		response.Success(c, nil)
	}
}

func handleToggleStatus(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := parseUserID(c)
		if !ok {
			return
		}

		if err := ToggleUserStatus(db, userID); err != nil {
			switch {
			case errors.Is(err, ErrUserNotFound):
				response.Error(c, http.StatusNotFound, response.CodeNotFound, err.Error())
			case errors.Is(err, ErrLastAdmin):
				response.ErrorBadRequest(c, err.Error())
			default:
				response.ErrorInternal(c, "切换用户状态失败")
			}
			return
		}

		audit.Log(db, c, "user_status", "user", userID, "管理员切换用户状态")
		response.Success(c, nil)
	}
}

func handleResetPassword(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			NewPassword string `json:"new_password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.ErrorBadRequest(c, "请输入新密码")
			return
		}

		userID, ok := parseUserID(c)
		if !ok {
			return
		}
		if err := ResetPassword(db, userID, req.NewPassword); err != nil {
			if errors.Is(err, ErrPasswordTooShort) || errors.Is(err, ErrPasswordTooLong) {
				response.ErrorBadRequest(c, err.Error())
			} else if errors.Is(err, ErrUserNotFound) {
				response.Error(c, http.StatusNotFound, response.CodeNotFound, err.Error())
			} else {
				response.ErrorInternal(c, "重置密码失败")
			}
			return
		}

		audit.Log(db, c, "password_reset", "user", userID, "管理员重置用户密码")
		response.Success(c, nil)
	}
}

// RegisterRoutes wires user and authentication routes.
func RegisterRoutes(publicGroup *gin.RouterGroup, optionalAuthGroup *gin.RouterGroup, authGroup *gin.RouterGroup, adminGroup *gin.RouterGroup, db *gorm.DB, fnOSApp bool) {
	publicGroup.GET("/auth/setup-required", handleSetupRequired(db))
	publicGroup.POST("/auth/register", handleRegister(db))
	publicGroup.POST("/auth/login", handleLogin(db))
	if fnOSApp {
		publicGroup.GET("/auth/fnos/identity", handleFnOSIdentity())
		publicGroup.POST("/auth/fnos/login", handleFnOSLogin(db))
		publicGroup.POST("/auth/fnos/bind", handleFnOSBind(db))
	}

	optionalAuthGroup.GET("/auth/check", handleCheckAuth(db))

	authGroup.GET("/auth/me", handleGetCurrentUser(db))
	authGroup.PUT("/auth/password", handleChangePassword(db))
	authGroup.POST("/auth/apikey", handleRegenerateAPIKey(db))

	adminGroup.GET("/users", handleGetAllUsers(db))
	adminGroup.PUT("/users/:id", handleUpdateUser(db))
	adminGroup.DELETE("/users/:id", handleDeleteUser(db))
	adminGroup.PUT("/users/:id/status", handleToggleStatus(db))
	adminGroup.PUT("/users/:id/password", handleResetPassword(db))
}
