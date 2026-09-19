// Package stats 提供匿名使用统计上报与「赞赏支持」计数。
//
// 设计原则（与《应用支持功能与在线统计接入指南》一致，不可妥协）：
//  1. 匿名透明：只上报设备级信息（哈希标识/系统/架构/版本），不含任何用户数据；
//  2. 可退出：-disable-stats 启动参数或 DISABLE_STATS=1 环境变量完全关闭，
//     管理员也可在系统配置中通过 stats_enabled 运行时开关关闭；
//  3. 上报走 HTTPS，地址可用 STATS_ENDPOINT 环境变量覆盖（本地测试指向假服务）；
//  4. 「已支持」状态以服务端结果为准：统计端点返回非 2xx 一律视为失败。
//
// 设备标识三级回退：系统机器标识 → 数据目录持久化随机 ID（容器环境）→
// hostname+os+arch 兜底。哈希原料只含应用名与机器标识，永不包含 hostname
// 原文、用户数据或数据目录路径。
package stats

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"smallgo/server/response"
	"smallgo/server/sysconfig"
)

// Request 是统计端点的心跳/事件上报体。
type Request struct {
	AppName    string `json:"app_name"`
	Version    string `json:"version"`
	DeviceType string `json:"device_type"` // fnos|docker|（裸二进制为空）
	DeviceID   string `json:"device_id"`   // 32 位 md5 哈希，永远不含原文
	OS         string `json:"os"`
	Arch       string `json:"arch"`
	// Event 区分上报类型：心跳不携带，赞赏点击为 "donate_support"
	Event string `json:"event,omitempty"`
	// Amount 赞赏金额（元），仅 donate_support 事件携带，用于接收端统计
	// 赞赏流水（app_support_log.amount）。心跳不携带。
	Amount float64 `json:"amount,omitempty"`
}

const (
	heartbeatInterval = 60 * time.Minute
	donateEvent       = "donate_support"
)

var (
	httpClient = &http.Client{Timeout: 5 * time.Second}
	mu         sync.Mutex
	base       Request
	running    bool
	stopCh     chan struct{}
)

// Endpoint 返回统计上报地址。STATS_ENDPOINT 环境变量可覆盖（测试用）。
func Endpoint() string {
	if e := os.Getenv("STATS_ENDPOINT"); e != "" {
		return e
	}
	return "https://techfunway.wycto.cn/api/apps.online/refresh"
}

// Init 组装本实例的公共上报字段。Start = Init + 心跳循环；
// 单元测试只调 Init，避免真实网络请求。
func Init(appName, ver, deviceType, dataDir string) {
	mu.Lock()
	defer mu.Unlock()
	base = Request{
		AppName:    appName,
		Version:    ver,
		DeviceType: deviceType,
		DeviceID:   DeviceID(appName, dataDir),
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
	}
}

// Start 启动心跳上报：启动时立即报第一次，之后每 60 分钟一次。
// 重复调用是安全的（只生效第一次）。
func Start(appName, ver, deviceType, dataDir string) {
	Init(appName, ver, deviceType, dataDir)

	mu.Lock()
	if running {
		mu.Unlock()
		return
	}
	running = true
	stopCh = make(chan struct{})
	mu.Unlock()

	go func() {
		report()
		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				report()
			}
		}
	}()
}

// Stop 停止心跳循环（进程优雅关闭时调用）。
func Stop() {
	mu.Lock()
	defer mu.Unlock()
	if running {
		close(stopCh)
		running = false
	}
}

func report() {
	mu.Lock()
	req := base
	mu.Unlock()
	if req.AppName == "" {
		return
	}
	body, _ := json.Marshal(req)
	resp, err := httpClient.Post(Endpoint(), "application/json", bytes.NewReader(body))
	if err != nil {
		fmt.Printf("使用统计上报失败: %v\n", err)
		return
	}
	resp.Body.Close()
}

// DeviceID 计算设备唯一标识：机器标识优先，容器等无标识环境退回
// 数据目录持久化 ID，最终兜底 hostname+os+arch。只上报加应用名前缀的
// md5 哈希，防止跨应用关联。
func DeviceID(appName, dataDir string) string {
	source := machineSignature()
	if source == "" {
		source = persistentDeviceID(dataDir)
	}
	return fmt.Sprintf("%x", md5.Sum([]byte(appName+"|"+source)))
}

// machineSignature 读取操作系统安装时生成的机器标识。
// 卸载重装应用不会变化，仅重装系统才会改变；读取失败返回空串。
func machineSignature() string {
	switch runtime.GOOS {
	case "linux":
		for _, p := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id"} {
			if b, err := os.ReadFile(p); err == nil {
				if s := strings.TrimSpace(string(b)); s != "" {
					return s
				}
			}
		}
	case "darwin":
		out, err := exec.Command("ioreg", "-rd1", "-c", "IOPlatformExpertDevice").Output()
		if err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				if !strings.Contains(line, "IOPlatformUUID") {
					continue
				}
				if i := strings.Index(line, "= \""); i >= 0 {
					return strings.Trim(line[i+3:], "\" ")
				}
			}
		}
	case "windows":
		out, err := exec.Command("reg", "query", `HKLM\SOFTWARE\Microsoft\Cryptography`, "/v", "MachineGuid").Output()
		if err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				if strings.Contains(line, "MachineGuid") {
					fields := strings.Fields(line)
					if len(fields) >= 3 {
						return fields[len(fields)-1]
					}
				}
			}
		}
	}
	return ""
}

// persistentDeviceID 无系统机器标识时（常见于容器环境），在数据目录持久化
// 一个随机 ID：容器重建、主机改名后仍保持同一身份（数据目录是挂载卷）。
func persistentDeviceID(dataDir string) string {
	path := filepath.Join(dataDir, "device.id")
	if b, err := os.ReadFile(path); err == nil {
		if s := strings.TrimSpace(string(b)); s != "" {
			return s
		}
	}
	id := fmt.Sprintf("%d-%d", time.Now().UnixNano(), os.Getpid())
	if err := os.MkdirAll(dataDir, 0o755); err == nil {
		if err := os.WriteFile(path, []byte(id), 0o600); err == nil {
			return id
		}
	}
	// 数据目录不可写时的最终兜底
	hostname, _ := os.Hostname()
	return hostname + runtime.GOOS + runtime.GOARCH
}

// enabled 读取运行时开关（管理员可在系统配置中关闭）。
// 未初始化或读取失败时默认开启，保持与 flag 行为一致。
func enabled(db *gorm.DB) bool {
	if db == nil {
		return true
	}
	v, err := sysconfig.GetConfig(db, "stats_enabled", 0)
	if err != nil || v == "" {
		return true
	}
	return v != "false"
}

// RegisterRoutes 挂载需要登录的赞赏支持端点。不要求管理员，但前端入口
// 只对管理员显示。
func RegisterRoutes(auth *gin.RouterGroup, db *gorm.DB) {
	auth.POST("/donate/support", func(c *gin.Context) {
		if !enabled(db) {
			response.ErrorForbidden(c, "统计上报已被管理员禁用")
			return
		}
		// 支持金额（元）：可选字段，缺省为 0（不校验支付，仅作金额标注）
		var payload struct {
			Amount float64 `json:"amount"`
		}
		_ = c.ShouldBindJSON(&payload)
		if payload.Amount < 0 {
			payload.Amount = 0
		}

		mu.Lock()
		req := base
		mu.Unlock()
		req.Event = donateEvent
		req.Amount = payload.Amount
		body, _ := json.Marshal(req)
		resp, err := httpClient.Post(Endpoint(), "application/json", bytes.NewReader(body))
		if err != nil {
			response.ErrorInternal(c, "支持计数发送失败，请稍后再试")
			return
		}
		resp.Body.Close()
		// 统计服务返回非 2xx 视为发送失败（http.Post 不把 4xx/5xx 当作 err）
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			response.ErrorInternal(c, "支持计数发送失败，请稍后再试")
			return
		}
		response.Success(c, gin.H{"ok": true})
	})
}

func init() {
	sysconfig.RegisterConfig(sysconfig.ConfigDef{
		Key:         "stats_enabled",
		Scope:       sysconfig.ScopeSystem,
		Type:        sysconfig.TypeBool,
		Default:     "true",
		Public:      false,
		Group:       "stats",
		Label:       "匿名使用统计",
		Description: "仅上报设备级信息（哈希标识/系统/架构/版本），不含任何用户数据；关闭后心跳与赞赏计数均停止",
	})
}
