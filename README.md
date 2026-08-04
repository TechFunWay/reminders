# Reminder

面向中国大陆用户的轻量多渠道提醒应用。界面借鉴移动端提醒工具的低学习成本，支持清单、
今天/计划/全部/已完成视图、重复提醒、稍后提醒，以及站内、邮件、短信、飞书和 QQ 通知。

## 技术栈

- Go + Gin + GORM + SQLite（WAL）
- Vue 3 + TypeScript + Pinia + Vue Router
- Tailwind CSS + Vite

## 本地开发

要求 Go 1.26+、Node.js 20+、CGO 和本地 C 编译器。

```bash
# 构建 Vue 前端、复制到后端静态目录、构建并启动 Go 后端
make dev
```

浏览器访问 `http://localhost:8906`。开发模式只启动 Go 后端一个服务，API 和前端页面
共用 8906 端口；首次运行会自动安装前端依赖。可通过 `PORT=9000 make dev` 临时改端口。

需要邮件、短信、飞书或 QQ 时，先复制 `.env.example` 为 `.env`，填入相应服务端凭证，再运行
`make dev`。`.env` 会在本地启动时自动读取，且不会覆盖已经设置的系统环境变量。

仅构建或启动已构建产物：

```bash
make build
make start
```

首次注册的用户自动成为管理员。

## 核心能力

- 默认清单和自定义清单；
- 今天、计划、全部、已完成智能视图；
- 快捷新增、编辑、删除、完成、恢复；
- 每天、每周、每月、每年重复；
- 稍后 10 分钟提醒；
- 持久化投递任务、幂等领取、失败分类和最多三次重试；
- 站内通知中心；
- SMTP 邮件；
- 可配置短信 HTTP 网关；
- 飞书企业自建应用机器人单聊；
- QQ 机器人主动单聊；
- 浅色/深色主题和响应式手机界面。

## 外部渠道配置

```text
# 邮件
SMTP_HOST
SMTP_PORT
SMTP_USERNAME
SMTP_PASSWORD
SMTP_FROM_NAME
SMTP_FROM_ADDRESS

# 短信 HTTP 网关
SMS_WEBHOOK_URL
SMS_WEBHOOK_TOKEN

# 飞书
FEISHU_APP_ID
FEISHU_APP_SECRET
FEISHU_BASE_URL

# QQ
QQ_BOT_APP_ID
QQ_BOT_APP_SECRET
QQ_BOT_API_BASE

# 可选：单独的数据加密密钥
REMINDER_DATA_KEY
```

渠道接收目标在数据库中使用 AES-GCM 加密。未设置 `REMINDER_DATA_KEY` 时，会从安装实例的
内部 JWT 密钥派生独立加密密钥。

## 构建与测试

```bash
cd server && go test ./...
cd web && npm run build
make build
```
