package middleware

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"smallgo/server/auth"
	"smallgo/server/database"
	"smallgo/server/logger"
	"smallgo/server/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type fnOSGatewayContextKey struct{}

// MarkFnOSGateway marks a request connection that arrived through the fnOS
// unified Unix-socket gateway. TCP clients can send the same header names, so
// the injected identity headers are only trusted when this marker was applied
// by server.Start's ConnContext.
func MarkFnOSGateway(ctx context.Context) context.Context {
	return context.WithValue(ctx, fnOSGatewayContextKey{}, true)
}

// OnFnOSGateway reports whether ConnContext marked this request as arriving
// through the fnOS unified gateway socket.
func OnFnOSGateway(ctx context.Context) bool {
	return ctx.Value(fnOSGatewayContextKey{}) == true
}

// GatewayIdentityUser 返回「仅凭网关注入的 NAS 身份」解析出的应用账号，即不需要
// 任何应用自己的凭证就成立的隐式登录态。它同时被两处使用，语义必须一致：
// authenticate 用它充当登录态，handleBootstrap 用它区分当前会话属于网关还是
// 属于应用自己。用户主动登出后（suppressed）这里不再返回账号。
func GatewayIdentityUser(r *http.Request, db *gorm.DB) (database.User, bool) {
	if r == nil || db == nil {
		return database.User{}, false
	}
	if !OnFnOSGateway(r.Context()) {
		return database.User{}, false
	}
	return userFromGatewayIdentityHeader(r, db)
}

func userFromGatewayIdentityHeader(r *http.Request, db *gorm.DB) (database.User, bool) {
	raw := strings.TrimSpace(r.Header.Get("X-Trim-Userid"))
	if raw == "" {
		return database.User{}, false
	}
	nasUserID, err := strconv.ParseUint(raw, 10, 32)
	if err != nil || nasUserID == 0 {
		return database.User{}, false
	}
	var user database.User
	if err := db.Where("fn_os_user_id = ? AND status = ?", uint(nasUserID), 1).First(&user).Error; err != nil {
		return database.User{}, false
	}
	if appSessionSuppressed(db, user.ID) {
		return database.User{}, false
	}
	return user, true
}

// appSessionSuppressed 读 user_sessions.suppressed。查询失败按「未抑制」处理：
// 一次数据库抖动不该把所有网关用户挡在登录页外。读写都只碰 database 包，
// 避免 middleware → user 的循环依赖。
func appSessionSuppressed(db *gorm.DB, userID uint) bool {
	var session database.UserSession
	if err := db.Where("user_id = ?", userID).First(&session).Error; err != nil {
		return false
	}
	return session.Suppressed
}

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
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, X-API-Key, X-Client-Id, X-FnOS-Ticket, X-Skip-Auth-Redirect")
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
	if tokenString != "" {
		if userID, _, _, authVersion, err := auth.ParseToken(tokenString, jwtSecret); err == nil {
			var user database.User
			if err := db.Where("id = ? AND status = ?", userID, 1).First(&user).Error; err == nil && user.AuthVersion == authVersion {
				return user, true
			}
		}
	}

	// 飞牛统一网关：网关在转发前已经校验过 NAS 会话，并按官方约定注入
	// X-Trim-Userid / X-Trim-Username。**应用自己的 Authorization 在网关域上不
	// 保证能到达应用**——接入层会把 Authorization 当成它自己的会话 token，认不出
	// 就直接回 200 "invalid token"（请求压根进不了应用），所以网关域以「该 NAS
	// 用户已绑定的应用账号」作为登录态，这正是飞牛文档给统一网关应用的身份通道。
	// 直连端口（局域网直连、旧书签）拿不到网关头，仍然只认自己的 JWT。
	//
	// 但隐式登录态只在用户没有主动登出时成立：应用自己的登录/退出归应用管，
	// 飞牛授权只是身份来源（见 user_sessions 表与 GatewayIdentityUser）。
	if user, ok := GatewayIdentityUser(c.Request, db); ok {
		return user, true
	}

	return database.User{}, false
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
