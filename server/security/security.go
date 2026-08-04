package security

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"unicode/utf8"

	"smallgo/server/auth"
	"smallgo/server/database"
	"smallgo/server/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var ErrAnswerTooLong = errors.New("安全问题答案不能超过72字节")

func SetSecurityQuestions(db *gorm.DB, userID uint, q1, a1, q2, a2, q3, a3 string) error {
	var sq database.SecurityQuestion
	result := db.Where("user_id = ?", userID).First(&sq)

	hashedA1, err := auth.HashPassword(a1)
	if err != nil {
		return ErrAnswerTooLong
	}
	hashedA2, err := auth.HashPassword(a2)
	if err != nil {
		return ErrAnswerTooLong
	}
	hashedA3, err := auth.HashPassword(a3)
	if err != nil {
		return ErrAnswerTooLong
	}

	if result.Error == gorm.ErrRecordNotFound {
		sq = database.SecurityQuestion{
			UserID:    userID,
			Question1: q1,
			Answer1:   hashedA1,
			Question2: q2,
			Answer2:   hashedA2,
			Question3: q3,
			Answer3:   hashedA3,
		}
		return db.Create(&sq).Error
	}

	if result.Error != nil {
		return result.Error
	}

	sq.Question1 = q1
	sq.Answer1 = hashedA1
	sq.Question2 = q2
	sq.Answer2 = hashedA2
	sq.Question3 = q3
	sq.Answer3 = hashedA3
	return db.Save(&sq).Error
}

func GetSecurityQuestions(db *gorm.DB, userID uint) (map[string]interface{}, error) {
	var sq database.SecurityQuestion
	result := db.Where("user_id = ?", userID).First(&sq)
	if result.Error == gorm.ErrRecordNotFound {
		return map[string]interface{}{
			"question1":     "",
			"question2":     "",
			"question3":     "",
			"has_questions": false,
		}, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return map[string]interface{}{
		"question1":     sq.Question1,
		"question2":     sq.Question2,
		"question3":     sq.Question3,
		"has_questions": true,
	}, nil
}

func GetSecurityQuestionsByUsername(db *gorm.DB, username string) (map[string]interface{}, error) {
	var user database.User
	if err := db.Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}

	questions, err := GetSecurityQuestions(db, user.ID)
	if err != nil {
		return nil, err
	}

	questions["user_id"] = user.ID
	return questions, nil
}

func VerifyAnswers(db *gorm.DB, username string, a1, a2, a3 string, a1md5, a2md5, a3md5 string) error {
	var user database.User
	if err := db.Where("username = ?", username).First(&user).Error; err != nil {
		return fmt.Errorf("用户不存在")
	}

	var sq database.SecurityQuestion
	if err := db.Where("user_id = ?", user.ID).First(&sq).Error; err != nil {
		return fmt.Errorf("未设置安全问题")
	}

	ok1, migrate1 := verifyAnswer(sq.Answer1, a1, a1md5)
	ok2, migrate2 := verifyAnswer(sq.Answer2, a2, a2md5)
	ok3, migrate3 := verifyAnswer(sq.Answer3, a3, a3md5)
	if !ok1 || !ok2 || !ok3 {
		return fmt.Errorf("安全问题验证失败")
	}
	if migrate1 || migrate2 || migrate3 {
		hashedA1, err := auth.HashPassword(a1)
		if err != nil {
			return ErrAnswerTooLong
		}
		hashedA2, err := auth.HashPassword(a2)
		if err != nil {
			return ErrAnswerTooLong
		}
		hashedA3, err := auth.HashPassword(a3)
		if err != nil {
			return ErrAnswerTooLong
		}
		if err := db.Model(&sq).Updates(map[string]interface{}{
			"answer1": hashedA1,
			"answer2": hashedA2,
			"answer3": hashedA3,
		}).Error; err != nil {
			return err
		}
	}

	return nil
}

func verifyAnswer(stored, raw, legacyMD5 string) (bool, bool) {
	if auth.VerifyPassword(stored, raw) {
		return true, auth.IsLegacyHash(stored)
	}
	sum := sha256.Sum256([]byte(raw))
	for _, candidate := range []string{hex.EncodeToString(sum[:]), legacyMD5} {
		if candidate != "" && candidate != raw && auth.VerifyPassword(stored, candidate) {
			return true, true
		}
	}
	return false, false
}

func VerifyAndResetPassword(db *gorm.DB, username string, a1, a2, a3 string, a1md5, a2md5, a3md5 string, newPassword string) error {
	if utf8.RuneCountInString(newPassword) < 6 {
		return fmt.Errorf("密码长度不能少于6位")
	}
	if len(newPassword) > 72 {
		return fmt.Errorf("密码长度不能超过72字节")
	}
	if err := VerifyAnswers(db, username, a1, a2, a3, a1md5, a2md5, a3md5); err != nil {
		return err
	}

	var user database.User
	if err := db.Where("username = ?", username).First(&user).Error; err != nil {
		return err
	}

	hashedPassword, err := auth.HashPassword(newPassword)
	if err != nil {
		return err
	}
	return db.Model(&user).Updates(map[string]interface{}{
		"password":     hashedPassword,
		"auth_version": gorm.Expr("auth_version + 1"),
	}).Error
}

func handleGetQuestions(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetUint("userID")

		questions, err := GetSecurityQuestions(db, userID)
		if err != nil {
			response.ErrorInternal(c, "获取安全问题失败")
			return
		}

		response.Success(c, questions)
	}
}

func handleSetQuestions(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetUint("userID")

		var req struct {
			Question1 string `json:"question1" binding:"required"`
			Answer1   string `json:"answer1" binding:"required"`
			Question2 string `json:"question2" binding:"required"`
			Answer2   string `json:"answer2" binding:"required"`
			Question3 string `json:"question3" binding:"required"`
			Answer3   string `json:"answer3" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			response.ErrorBadRequest(c, "请填写所有安全问题和答案")
			return
		}

		if err := SetSecurityQuestions(db, userID, req.Question1, req.Answer1, req.Question2, req.Answer2, req.Question3, req.Answer3); err != nil {
			if errors.Is(err, ErrAnswerTooLong) {
				response.ErrorBadRequest(c, err.Error())
			} else {
				response.ErrorInternal(c, "设置安全问题失败")
			}
			return
		}

		response.Success(c, nil)
	}
}

func handleGetQuestionsByUsername(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Username string `json:"username" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			response.ErrorBadRequest(c, "请输入用户名")
			return
		}

		questions, err := GetSecurityQuestionsByUsername(db, req.Username)
		if err != nil {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "用户不存在")
			return
		}

		response.Success(c, questions)
	}
}

func handleVerifyAnswers(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Username   string `json:"username" binding:"required"`
			Answer1    string `json:"answer1" binding:"required"`
			Answer2    string `json:"answer2" binding:"required"`
			Answer3    string `json:"answer3" binding:"required"`
			Answer1Md5 string `json:"answer1_md5"`
			Answer2Md5 string `json:"answer2_md5"`
			Answer3Md5 string `json:"answer3_md5"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			response.ErrorBadRequest(c, "请填写所有安全问题答案")
			return
		}

		if err := VerifyAnswers(db, req.Username, req.Answer1, req.Answer2, req.Answer3, req.Answer1Md5, req.Answer2Md5, req.Answer3Md5); err != nil {
			response.ErrorBadRequest(c, err.Error())
			return
		}

		response.Success(c, nil)
	}
}

func handleVerifyAndReset(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Username    string `json:"username" binding:"required"`
			Answer1     string `json:"answer1" binding:"required"`
			Answer2     string `json:"answer2" binding:"required"`
			Answer3     string `json:"answer3" binding:"required"`
			Answer1Md5  string `json:"answer1_md5"`
			Answer2Md5  string `json:"answer2_md5"`
			Answer3Md5  string `json:"answer3_md5"`
			NewPassword string `json:"new_password" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			response.ErrorBadRequest(c, "请填写所有字段")
			return
		}

		if err := VerifyAndResetPassword(db, req.Username, req.Answer1, req.Answer2, req.Answer3, req.Answer1Md5, req.Answer2Md5, req.Answer3Md5, req.NewPassword); err != nil {
			response.ErrorBadRequest(c, err.Error())
			return
		}

		response.Success(c, nil)
	}
}

// RegisterRoutes wires security-question endpoints. Managing one's own
// questions requires auth; the forgot-password flow is public but rate limited
// to slow down brute-forcing of answers.
func RegisterRoutes(public *gin.RouterGroup, auth *gin.RouterGroup, db *gorm.DB) {
	auth.GET("/security/questions", handleGetQuestions(db))
	auth.POST("/security/questions", handleSetQuestions(db))
	public.POST("/security/forgot/username", handleGetQuestionsByUsername(db))
	public.POST("/security/forgot/verify", handleVerifyAnswers(db))
	public.POST("/security/forgot/reset", handleVerifyAndReset(db))
}
