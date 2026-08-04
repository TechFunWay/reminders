package middleware

import (
	"net/http"
	"strings"
	"time"

	"smallgo/server/auth"
	"smallgo/server/database"
	"smallgo/server/logger"
	"smallgo/server/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func LimitJSONBody(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}

func CORS(origin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, X-API-Key")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

func RequireAuth(jwtSecret string, db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if user, ok := authenticate(c, jwtSecret, db); ok {
			setUserContext(c, user)
			c.Next()
			return
		}

		response.ErrorUnauthorized(c, "未授权")
		c.Abort()
	}
}

func OptionalAuth(jwtSecret string, db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if user, ok := authenticate(c, jwtSecret, db); ok {
			setUserContext(c, user)
		}
		c.Next()
	}
}

func authenticate(c *gin.Context, jwtSecret string, db *gorm.DB) (database.User, bool) {
	if apiKey := c.GetHeader("X-API-Key"); apiKey != "" {
		var user database.User
		if err := db.Where("api_key = ? AND status = ?", apiKey, 1).First(&user).Error; err == nil {
			return user, true
		}
	}

	tokenString := ""
	if authHeader := c.GetHeader("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
		tokenString = strings.TrimPrefix(authHeader, "Bearer ")
	} else if cookie, err := c.Cookie("token"); err == nil {
		tokenString = cookie
	}
	if tokenString == "" {
		return database.User{}, false
	}

	userID, _, _, authVersion, err := auth.ParseToken(tokenString, jwtSecret)
	if err != nil {
		return database.User{}, false
	}
	var user database.User
	if err := db.Where("id = ? AND status = ?", userID, 1).First(&user).Error; err != nil {
		return database.User{}, false
	}
	return user, user.AuthVersion == authVersion
}

func setUserContext(c *gin.Context, user database.User) {
	c.Set("userID", user.ID)
	c.Set("username", user.Username)
	c.Set("role", user.Role)
}

func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetString("role") != "admin" {
			response.ErrorForbidden(c, "禁止访问")
			c.Abort()
			return
		}
		c.Next()
	}
}

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		ip := c.ClientIP()

		logger.Info("%3d | %13v | %15s | %-7s %s", status, latency, ip, method, path)
	}
}
