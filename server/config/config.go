package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	Port               int
	DataDir            string
	WebDir             string
	UploadDir          string
	LogMode            string
	JWTSecret          string
	DBPath             string
	ShowVersion        bool
	ShowHelp           bool
	ResetAdminPassword bool
	AppName            string
	RateLimit          int
	CORSOrigin         string
	LogDir             string
	LogRetentionDays   int
	LogConsole         bool
	// FnOSApp enables the fnOS unified-gateway integration. Ordinary binaries
	// and Docker deployments keep their TCP listener and never trust fnOS
	// gateway identity headers.
	FnOSApp       bool
	GatewaySocket string
	GatewayPrefix string
}

var C Config

func Parse() Config {
	flag.IntVar(&C.Port, "port", 8906, "服务监听端口（环境变量 PORT）")
	flag.StringVar(&C.DataDir, "data-dir", "./data", "数据目录：数据库、上传文件、日志（环境变量 DATA_DIR）")
	flag.StringVar(&C.WebDir, "web-dir", "./", "前端静态文件目录")
	flag.StringVar(&C.UploadDir, "upload-dir", "", "上传文件目录（默认 <data-dir>/uploads）")
	flag.StringVar(&C.LogMode, "logmode", "release", "日志模式：release 或 debug（环境变量 ENV）")
	flag.BoolVar(&C.ShowVersion, "version", false, "显示版本信息后退出")
	flag.BoolVar(&C.ShowHelp, "help", false, "显示帮助信息")
	flag.BoolVar(&C.ShowHelp, "h", false, "显示帮助信息")
	flag.BoolVar(&C.ResetAdminPassword, "reset-admin-password", false, "将管理员密码重置为 admin123 后退出")
	flag.IntVar(&C.RateLimit, "rate-limit", 20, "公开接口每分钟最大请求数，0 表示不限（环境变量 RATE_LIMIT）")
	flag.StringVar(&C.CORSOrigin, "cors-origin", "*", "允许的 CORS 来源，* 表示全部（环境变量 CORS_ORIGIN）")
	flag.IntVar(&C.LogRetentionDays, "log-retention-days", 30, "日志文件保留天数，0 表示永久保留（环境变量 LOG_RETENTION_DAYS）")
	flag.BoolVar(&C.LogConsole, "log-console", false, "同时将运行日志输出到终端（默认 false，环境变量 LOG_CONSOLE）")
	flag.BoolVar(&C.FnOSApp, "fnos-app", false, "以飞牛 fnOS 统一网关应用模式运行")
	flag.StringVar(&C.GatewaySocket, "gateway-socket", "", "飞牛统一网关 Unix Socket 路径（仅 -fnos-app）")
	flag.StringVar(&C.GatewayPrefix, "gateway-prefix", "/app/techfunway-reminders", "飞牛统一网关路径前缀（仅 -fnos-app）")
	flag.Usage = PrintHelp
	flag.Parse()

	// `smallgo help` (positional) or `-help`/`-h` both request help. Help and
	// version are print-and-exit modes, so return early without creating the
	// data directory as a side effect.
	if flag.NArg() > 0 && flag.Arg(0) == "help" {
		C.ShowHelp = true
	}
	if C.ShowHelp || C.ShowVersion {
		return C
	}

	if v := os.Getenv("PORT"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			C.Port = i
		}
	}
	if v := os.Getenv("DATA_DIR"); v != "" {
		C.DataDir = v
	}
	if v := os.Getenv("DB_PATH"); v != "" {
		C.DBPath = v
	}
	if v := os.Getenv("ENV"); v != "" {
		C.LogMode = v
	}
	if v := os.Getenv("RATE_LIMIT"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			C.RateLimit = i
		}
	}
	if v := os.Getenv("CORS_ORIGIN"); v != "" {
		C.CORSOrigin = v
	}
	if v := os.Getenv("LOG_RETENTION_DAYS"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			C.LogRetentionDays = i
		}
	}
	if v := os.Getenv("LOG_CONSOLE"); v != "" {
		if enabled, err := strconv.ParseBool(v); err == nil {
			C.LogConsole = enabled
		}
	}

	if C.UploadDir == "" {
		C.UploadDir = filepath.Join(C.DataDir, "uploads")
	}
	if C.DBPath == "" {
		C.DBPath = filepath.Join(C.DataDir, "db", "reminder.db")
	}
	if C.LogDir == "" {
		C.LogDir = filepath.Join(C.DataDir, "logs")
	}

	os.MkdirAll(C.DataDir, 0755)
	os.MkdirAll(filepath.Join(C.DataDir, "db"), 0755)
	os.MkdirAll(filepath.Join(C.DataDir, "logs"), 0755)
	os.MkdirAll(filepath.Join(C.DataDir, "uploads"), 0755)
	os.MkdirAll(C.UploadDir, 0755)

	return C
}

// PrintHelp prints a user-facing guide to the startup parameters. It is wired
// as flag.Usage so it also shows on invalid flags.
func PrintHelp() {
	fmt.Print(`提醒事项 — 简洁可靠的多渠道提醒应用

用法:
  reminder [选项]
  reminder help            显示本帮助
  reminder -version        显示版本信息

启动参数:
  -port int
        服务监听端口（默认 8906，环境变量 PORT）
  -data-dir string
        数据目录，存放数据库、上传文件与日志（默认 "./data"，环境变量 DATA_DIR）
  -web-dir string
        前端静态文件目录（默认 "./"）
  -upload-dir string
        上传文件目录（默认 "<data-dir>/uploads"）
  -logmode string
        日志模式：release 或 debug（默认 "release"，环境变量 ENV）
  -rate-limit int
        公开接口每分钟最大请求数，0 表示不限（默认 20，环境变量 RATE_LIMIT）
  -cors-origin string
        允许的 CORS 来源，* 表示全部（默认 "*"，环境变量 CORS_ORIGIN）
  -log-retention-days int
        日志文件保留天数，0 表示永久保留（默认 30，环境变量 LOG_RETENTION_DAYS）
  -log-console
        同时将运行日志输出到终端（默认 false，环境变量 LOG_CONSOLE）
  -fnos-app
        以飞牛 fnOS 统一网关应用模式运行（默认 false）
  -gateway-socket string
        飞牛统一网关 Unix Socket 路径（仅 -fnos-app）
  -gateway-prefix string
        飞牛统一网关路径前缀（默认 "/app/techfunway-reminders"，仅 -fnos-app）
  -reset-admin-password
        将管理员密码重置为 admin123 后退出
  -version
        显示版本信息后退出
  -help, -h
        显示本帮助

环境变量:
  PORT、DATA_DIR、DB_PATH、ENV、RATE_LIMIT、CORS_ORIGIN、LOG_RETENTION_DAYS、LOG_CONSOLE
  环境变量的优先级高于启动参数。

示例:
  reminder -port 8906 -data-dir /var/lib/reminder
  DATA_DIR=/app/data RATE_LIMIT=0 reminder
`)
}
