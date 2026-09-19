package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"smallgo/server/apps"
	"smallgo/server/audit"
	"smallgo/server/config"
	"smallgo/server/database"
	"smallgo/server/logger"
	"smallgo/server/middleware"
	"smallgo/server/response"
	"smallgo/server/scheduler"
	"smallgo/server/security"
	"smallgo/server/stats"
	"smallgo/server/sysconfig"
	"smallgo/server/upload"
	"smallgo/server/user"
	"smallgo/server/version"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func Start() {
	cfg := config.C

	if cfg.ShowHelp {
		config.PrintHelp()
		os.Exit(0)
	}

	if cfg.ShowVersion {
		version.PrintVersion()
		os.Exit(0)
	}

	if cfg.ResetAdminPassword {
		db, err := database.InitDB(cfg.DBPath)
		if err != nil {
			fmt.Printf("Failed to connect database: %v\n", err)
			os.Exit(1)
		}
		defer database.CloseDB(db)

		if err := user.ResetAdminPassword(db); err != nil {
			fmt.Printf("Failed to reset admin password: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if cfg.LogMode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	if err := logger.Init(cfg.LogDir, cfg.LogRetentionDays, cfg.LogConsole); err != nil {
		fmt.Printf("Failed to init logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Close()

	// 监听先行：首次安装时建库、迁移、种子数据可能耗时数秒，桌面端会先于
	// 端口可用就提示“已打开”，连接被拒看起来就是“页面打不开”。先把端口
	// （和网关 socket）占住，初始化期间到达的请求由 readyGate 挂起等待，
	// 就绪后统一放行，浏览器只感到稍慢而不会收到连接拒绝或 502。
	listeners, err := listen(cfg)
	if err != nil {
		fmt.Printf("Failed to listen: %v\n", err)
		os.Exit(1)
	}
	for _, listener := range listeners {
		defer listener.Listener.Close()
	}
	if cfg.FnOSApp {
		defer os.Remove(cfg.GatewaySocket)
	}

	readyGates := make([]*readyGate, 0, len(listeners))
	servers := make([]*http.Server, 0, len(listeners))
	for _, listener := range listeners {
		// 启动阶段先挂 gate：就绪前请求在 gate 上等待，activate 后按监听器
		// 类型切换为真实路由（网关）或带直连重定向的路由（TCP）。
		gate := newReadyGate()
		readyGates = append(readyGates, gate)
		srv := newHTTPServer(gate, listener.IsFnOSGateway)
		servers = append(servers, srv)
		go func(srv *http.Server, listener net.Listener) {
			if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
				fmt.Printf("Server error: %v\n", err)
				os.Exit(1)
			}
		}(srv, listener.Listener)
	}

	db, err := database.InitDB(cfg.DBPath)
	if err != nil {
		fmt.Printf("Failed to connect database: %v\n", err)
		os.Exit(1)
	}
	defer database.CloseDB(db)

	if err := database.AutoMigrate(db); err != nil {
		fmt.Printf("Failed to migrate database: %v\n", err)
		os.Exit(1)
	}

	if err := database.RunUpgrades(db, version.Version, database.Upgrades); err != nil {
		fmt.Printf("Failed to run upgrades: %v\n", err)
		os.Exit(1)
	}

	if err := apps.MigrateAll(db); err != nil {
		fmt.Printf("Failed to run app migrations: %v\n", err)
		os.Exit(1)
	}

	if err := sysconfig.InitDefaultConfigs(db); err != nil {
		fmt.Printf("Failed to init default configs: %v\n", err)
		os.Exit(1)
	}

	logger.SetRetentionFunc(func() int {
		val, err := sysconfig.GetConfig(db, "log_retention_days", 0)
		if err != nil || val == "" {
			return cfg.LogRetentionDays
		}
		if days, err := strconv.Atoi(val); err == nil {
			return days
		}
		return cfg.LogRetentionDays
	})
	audit.RegisterCleanup(db)

	jwtSecret, err := sysconfig.GetConfig(db, "jwt_secret", 0)
	if err != nil || jwtSecret == "" {
		fmt.Println("Failed to get JWT secret")
		os.Exit(1)
	}

	r := NewRouter(cfg, db, jwtSecret)

	// 匿名使用统计心跳：-disable-stats / DISABLE_STATS=1 可完全退出，
	// 运行时也可由管理员通过 stats_enabled 配置关闭。
	if !cfg.DisableStats {
		stats.Start(version.AppID, version.Version, cfg.DeviceType, cfg.DataDir)
		defer stats.Stop()
	}

	// Start background jobs registered by the framework or apps.
	sched := scheduler.Start()
	defer sched.Stop()

	logger.Info("========================================")
	logger.Info("  %s %s", version.AppName, version.Version)
	logger.Info("========================================")
	logger.Info("  Data Dir:    %s", cfg.DataDir)
	logger.Info("  Database:    %s", cfg.DBPath)
	logger.Info("  Web Dir:     %s", cfg.WebDir)
	logger.Info("  Upload Dir:  %s", cfg.UploadDir)
	if cfg.FnOSApp {
		logger.Info("  Gateway:     %s", cfg.GatewayPrefix)
		logger.Info("  Socket:      %s", cfg.GatewaySocket)
	}
	logger.Info("  Access URL:  http://localhost:%d", cfg.Port)
	if cfg.RateLimit > 0 {
		logger.Info("  Rate Limit:  %d req/min per IP", cfg.RateLimit)
	}
	logger.Info("========================================")

	// 路由就绪：按监听器类型绑定最终处理器，并放行初始化期间挂起的请求。
	for i, gate := range readyGates {
		handler := http.Handler(r)
		if !listeners[i].IsFnOSGateway {
			handler = newDirectHandler(r, cfg)
		}
		gate.activate(handler)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, srv := range servers {
		if err := srv.Shutdown(ctx); err != nil {
			logger.Error("Server forced to shutdown: %v", err)
		}
	}

	apps.ShutdownAll()

	logger.Info("Server exited")
}

// readyGate 让端口先于应用就绪开始接受连接：初始化（建库、迁移、种子数据）
// 完成前请求在 gate 上等待，activate 换入最终处理器后统一放行。这样首次安装
// 期间桌面端立即打开页面也不会被拒绝连接，只会稍等片刻。
//
// 等待有上限：初始化若因故卡住，无限挂起会让浏览器停在白屏且无法自愈。
// 超过 readyWaitTimeout 仍未就绪就回 503 + Retry-After，前端据此显示
// 「正在启动」并可重试，而不是永远转圈。
const readyWaitTimeout = 30 * time.Second

type readyGate struct {
	mu    sync.RWMutex
	final http.Handler
	wait  chan struct{}
}

func newReadyGate() *readyGate {
	return &readyGate{wait: make(chan struct{})}
}

// ServeHTTP 在最终处理器就绪前阻塞请求，就绪后直接转发，不再经过本 gate。
// 客户端断开（如浏览器超时重试）时立即返回，不残留无用 goroutine。
func (g *readyGate) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mu.RLock()
	final := g.final
	g.mu.RUnlock()
	if final != nil {
		final.ServeHTTP(w, r)
		return
	}
	timer := time.NewTimer(readyWaitTimeout)
	defer timer.Stop()
	select {
	case <-g.wait:
	case <-timer.C:
		// 未就绪：明确告知可重试，绝不把请求无限挂起。
		w.Header().Set("Retry-After", "3")
		w.Header().Set("Cache-Control", "no-store")
		http.Error(w, "服务正在启动，请稍候重试", http.StatusServiceUnavailable)
		return
	case <-r.Context().Done():
		return
	}
	g.mu.RLock()
	final = g.final
	g.mu.RUnlock()
	if final != nil {
		final.ServeHTTP(w, r)
	}
}

// activate 绑定最终处理器并放行所有等待中的请求。只在初始化完成时调用一次。
func (g *readyGate) activate(final http.Handler) {
	g.mu.Lock()
	g.final = final
	g.mu.Unlock()
	close(g.wait)
}

func newDirectHandler(handler http.Handler, cfg config.Config) http.Handler {
	if !cfg.FnOSApp {
		return handler
	}
	prefix := strings.TrimSuffix(cfg.GatewayPrefix, "/") + "/"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, prefix, http.StatusTemporaryRedirect)
			return
		}
		handler.ServeHTTP(w, r)
	})
}

type listenerBinding struct {
	Listener      net.Listener
	IsFnOSGateway bool
}

func newHTTPServer(handler http.Handler, fnOSGateway bool) *http.Server {
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		// Reminder's SSE connections must stay open indefinitely.
		WriteTimeout: 0,
		IdleTimeout:  60 * time.Second,
	}
	if fnOSGateway {
		srv.ConnContext = func(ctx context.Context, _ net.Conn) context.Context {
			return user.MarkFnOSGateway(ctx)
		}
	}
	return srv
}

func listen(cfg config.Config) ([]listenerBinding, error) {
	tcpListener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		return nil, err
	}
	listeners := []listenerBinding{{Listener: tcpListener}}
	if !cfg.FnOSApp {
		return listeners, nil
	}
	if cfg.GatewaySocket == "" {
		tcpListener.Close()
		return nil, fmt.Errorf("-fnos-app requires -gateway-socket")
	}
	if !strings.HasPrefix(cfg.GatewayPrefix, "/app/") {
		tcpListener.Close()
		return nil, fmt.Errorf("-gateway-prefix must begin with /app/")
	}
	if err := os.MkdirAll(filepath.Dir(cfg.GatewaySocket), 0755); err != nil {
		tcpListener.Close()
		return nil, fmt.Errorf("create gateway socket directory: %w", err)
	}
	if err := os.Remove(cfg.GatewaySocket); err != nil && !os.IsNotExist(err) {
		tcpListener.Close()
		return nil, fmt.Errorf("remove stale gateway socket: %w", err)
	}
	listener, err := net.Listen("unix", cfg.GatewaySocket)
	if err != nil {
		tcpListener.Close()
		return nil, fmt.Errorf("listen on gateway socket: %w", err)
	}
	return append(listeners, listenerBinding{Listener: listener, IsFnOSGateway: true}), nil
}

// NewRouter builds the full API router (all middleware, routes and registered
// apps) without starting an HTTP listener. Start uses it in production; tests
// use it to exercise the API against an in-memory database.
func NewRouter(cfg config.Config, db *gorm.DB, jwtSecret string) *gin.Engine {
	r := gin.New()
	r.Use(middleware.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.CORS(cfg.CORSOrigin))
	r.Use(middleware.LimitJSONBody(1 << 20))
	// 静态资源长缓存：构建产物名带内容哈希，可以与 SPA 入口的 no-store 并存。
	r.Use(middleware.StaticCacheHeaders())
	// 文本响应统一 gzip：160KB 的入口包压到约 1/5，弱网首屏差别最明显。
	r.Use(middleware.Compress())

	appGroup := r.Group("")
	if cfg.FnOSApp {
		appGroup = r.Group(strings.TrimSuffix(cfg.GatewayPrefix, "/"))
	}
	api := appGroup.Group("/api")

	// SPA 入口绝不缓存：升级后浏览器拿到旧版前端会按旧协议与新版跳板页
	// 对话（如弹窗 postMessage），出现取票后整页跳走、登录后弹回等错位。
	// History-mode 路由（/admin、/login…）命中 NoRoute；/ 命中注册的根
	// 路由，二者都要禁缓存。带扩展名的路径（静态资源）仍返回 404，
	// 避免把 index.html 当 JS/CSS 发出去。
	//
	// 入口 HTML 里把 __FNOS_GATEWAY_FLAG__ 替换成「这次文档请求是不是从飞牛网关
	// socket 进来的」：只有服务端知道答案（两条监听器分别是网关 socket 与直连
	// 端口），而前端必须知道——网关域上不能带应用自己的 Authorization，会被接入层
	// 当成无效会话 token 直接拒掉（200 "invalid token"，请求到不了应用），见
	// web/src/utils/gateway.ts 与 AGENTS.md。占位符只出现在字符串字面量里
	// （window.__FNOS_GATEWAY__ = "__FNOS_GATEWAY_FLAG__" === "true"），
	// 这样开发服务器不替换时它就是 false，也不会误伤属性名。
	serveSPA := func(c *gin.Context) {
		if path.Ext(c.Request.URL.Path) != "" {
			c.Status(http.StatusNotFound)
			return
		}
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		data, err := os.ReadFile(filepath.Join(cfg.WebDir, "index.html"))
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		page := strings.ReplaceAll(string(data), "__FNOS_GATEWAY_FLAG__", strconv.FormatBool(middleware.OnFnOSGateway(c.Request.Context())))
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(page))
	}
	if cfg.WebDir != "" {
		r.NoRoute(serveSPA)
		r.GET("/", serveSPA)
	}

	api.GET("/version", func(c *gin.Context) {
		info := version.GetVersion()
		// 是否以飞牛应用模式（-fnos-app）运行：与 /api/bootstrap 的 fnos_app
		// 同源，给跳过引导直接查版本的路径用（resolveFnOSGatewayEntry 兜底）。
		// 前端据此显隐「使用飞牛 NAS 登录」，非飞牛部署不再摆一条点了必死的路。
		info["fnosApp"] = cfg.FnOSApp
		// 飞牛网关嵌入的入口需要真实服务端口：SPA 据此判断当前是否停留在
		// 网关域名下（旧书签、隐藏登录入口打开的场景），才能正确推导跳板页
		// 地址并完成授权交接。0 表示不适用（非飞牛模式）。
		info["servicePort"] = strconv.Itoa(cfg.Port)
		// 跳板页登记过的网关入口：手机端首次打开时 referrer 为空、localStorage
		// 也为空，只有这里能给出「本机在网关下的入口」，避免拼出打不开的地址。
		if cfg.FnOSApp {
			if entry := user.GatewayEntry(); entry != "" {
				info["fnosGatewayEntry"] = entry
			}
		}
		response.Success(c, info)
	})

	// Liveness/readiness probe for Docker, NAS health checks, uptime monitors.
	api.GET("/health", func(c *gin.Context) {
		response.Success(c, gin.H{
			"status":  "ok",
			"version": version.Version,
			"time":    time.Now().Format(time.RFC3339),
		})
	})

	// Rate limiter for public endpoints
	var publicLimiter gin.HandlerFunc
	if cfg.RateLimit > 0 {
		rl := middleware.NewRateLimiter(cfg.RateLimit, 1*time.Minute)
		publicLimiter = middleware.LimitRequests(rl)
	} else {
		publicLimiter = middleware.LimitRequests(nil)
	}

	// Auth route groups shared by all features.
	publicGroup := api.Group("", publicLimiter)
	optionalAuthGroup := api.Group("")
	optionalAuthGroup.Use(middleware.OptionalAuth(jwtSecret, db))
	authGroup := api.Group("")
	authGroup.Use(middleware.RequireAuth(jwtSecret, db))
	authGroup.Use(audit.MutationLogger(db))
	adminGroup := api.Group("")
	adminGroup.Use(middleware.RequireAuth(jwtSecret, db))
	adminGroup.Use(middleware.RequireAdmin())
	adminGroup.Use(audit.MutationLogger(db))

	// Config: public read is rate-limited; user config read & write require
	// auth; system config read & metadata require admin.
	sysconfig.RegisterRoutes(publicGroup, authGroup, adminGroup, db)

	// Security questions: forgot-password flow is public (rate limited); managing
	// one's own questions requires auth.
	security.RegisterRoutes(publicGroup, authGroup, db)

	// File upload requires authentication; uploaded content is still readable by
	// URL so it can be embedded in public pages.
	authGroup.POST("/upload", upload.HandleUpload(cfg.UploadDir))
	appGroup.GET("/uploads/*filepath", upload.ServeUpload(cfg.UploadDir))

	user.RegisterRoutes(publicGroup, optionalAuthGroup, authGroup, adminGroup, db, cfg.FnOSApp, strconv.Itoa(cfg.Port))

	// 赞赏支持计数：需要登录（前端入口仅对管理员显示），
	// 复用匿名统计通道发送一次 donate_support 事件。
	stats.RegisterRoutes(authGroup, db)

	// Admin operation log (browse + CSV export).
	audit.RegisterRoutes(adminGroup, db)

	// Register apps from the registry (includes the qrcode example app).
	apps.SetupAll(publicGroup, db)
	apps.SetupProtectedAll(authGroup, adminGroup, db)

	if cfg.WebDir != "" {
		appGroup.Static("/assets", cfg.WebDir+"/assets")
		appGroup.StaticFile("/favicon.ico", cfg.WebDir+"/favicon.ico")
		appGroup.StaticFile("/favicon.svg", cfg.WebDir+"/favicon.svg")
		appGroup.StaticFile("/favicon-16.png", cfg.WebDir+"/favicon-16.png")
		appGroup.StaticFile("/favicon-32.png", cfg.WebDir+"/favicon-32.png")
		appGroup.StaticFile("/favicon-192.png", cfg.WebDir+"/favicon-192.png")
		appGroup.StaticFile("/favicon-512.png", cfg.WebDir+"/favicon-512.png")
		appGroup.StaticFile("/apple-touch-icon.png", cfg.WebDir+"/apple-touch-icon.png")
		appGroup.GET("/manifest.webmanifest", func(c *gin.Context) {
			c.Header("Content-Type", "application/manifest+json")
			c.File(cfg.WebDir + "/manifest.webmanifest")
		})
		// Gateway trampoline: served on the gateway prefix with the real
		// service port injected, it exchanges the fnOS gateway session for a
		// one-time ticket and sends the browser to the plain TCP listener.
		appGroup.GET("/fnos-entry.html", func(c *gin.Context) {
			data, err := os.ReadFile(filepath.Join(cfg.WebDir, "fnos-entry.html"))
			if err != nil {
				c.String(http.StatusNotFound, "未找到飞牛登录跳板页")
				return
			}
			page := strings.ReplaceAll(string(data), "__APP_PORT__", strconv.Itoa(cfg.Port))
			// 跳板页协议随版本演进，绝不缓存：旧版缓存会按旧协议对话导致
			// 授权卡死（v0.6.2 的教训）。
			c.Header("Cache-Control", "no-store")
			c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(page))
		})
	}

	return r
}
