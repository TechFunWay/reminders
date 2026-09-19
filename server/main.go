package main

import (
	// 嵌入 IANA 时区库：本应用的日历语义（今天/计划视图过滤、重复规则、汇总
	// 计数）依赖 Asia/Shanghai，代码里多处 `time.LoadLocation("Asia/Shanghai")`
	// 都忽略了错误。宿主环境（精简的 fnOS 根文件系统、最小化容器镜像）可能
	// 没有 /usr/share/zoneinfo，此时 LoadLocation 返回 nil，相关接口会直接
	// 500（实测：无 tzdata 的环境下 /api/reminder/items 与 /api/reminder/summary
	// 返回 500，装上 tzdata 后恢复 200）。嵌入后与宿主机是否带时区库无关。
	_ "time/tzdata"

	"smallgo/server/config"
	"smallgo/server/server"

	// Blank-import apps so their init() registers routes with the app registry.
	// Add your own apps here.
	_ "smallgo/server/reminder"
)

func main() {
	// `make dev` starts the binary from the repository root. Loading .env here
	// makes provider credentials available locally without changing production
	// environment-variable precedence.
	config.LoadDotEnv(".env", "server/.env")
	config.Parse()
	server.Start()
}
