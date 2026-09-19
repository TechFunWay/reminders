package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type visitor struct {
	count   int
	expires time.Time
}

type RateLimiter struct {
	mu          sync.Mutex
	visitors    map[string]*visitor
	rate        int
	window      time.Duration
	nextCleanup time.Time
}

func NewRateLimiter(rate int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		visitors:    make(map[string]*visitor),
		rate:        rate,
		window:      window,
		nextCleanup: time.Now().Add(window),
	}
	return rl
}

func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	if !now.Before(rl.nextCleanup) {
		for key, item := range rl.visitors {
			if !now.Before(item.expires) {
				delete(rl.visitors, key)
			}
		}
		rl.nextCleanup = now.Add(rl.window)
	}

	v, exists := rl.visitors[ip]
	if !exists || !now.Before(v.expires) {
		rl.visitors[ip] = &visitor{count: 1, expires: now.Add(rl.window)}
		return true
	}

	if v.count >= rl.rate {
		return false
	}

	v.count++
	return true
}

// bootstrapReads 是 SPA 每次进入都必须读的只读引导接口，不参与限流：
// 它们既不是可爆破的凭证接口，又被前端当作「应用是否可用」的依据。一旦被
// 429，前端只能拿到默认值继续跑——初始化状态读不到就会把首次安装的用户带到
// 登录页而不是管理员创建页，且完全没有可见错误。限流的目的是抵挡凭证爆破，
// 不该牺牲这些幂等读。
var bootstrapReads = map[string]struct{}{
	"/configs/public":      {},
	"/auth/setup-required": {},
}

func isBootstrapRead(c *gin.Context) bool {
	if c.Request.Method != http.MethodGet {
		return false
	}
	path := c.Request.URL.Path
	// 网关前缀（/app/<包名>）随部署变化，按后缀匹配真实路由。
	for suffix := range bootstrapReads {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	return false
}

// rateLimitKey decides which bucket a request counts against.
//
// Requests arriving through the fnOS unified gateway come from the local unix
// socket, so ClientIP() is empty for every one of them. Keying those by IP
// collapses every NAS user on the box into a single bucket: one mobile page
// load spends the whole per-minute allowance on public calls and the next load
// gets 429s, which the SPA cannot distinguish from a dead server. The gateway
// has already authenticated the NAS session and injects X-Trim-Userid, so
// gateway traffic is keyed per NAS user instead.
func rateLimitKey(c *gin.Context) string {
	if OnFnOSGateway(c.Request.Context()) {
		if uid := strings.TrimSpace(c.GetHeader("X-Trim-Userid")); uid != "" {
			return "fnos:" + uid
		}
		return "fnos:anonymous"
	}
	if ip := strings.TrimSpace(c.ClientIP()); ip != "" {
		return "ip:" + ip
	}
	return "unknown"
}

func LimitRequests(rl *RateLimiter) gin.HandlerFunc {
	if rl == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		if isBootstrapRead(c) {
			c.Next()
			return
		}
		if !rl.Allow(rateLimitKey(c)) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    429,
				"message": "too many requests, please try again later",
				"data":    nil,
			})
			return
		}
		c.Next()
	}
}
