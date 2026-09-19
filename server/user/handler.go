package user

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	neturl "net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"smallgo/server/audit"
	"smallgo/server/database"
	"smallgo/server/middleware"
	"smallgo/server/response"
	"smallgo/server/sysconfig"
	"smallgo/server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// MarkFnOSGateway marks a request connection that arrived through the fnOS
// Unix-socket gateway. TCP clients can send the same header names, so headers
// are only trusted when this marker was applied by server.Start's ConnContext.
// The marker lives in middleware so the rate limiter can also key gateway
// traffic per NAS user instead of collapsing every NAS user into one bucket.
func MarkFnOSGateway(ctx context.Context) context.Context {
	return middleware.MarkFnOSGateway(ctx)
}

// fnOSLoginTicket carries a gateway-verified NAS identity to the plain TCP
// listener, where the gateway headers are absent. Tickets are short lived and
// single use so a leaked value cannot be replayed later.
const fnOSTicketTTL = 2 * time.Minute

type fnOSLoginTicket struct {
	Identity  FnOSIdentity
	ExpiresAt time.Time
}

var (
	fnOSTicketsMu sync.Mutex
	fnOSTickets   = make(map[string]fnOSLoginTicket)
)

// 网关跳板页（fnos-entry.html）最近一次被访问时的完整入口地址。跳板页是唯一
// 能确知「本机在飞牛网关下的入口」的一方；把它记在服务端后，任何客户端（尤其
// 手机端首次打开、referrer 为空、localStorage 为空的场景）都能从 /api/version
// 拿到正确入口，而不必猜「主机名 + 服务端口」，也不会拿着过期的旧入口去跳。
// 只接受经网关 socket 到达的登记，TCP 客户端无法伪造。
var (
	fnOSGatewayEntryMu  sync.RWMutex
	fnOSGatewayEntryURL string
)

func setFnOSGatewayEntry(url string) {
	trimmed := strings.TrimSpace(url)
	if trimmed == "" || len(trimmed) > 512 {
		return
	}
	parsed, err := neturl.Parse(trimmed)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || !strings.Contains(parsed.Path, "/app/") {
		return
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	fnOSGatewayEntryMu.Lock()
	fnOSGatewayEntryURL = parsed.String()
	fnOSGatewayEntryMu.Unlock()
}

// GatewayEntry 返回最近的网关跳板页入口；从未访问过时为空字符串。
func GatewayEntry() string {
	fnOSGatewayEntryMu.RLock()
	defer fnOSGatewayEntryMu.RUnlock()
	return fnOSGatewayEntryURL
}

func newFnOSTicketValue() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func storeFnOSTicket(identity FnOSIdentity) (string, error) {
	ticket, err := newFnOSTicketValue()
	if err != nil {
		return "", err
	}
	now := time.Now()
	fnOSTicketsMu.Lock()
	for key, item := range fnOSTickets {
		if !item.ExpiresAt.After(now) {
			delete(fnOSTickets, key)
		}
	}
	fnOSTickets[ticket] = fnOSLoginTicket{Identity: identity, ExpiresAt: now.Add(fnOSTicketTTL)}
	fnOSTicketsMu.Unlock()
	return ticket, nil
}

func consumeFnOSTicket(ticket string) {
	if ticket == "" {
		return
	}
	fnOSTicketsMu.Lock()
	delete(fnOSTickets, ticket)
	fnOSTicketsMu.Unlock()
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
	if !middleware.OnFnOSGateway(c.Request.Context()) {
		// Plain TCP access: the gateway headers cannot be trusted here, so the
		// identity must arrive via a one-time ticket minted on the gateway.
		ticket := c.GetHeader("X-FnOS-Ticket")
		if ticket == "" {
			response.ErrorUnauthorized(c, "请从飞牛桌面中的应用入口使用一键登录")
			return FnOSIdentity{}, false
		}
		fnOSTicketsMu.Lock()
		item, ok := fnOSTickets[ticket]
		if ok && !item.ExpiresAt.After(time.Now()) {
			delete(fnOSTickets, ticket)
			ok = false
		}
		fnOSTicketsMu.Unlock()
		if !ok {
			response.ErrorUnauthorized(c, "飞牛登录凭证无效或已过期，请重新点击飞牛授权登录")
			return FnOSIdentity{}, false
		}
		return item.Identity, true
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
		consumeFnOSTicket(c.GetHeader("X-FnOS-Ticket"))
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
		consumeFnOSTicket(c.GetHeader("X-FnOS-Ticket"))
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

// handleLogout 结束**应用自己的**登录态。
//
// 直连端口上会话就是应用签发的 JWT，前端清掉就结束，服务端只需把这个动作
// 记进审计；网关域上光清前端没用——服务端每个请求都能从网关注入的 NAS 身份
// 重新认出应用账号，所以必须落一条持久化抑制标记，告诉网关隐式认人「这位
// 用户主动登出了，别再自动放行」。两种环境返回同样的结果，前端不必分支。
//
// 这里刻意用 OptionalAuth：退出必须幂等，重复点击或会话已失效也应返回成功。
func handleLogout(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("userID")
		uid, _ := userID.(uint)
		if err := LogoutApplicationSession(db, uid); err != nil {
			response.ErrorInternal(c, "退出登录失败，请稍后重试")
			return
		}
		if uid != 0 {
			audit.Log(db, c, "logout", "user", uid, "用户退出登录")
		}
		response.Success(c, nil)
	}
}

// handleBootstrap 把 SPA 首屏必需的引导信息合并为一次往返：公开配置、是否
// 需要初始化、当前登录态、是否以飞牛应用模式运行，以及飞牛部署下授权登录
// 需要的服务端口与网关入口。
//
// 这些数据以前分散在三个接口上串行请求，弱网（手机端经飞牛网关远程访问）
// 下就是多等两三个往返才渲染出第一帧。合成一次请求后首屏只需一个 RTT，也
// 让「打开页面就点飞牛登录」不再有拿不到入口的竞态。
func handleBootstrap(db *gorm.DB, fnOSApp bool, servicePort string) gin.HandlerFunc {
	return func(c *gin.Context) {
		configs, err := sysconfig.GetPublicConfigs(db)
		if err != nil {
			response.ErrorInternal(c, "读取公开配置失败")
			return
		}

		var accountCount int64
		if err := db.Model(&database.User{}).Count(&accountCount).Error; err != nil {
			response.ErrorInternal(c, "检查初始化状态失败")
			return
		}

		requireLogin := configs["require_login"]
		if requireLogin == "" {
			requireLogin = "true"
		}

		userID, _ := c.Get("userID")
		uid, _ := userID.(uint)

		data := map[string]interface{}{
			"setup_required": accountCount == 0,
			"configs":        configs,
			"auth":           CheckAuth(db, uid, requireLogin),
			// 是否真以飞牛应用模式（-fnos-app）运行。前端据此显隐「使用飞牛
			// NAS 登录」：同一套飞牛版前端产物也会被裸二进制 / Docker 直接
			// 跑起来，那种部署下按钮点了只会得到「请先从飞牛桌面打开本应用」
			// 的死路，必须藏掉。编译期标志管不住运行形态，只有服务端知道。
			"fnos_app": fnOSApp,
		}
		if fnOSApp {
			data["service_port"] = servicePort
			if entry := GatewayEntry(); entry != "" {
				data["fnos_gateway_entry"] = entry
			}
			// 网关域上的「登录态来源」。客户端据此区分两种会话：
			// gateway —— 用户已主动登出应用，现在仅靠网关注入的 NAS 身份隐式
			// 维持登录态，NAS 那侧一退出，应用必须跟着退出（会话所属方是 NAS）；
			// app —— 用户显式登录换来的应用会话（含直连端口的 JWT），归应用
			// 自己所有，可以独立退出，也不需要跟随 NAS。
			if middleware.OnFnOSGateway(c.Request.Context()) {
				if _, implicit := middleware.GatewayIdentityUser(c.Request, db); implicit {
					data["session_source"] = "gateway"
				} else {
					data["session_source"] = "app"
				}
			}
		}
		response.Success(c, data)
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
func RegisterRoutes(publicGroup *gin.RouterGroup, optionalAuthGroup *gin.RouterGroup, authGroup *gin.RouterGroup, adminGroup *gin.RouterGroup, db *gorm.DB, fnOSApp bool, servicePort string) {
	publicGroup.GET("/auth/setup-required", handleSetupRequired(db))
	publicGroup.POST("/auth/register", handleRegister(db))
	publicGroup.POST("/auth/login", handleLogin(db))
	if fnOSApp {
		// Minted only for requests that arrived through the gateway unix socket:
		// the browser session was already verified by fnOS and the identity
		// headers cannot be forged there. Tickets let the SPA on the plain TCP
		// port log in without ever seeing or sending the gateway headers.
		publicGroup.POST("/auth/fnos/ticket", func(c *gin.Context) {
			if !middleware.OnFnOSGateway(c.Request.Context()) {
				response.ErrorUnauthorized(c, "请从飞牛桌面中的应用入口使用一键登录")
				return
			}
			identity, ok := fnOSIdentity(c)
			if !ok {
				return
			}
			ticket, err := storeFnOSTicket(identity)
			if err != nil {
				response.ErrorInternal(c, "生成飞牛登录凭证失败")
				return
			}
			response.Success(c, map[string]interface{}{"ticket": ticket, "fnos_username": identity.Username})
		})
		publicGroup.GET("/auth/fnos/identity", handleFnOSIdentity())
		// 飞牛授权诊断（公开、只回显调用者自己的请求信息）：告诉跳板页/排查者
		// 这次请求是不是走网关 socket 来的、网关注入了哪些身份头。直连端口上
		// 必然是 onGatewaySocket=false 且没有任何注入头——这正是「取票 401」
		// 的判据：跳板页被打开在应用自己的直连端口，而不是飞牛网关域。
		// headerNames 只给头名与凭证形态（方案 + 长度），不泄漏任何值。
		publicGroup.GET("/auth/fnos/probe", func(c *gin.Context) {
			names := make([]string, 0, len(c.Request.Header))
			for name := range c.Request.Header {
				names = append(names, strings.ToLower(name))
			}
			sort.Strings(names)
			response.Success(c, map[string]interface{}{
				"onGatewaySocket": middleware.OnFnOSGateway(c.Request.Context()),
				"userid":          c.GetHeader("X-Trim-Userid"),
				"username":        c.GetHeader("X-Trim-Username"),
				"isadmin":         c.GetHeader("X-Trim-Isadmin"),
				"servicePort":     servicePort,
				"host":            c.Request.Host,
				"uri":             c.Request.URL.RequestURI(),
				"headerNames":     names,
				"authorization":   authShape(c.GetHeader("Authorization")),
			})
		})
		publicGroup.POST("/auth/fnos/login", handleFnOSLogin(db))
		publicGroup.POST("/auth/fnos/bind", handleFnOSBind(db))
		// 跳板页登记自己的网关入口：/api/version 随后把它发给 SPA，手机端首次
		// 打开（referrer 为空）也能拿到正确入口，不必猜主机名 + 服务端口。
		publicGroup.POST("/auth/fnos/entry", func(c *gin.Context) {
			if !middleware.OnFnOSGateway(c.Request.Context()) {
				response.ErrorUnauthorized(c, "请从飞牛桌面中的应用入口使用一键登录")
				return
			}
			var req struct {
				URL string `json:"url"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.ErrorBadRequest(c, "无效的网关入口")
				return
			}
			setFnOSGatewayEntry(req.URL)
			response.Success(c, nil)
		})
	}

	optionalAuthGroup.GET("/auth/check", handleCheckAuth(db))
	// 首屏引导：一次请求拿到渲染第一帧所需的全部信息（见 handleBootstrap）。
	// 走 optionalAuth，携带令牌时顺带完成鉴权，前端因此省掉一次 check 往返。
	optionalAuthGroup.GET("/bootstrap", handleBootstrap(db, fnOSApp, servicePort))

	authGroup.GET("/auth/me", handleGetCurrentUser(db))
	authGroup.PUT("/auth/password", handleChangePassword(db))
	authGroup.POST("/auth/apikey", handleRegenerateAPIKey(db))

	// 退出登录走 optionalAuth：会话已失效时重复调用也必须成功（幂等），
	// 网关域上它还会落一条「别再自动认人」的持久化标记（见 handleLogout）。
	optionalAuthGroup.POST("/auth/logout", handleLogout(db))

	adminGroup.GET("/users", handleGetAllUsers(db))
	adminGroup.PUT("/users/:id", handleUpdateUser(db))
	adminGroup.DELETE("/users/:id", handleDeleteUser(db))
	adminGroup.PUT("/users/:id/status", handleToggleStatus(db))
	adminGroup.PUT("/users/:id/password", handleResetPassword(db))
}

// authShape 只描述 Authorization 头的形态（方案名 + 总长度），用于飞牛授权
// 诊断，不泄漏凭证内容本身。
func authShape(raw string) string {
	if raw == "" {
		return ""
	}
	scheme := raw
	if idx := strings.IndexByte(raw, ' '); idx > 0 {
		scheme = raw[:idx]
	}
	return scheme + "/len=" + strconv.Itoa(len(raw))
}
