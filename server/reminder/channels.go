package reminder

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"net/smtp"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"smallgo/server/response"
	"smallgo/server/sysconfig"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type deliveryError struct {
	Code      string
	Message   string
	Permanent bool
}

func (e *deliveryError) Error() string { return e.Message }

type sendResult struct {
	ExternalID string
}

var cnMobile = regexp.MustCompile(`^1[3-9]\d{9}$`)

func handleChannelStatuses(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		response.Success(c, channelStatuses(db, currentUserID(c)))
	}
}

func handleProviderStatuses(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		response.Success(c, gin.H{
			"email": gin.H{"configured": emailConfigured(db)}, "sms": gin.H{"configured": smsConfigured(db)},
			"feishu": gin.H{"configured": feishuConfigured(db)}, "qq": gin.H{"configured": qqConfigured(db)},
		})
	}
}

// The notification brand is the human-readable source shown in robot
// messages. It is separate from the sender mailbox so a friendly product name
// can be used consistently in Feishu and QQ.
func handleNotificationBrand(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodGet {
			response.Success(c, gin.H{"name": notificationBrand(db)})
			return
		}
		var in struct {
			Name string `json:"name"`
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			response.ErrorBadRequest(c, "通知来源名称无效")
			return
		}
		in.Name = strings.TrimSpace(in.Name)
		if in.Name == "" || len([]rune(in.Name)) > 40 {
			response.ErrorBadRequest(c, "通知来源名称应为 1 到 40 个字符")
			return
		}
		if err := saveProviderSettings(db, "notification_brand", map[string]string{"name": in.Name}); err != nil {
			response.ErrorInternal(c, "保存通知来源名称失败")
			return
		}
		response.Success(c, gin.H{"name": in.Name})
	}
}

func handleSaveProvider(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		provider := strings.ToLower(c.Param("provider"))
		if provider != ChannelEmail && provider != ChannelSMS && provider != ChannelQQ {
			response.ErrorBadRequest(c, "不支持的通知服务")
			return
		}
		var values map[string]string
		if err := c.ShouldBindJSON(&values); err != nil {
			response.ErrorBadRequest(c, "配置参数无效")
			return
		}
		for k, v := range values {
			values[k] = strings.TrimSpace(v)
		}
		valid := false
		switch provider {
		case ChannelEmail:
			host := strings.TrimSpace(strings.ToLower(values["host"]))
			valid = strings.Contains(host, ".") && !strings.ContainsAny(host, " /:@") && values["from_address"] != ""
			if !valid {
				response.ErrorBadRequest(c, "SMTP 服务器应填写完整域名，例如 smtp.qq.com，而不是 SMTP")
				return
			}
		case ChannelSMS:
			valid = values["webhook_url"] != ""
		case ChannelQQ:
			valid = values["app_id"] != "" && values["app_secret"] != ""
			if link := values["bot_link"]; link != "" {
				parsed, err := url.ParseRequestURI(link)
				if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
					response.ErrorBadRequest(c, "机器人主页应填写有效的 http 或 https 链接")
					return
				}
			}
		}
		if !valid {
			response.ErrorBadRequest(c, "请填写必填配置")
			return
		}
		if err := saveProviderSettings(db, provider, values); err != nil {
			response.ErrorInternal(c, "保存通知服务配置失败")
			return
		}
		response.Success(c, gin.H{"configured": true})
	}
}

func handleSaveFeishuProvider(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in struct {
			AppID     string `json:"app_id"`
			AppSecret string `json:"app_secret"`
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			response.ErrorBadRequest(c, "配置参数无效")
			return
		}
		in.AppID, in.AppSecret = strings.TrimSpace(in.AppID), strings.TrimSpace(in.AppSecret)
		if in.AppID == "" || in.AppSecret == "" {
			response.ErrorBadRequest(c, "请填写 App ID 和 App Secret")
			return
		}
		secret, err := encryptTarget(db, in.AppID+"\n"+in.AppSecret)
		if err != nil {
			response.ErrorInternal(c, "加密飞书配置失败")
			return
		}
		row := ProviderConfig{Provider: ChannelFeishu, Secret: secret}
		if err := db.Where("provider = ?", ChannelFeishu).Assign(map[string]interface{}{"secret": secret}).FirstOrCreate(&row).Error; err != nil {
			response.ErrorInternal(c, "保存飞书配置失败")
			return
		}
		response.Success(c, gin.H{"configured": true})
	}
}

func handleBindChannel(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		channel := strings.ToLower(c.Param("channel"))
		if channel == ChannelInApp || !supportedChannels[channel] {
			response.ErrorBadRequest(c, "该渠道不支持绑定")
			return
		}
		var in struct {
			Target string `json:"target"`
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			response.ErrorBadRequest(c, "绑定参数无效")
			return
		}
		target, err := normalizeTarget(channel, in.Target)
		if err == nil && channel == ChannelFeishu {
			target, err = resolveFeishuOpenID(c.Request.Context(), db, target)
		}
		if err != nil {
			response.ErrorBadRequest(c, err.Error())
			return
		}
		encrypted, err := encryptTarget(db, target)
		if err != nil {
			response.ErrorInternal(c, "加密绑定信息失败")
			return
		}
		now := time.Now()
		binding := ChannelBinding{
			UserID: currentUserID(c), Channel: channel, Target: encrypted,
			TargetMasked: maskTarget(channel, target), Status: "active", VerifiedAt: &now,
		}
		err = db.Where("user_id = ? AND channel = ?", binding.UserID, channel).
			Assign(map[string]interface{}{
				"target": encrypted, "target_masked": binding.TargetMasked,
				"status": "active", "verified_at": &now, "last_error_code": "",
			}).FirstOrCreate(&binding).Error
		if err != nil {
			response.ErrorInternal(c, "保存绑定失败")
			return
		}
		response.Success(c, binding)
	}
}

func handleToggleChannel(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		channel := strings.ToLower(c.Param("channel"))
		var in struct {
			Enabled bool `json:"enabled"`
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			response.ErrorBadRequest(c, "渠道参数无效")
			return
		}
		status := "disabled"
		if in.Enabled {
			status = "active"
		}
		result := db.Model(&ChannelBinding{}).Where("user_id = ? AND channel = ?", currentUserID(c), channel).Update("status", status)
		if result.Error != nil {
			response.ErrorInternal(c, "更新渠道失败")
			return
		}
		if result.RowsAffected == 0 {
			response.ErrorBadRequest(c, "请先绑定该渠道")
			return
		}
		response.Success(c, gin.H{"status": status})
	}
}

func handleUnbindChannel(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		channel := strings.ToLower(c.Param("channel"))
		if err := db.Where("user_id = ? AND channel = ?", currentUserID(c), channel).Delete(&ChannelBinding{}).Error; err != nil {
			response.ErrorInternal(c, "解绑失败")
			return
		}
		response.Success(c, gin.H{"deleted": true})
	}
}

func handleTestChannel(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		channel := strings.ToLower(c.Param("channel"))
		if !supportedChannels[channel] {
			response.ErrorBadRequest(c, "不支持的通知渠道")
			return
		}
		userID := currentUserID(c)
		item := Reminder{ID: 0, UserID: userID, Title: "这是一条测试提醒", Notes: "渠道已经连接成功。"}
		result, err := sendChannel(c.Request.Context(), db, channel, userID, item, "test-"+strconv.FormatInt(time.Now().UnixNano(), 10))
		if err != nil {
			response.ErrorBadRequest(c, err.Error())
			return
		}
		now := time.Now()
		_ = db.Model(&ChannelBinding{}).Where("user_id = ? AND channel = ?", userID, channel).Updates(map[string]interface{}{
			"last_tested_at": &now, "last_error_code": "",
		}).Error
		response.Success(c, gin.H{"sent": true, "external_id": result.ExternalID})
	}
}

func handleCreateQQBindCode(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !qqConfigured(db) {
			response.ErrorBadRequest(c, "QQ 机器人尚未开通")
			return
		}
		_ = db.Where("user_id = ?", currentUserID(c)).Delete(&QQBindCode{}).Error
		code, err := newQQBindCode()
		if err != nil {
			response.ErrorInternal(c, "生成绑定码失败")
			return
		}
		row := QQBindCode{UserID: currentUserID(c), Code: code, ExpiresAt: time.Now().Add(10 * time.Minute)}
		if err := db.Create(&row).Error; err != nil {
			response.ErrorInternal(c, "保存绑定码失败")
			return
		}
		response.Success(c, gin.H{"code": code, "expires_at": row.ExpiresAt})
	}
}

func channelStatuses(db *gorm.DB, userID uint) []ChannelStatus {
	var bindings []ChannelBinding
	_ = db.Where("user_id = ?", userID).Find(&bindings).Error
	byChannel := map[string]ChannelBinding{}
	for _, binding := range bindings {
		byChannel[binding.Channel] = binding
	}
	defs := []ChannelStatus{
		{Channel: ChannelInApp, Label: "站内消息", Configured: true, Bound: true, Status: "active", Description: "在通知中心准时提醒"},
		{Channel: ChannelEmail, Label: "电子邮件", Configured: emailConfigured(db), Description: "适合重要事项和较长内容"},
		{Channel: ChannelSMS, Label: "手机短信", Configured: smsConfigured(db), Description: "无需打开应用即可收到"},
		{Channel: ChannelFeishu, Label: "飞书机器人", Configured: feishuConfigured(db), Description: "由企业自建应用机器人单聊提醒"},
		{Channel: ChannelQQ, Label: "QQ 机器人", Configured: qqConfigured(db), Description: "通过 QQ 机器人主动单聊提醒"},
	}
	for i := range defs {
		if defs[i].Channel == ChannelQQ {
			// This is intentionally public metadata: it lets a new user open the
			// already-configured QQ bot before sending the one-time bind code.
			if settings, err := providerSettings(db, ChannelQQ); err == nil {
				defs[i].BotLink = settings["bot_link"]
			}
		}
		if binding, ok := byChannel[defs[i].Channel]; ok {
			defs[i].Bound = true
			defs[i].Status = binding.Status
			defs[i].TargetMasked = binding.TargetMasked
		} else if defs[i].Channel != ChannelInApp {
			defs[i].Status = "unbound"
		}
	}
	return defs
}

func normalizeTarget(channel, raw string) (string, error) {
	target := strings.TrimSpace(raw)
	switch channel {
	case ChannelEmail:
		addr, err := mail.ParseAddress(target)
		if err != nil || addr.Address != target {
			return "", errors.New("请输入有效的邮箱地址")
		}
	case ChannelSMS:
		target = strings.TrimPrefix(target, "+86")
		if !cnMobile.MatchString(target) {
			return "", errors.New("请输入有效的中国大陆手机号")
		}
	case ChannelFeishu:
		if target == "" || len(target) > 200 {
			return "", errors.New("请输入飞书工作邮箱、中国大陆手机号或 OpenID")
		}
	case ChannelQQ:
		if target == "" || len(target) > 200 {
			return "", errors.New("请输入有效的平台 OpenID")
		}
	default:
		return "", errors.New("不支持的通知渠道")
	}
	return target, nil
}

func maskTarget(channel, target string) string {
	switch channel {
	case ChannelEmail:
		parts := strings.Split(target, "@")
		if len(parts) == 2 {
			name := parts[0]
			if len(name) > 2 {
				name = name[:2] + "***"
			} else {
				name += "***"
			}
			return name + "@" + parts[1]
		}
	case ChannelSMS:
		if len(target) == 11 {
			return target[:3] + "****" + target[7:]
		}
	default:
		if len(target) > 10 {
			return target[:5] + "…" + target[len(target)-4:]
		}
	}
	return target
}

func encryptionKey(db *gorm.DB) ([]byte, error) {
	if raw := os.Getenv("REMINDER_DATA_KEY"); raw != "" {
		sum := sha256.Sum256([]byte(raw))
		return sum[:], nil
	}
	secret, err := sysconfig.GetConfig(db, "jwt_secret", 0)
	if err != nil || secret == "" {
		return nil, errors.New("missing encryption key")
	}
	sum := sha256.Sum256([]byte("reminder-channel:" + secret))
	return sum[:], nil
}

func encryptTarget(db *gorm.DB, plain string) (string, error) {
	key, err := encryptionKey(db)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.RawStdEncoding.EncodeToString(sealed), nil
}

func decryptTarget(db *gorm.DB, encoded string) (string, error) {
	key, err := encryptionKey(db)
	if err != nil {
		return "", err
	}
	raw, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("invalid encrypted target")
	}
	return stringValue(gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil))
}

func stringValue(value []byte, err error) (string, error) {
	if err != nil {
		return "", err
	}
	return string(value), nil
}

func sendChannel(ctx context.Context, db *gorm.DB, channel string, userID uint, item Reminder, idempotencyKey string) (sendResult, error) {
	if channel == ChannelInApp {
		n := Notification{UserID: userID, Type: "reminder_due", Title: item.Title, Body: notificationBody(item)}
		if item.ID > 0 {
			n.ReminderID = &item.ID
		}
		if err := db.Create(&n).Error; err != nil {
			return sendResult{}, &deliveryError{Code: "INAPP_WRITE_FAILED", Message: "写入站内通知失败"}
		}
		publishNotification(n)
		return sendResult{ExternalID: strconv.FormatUint(uint64(n.ID), 10)}, nil
	}
	var binding ChannelBinding
	if err := db.Where("user_id = ? AND channel = ? AND status = ?", userID, channel, "active").First(&binding).Error; err != nil {
		return sendResult{}, &deliveryError{Code: "CHANNEL_NOT_BOUND", Message: "该通知渠道尚未绑定或已停用", Permanent: true}
	}
	target, err := decryptTarget(db, binding.Target)
	if err != nil {
		return sendResult{}, &deliveryError{Code: "TARGET_DECRYPT_FAILED", Message: "读取渠道绑定信息失败", Permanent: true}
	}
	switch channel {
	case ChannelEmail:
		return sendEmail(db, target, item)
	case ChannelSMS:
		return sendSMSWebhook(ctx, db, target, item, idempotencyKey)
	case ChannelFeishu:
		return sendFeishu(ctx, db, target, item, idempotencyKey)
	case ChannelQQ:
		return sendQQ(ctx, db, target, item, idempotencyKey)
	default:
		return sendResult{}, &deliveryError{Code: "CHANNEL_UNSUPPORTED", Message: "不支持的通知渠道", Permanent: true}
	}
}

func notificationBody(item Reminder) string {
	if item.DueAt != nil {
		return "计划时间：" + item.DueAt.In(shanghai()).Format("01月02日 15:04")
	}
	return "你有一条新的提醒"
}

func emailConfigured(db *gorm.DB) bool {
	values, err := providerSettings(db, ChannelEmail)
	return err == nil && values["host"] != "" && values["from_address"] != ""
}
func smsConfigured(db *gorm.DB) bool {
	values, err := providerSettings(db, ChannelSMS)
	return err == nil && values["webhook_url"] != ""
}
func feishuConfigured(db *gorm.DB) bool {
	_, _, err := feishuCredentials(db)
	return err == nil
}
func qqConfigured(db *gorm.DB) bool {
	values, err := providerSettings(db, ChannelQQ)
	return err == nil && values["app_id"] != "" && values["app_secret"] != ""
}

func sendEmail(db *gorm.DB, target string, item Reminder) (sendResult, error) {
	values, err := providerSettings(db, ChannelEmail)
	if err != nil || values["host"] == "" || values["from_address"] == "" {
		return sendResult{}, &deliveryError{Code: "PROVIDER_CONFIG_INVALID", Message: "邮件服务尚未配置", Permanent: true}
	}
	host := values["host"]
	port := values["port"]
	if port == "" {
		port = "587"
	}
	from := values["from_address"]
	name := values["from_name"]
	if name == "" {
		name = "提醒事项"
	}
	subject := "提醒：" + item.Title
	body := notificationBody(item)
	if item.Notes != "" {
		body += "\r\n\r\n" + item.Notes
	}
	message := []byte("From: " + name + " <" + from + ">\r\n" +
		"To: " + target + "\r\n" +
		"Subject: =?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte(subject)) + "?=\r\n" +
		"MIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n" + body)
	var auth smtp.Auth
	if user := values["username"]; user != "" {
		auth = smtp.PlainAuth("", user, values["password"], host)
	}
	if err := sendSMTP(host, port, auth, from, []string{target}, message); err != nil {
		return sendResult{}, &deliveryError{Code: "SMTP_SEND_FAILED", Message: smtpErrorMessage(host, err)}
	}
	return sendResult{}, nil
}

func smtpErrorMessage(host string, err error) string {
	detail := strings.ToLower(err.Error())
	if strings.Contains(detail, "535") || strings.Contains(detail, "authentication failed") || strings.Contains(detail, "login fail") {
		if strings.EqualFold(host, "smtp.qq.com") {
			return "QQ 邮箱 SMTP 登录失败：请先在 QQ 邮箱“设置 → 账号”中开启 POP3/SMTP 或 IMAP/SMTP 服务，用户名填写完整邮箱，并使用新生成的授权码（不是 QQ/邮箱登录密码）"
		}
		return "SMTP 登录失败：请检查用户名，并使用邮件服务商提供的 SMTP 授权码或应用专用密码"
	}
	return "邮件发送失败：" + safeError(err)
}

// Most Chinese mailbox providers use either STARTTLS on 587 or implicit TLS
// on 465. net/smtp's SendMail covers the former only, so support both here.
func sendSMTP(host, port string, auth smtp.Auth, from string, recipients []string, message []byte) error {
	if port != "465" {
		return smtp.SendMail(host+":"+port, auth, from, recipients, message)
	}
	conn, err := tls.Dial("tcp", host+":"+port, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
	if err != nil {
		return err
	}
	defer conn.Close()
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer client.Quit()
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return err
		}
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return err
		}
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write(message); err != nil {
		_ = writer.Close()
		return err
	}
	return writer.Close()
}

func sendSMSWebhook(ctx context.Context, db *gorm.DB, target string, item Reminder, key string) (sendResult, error) {
	values, _ := providerSettings(db, ChannelSMS)
	endpoint := values["webhook_url"]
	if endpoint == "" {
		return sendResult{}, &deliveryError{Code: "PROVIDER_CONFIG_INVALID", Message: "短信服务尚未配置", Permanent: true}
	}
	payload := map[string]interface{}{
		"phone": target, "title": truncateRunes(item.Title, 32),
		"due_at": item.DueAt, "idempotency_key": key,
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := postJSON(ctx, endpoint, values["webhook_token"], payload, &out); err != nil {
		return sendResult{}, err
	}
	return sendResult{ExternalID: out.ID}, nil
}

func sendFeishu(ctx context.Context, db *gorm.DB, openID string, item Reminder, idempotencyKey string) (sendResult, error) {
	base, token, err := feishuTenantAccessToken(ctx, db)
	if err != nil {
		return sendResult{}, err
	}
	content, _ := json.Marshal(map[string]string{"text": robotNotificationText(db, item)})
	var sendResp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			MessageID string `json:"message_id"`
		} `json:"data"`
	}
	// Feishu uses uuid to de-duplicate a repeated submission of one message;
	// its API limits this field to 50 characters.
	if len(idempotencyKey) > 50 {
		idempotencyKey = idempotencyKey[:50]
	}
	endpoint := base + "/open-apis/im/v1/messages?receive_id_type=open_id&uuid=" + url.QueryEscape(idempotencyKey)
	if err := postJSON(ctx, endpoint, "Bearer "+token, map[string]interface{}{
		"receive_id": openID, "msg_type": "text", "content": string(content),
	}, &sendResp); err != nil {
		return sendResult{}, doNotRetryExternalSend(err)
	}
	if sendResp.Code != 0 {
		permanent := sendResp.Code >= 230000 && sendResp.Code < 240000
		return sendResult{}, &deliveryError{Code: "FEISHU_SEND_FAILED", Message: "飞书发送失败：" + sendResp.Msg, Permanent: permanent}
	}
	return sendResult{ExternalID: sendResp.Data.MessageID}, nil
}

func resolveFeishuOpenID(ctx context.Context, db *gorm.DB, identifier string) (string, error) {
	identifier = strings.TrimSpace(identifier)
	if strings.HasPrefix(identifier, "ou_") {
		return identifier, nil
	}

	var emails, mobiles []string
	if addr, err := mail.ParseAddress(identifier); err == nil && addr.Address == identifier {
		emails = []string{identifier}
	} else {
		mobile := strings.TrimPrefix(identifier, "+86")
		if !cnMobile.MatchString(mobile) {
			return "", errors.New("请输入飞书工作邮箱、中国大陆手机号或 OpenID")
		}
		mobiles = []string{mobile}
	}

	base, token, err := feishuTenantAccessToken(ctx, db)
	if err != nil {
		return "", err
	}
	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			UserList []struct {
				UserID string `json:"user_id"`
			} `json:"user_list"`
		} `json:"data"`
	}
	endpoint := base + "/open-apis/contact/v3/users/batch_get_id?user_id_type=open_id"
	if err := postJSON(ctx, endpoint, "Bearer "+token, map[string]interface{}{"emails": emails, "mobiles": mobiles}, &result); err != nil {
		return "", err
	}
	if result.Code != 0 {
		return "", &deliveryError{Code: "FEISHU_USER_LOOKUP_FAILED", Message: "无法查询飞书用户：" + result.Msg, Permanent: true}
	}
	if len(result.Data.UserList) != 1 || !strings.HasPrefix(result.Data.UserList[0].UserID, "ou_") {
		return "", &deliveryError{Code: "FEISHU_USER_NOT_FOUND", Message: "未找到该飞书用户，请确认应用可见范围和通讯录权限", Permanent: true}
	}
	return result.Data.UserList[0].UserID, nil
}

func feishuTenantAccessToken(ctx context.Context, db *gorm.DB) (string, string, error) {
	appID, appSecret, err := feishuCredentials(db)
	if err != nil {
		return "", "", &deliveryError{Code: "PROVIDER_CONFIG_INVALID", Message: "飞书服务尚未由管理员开通", Permanent: true}
	}
	base := strings.TrimRight(os.Getenv("FEISHU_BASE_URL"), "/")
	if base == "" {
		base = "https://open.feishu.cn"
	}
	var tokenResp struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
	}
	if err := postJSON(ctx, base+"/open-apis/auth/v3/tenant_access_token/internal", "", map[string]string{
		"app_id": appID, "app_secret": appSecret,
	}, &tokenResp); err != nil {
		return "", "", err
	}
	if tokenResp.Code != 0 || tokenResp.TenantAccessToken == "" {
		return "", "", &deliveryError{Code: "FEISHU_TOKEN_FAILED", Message: "获取飞书访问凭证失败：" + tokenResp.Msg}
	}
	return base, tokenResp.TenantAccessToken, nil
}

// Database settings take priority so an administrator can finish setup from
// the app. Environment variables remain a backwards-compatible deployment
// option for automated installations.
func feishuCredentials(db *gorm.DB) (string, string, error) {
	var row ProviderConfig
	if db != nil {
		if err := db.Where("provider = ?", ChannelFeishu).First(&row).Error; err == nil {
			plain, decErr := decryptTarget(db, row.Secret)
			if decErr != nil {
				return "", "", decErr
			}
			parts := strings.SplitN(plain, "\n", 2)
			if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
				return parts[0], parts[1], nil
			}
		}
	}
	appID, appSecret := os.Getenv("FEISHU_APP_ID"), os.Getenv("FEISHU_APP_SECRET")
	if appID == "" || appSecret == "" {
		return "", "", errors.New("feishu not configured")
	}
	return appID, appSecret, nil
}

func saveProviderSettings(db *gorm.DB, provider string, values map[string]string) error {
	raw, err := json.Marshal(values)
	if err != nil {
		return err
	}
	secret, err := encryptTarget(db, string(raw))
	if err != nil {
		return err
	}
	row := ProviderConfig{Provider: provider, Secret: secret}
	return db.Where("provider = ?", provider).Assign(map[string]interface{}{"secret": secret}).FirstOrCreate(&row).Error
}

func providerSettings(db *gorm.DB, provider string) (map[string]string, error) {
	if db != nil {
		var row ProviderConfig
		if err := db.Where("provider = ?", provider).First(&row).Error; err == nil {
			plain, decErr := decryptTarget(db, row.Secret)
			if decErr != nil {
				return nil, decErr
			}
			values := map[string]string{}
			if err := json.Unmarshal([]byte(plain), &values); err == nil {
				return values, nil
			}
		}
	}
	values := map[string]string{}
	switch provider {
	case ChannelEmail:
		values = map[string]string{"host": os.Getenv("SMTP_HOST"), "port": os.Getenv("SMTP_PORT"), "from_address": os.Getenv("SMTP_FROM_ADDRESS"), "from_name": os.Getenv("SMTP_FROM_NAME"), "username": os.Getenv("SMTP_USERNAME"), "password": os.Getenv("SMTP_PASSWORD")}
	case ChannelSMS:
		values = map[string]string{"webhook_url": os.Getenv("SMS_WEBHOOK_URL"), "webhook_token": os.Getenv("SMS_WEBHOOK_TOKEN")}
	case ChannelQQ:
		values = map[string]string{"app_id": os.Getenv("QQ_BOT_APP_ID"), "app_secret": os.Getenv("QQ_BOT_APP_SECRET"), "api_base": os.Getenv("QQ_BOT_API_BASE")}
	}
	return values, nil
}

func notificationBrand(db *gorm.DB) string {
	values, err := providerSettings(db, "notification_brand")
	if err == nil && strings.TrimSpace(values["name"]) != "" {
		return strings.TrimSpace(values["name"])
	}
	return "提醒事项"
}

func robotNotificationText(db *gorm.DB, item Reminder) string {
	text := "【" + notificationBrand(db) + "】\n⏰ " + item.Title + "\n" + notificationBody(item)
	if item.Notes != "" {
		text += "\n" + item.Notes
	}
	return text
}

// A timeout or 5xx may occur after the robot has accepted a message. Retrying
// would then deliver duplicate reminders, so robot sends are deliberately
// recorded as a single attempt and shown as a visible channel error instead.
func doNotRetryExternalSend(err error) error {
	var classified *deliveryError
	if errors.As(err, &classified) {
		return &deliveryError{Code: classified.Code, Message: classified.Message + "（为避免重复发送，未自动重试）", Permanent: true}
	}
	return &deliveryError{Code: "PROVIDER_SEND_UNCERTAIN", Message: "机器人发送结果不确定（为避免重复发送，未自动重试）", Permanent: true}
}

func sendQQ(ctx context.Context, db *gorm.DB, openID string, item Reminder, _ string) (sendResult, error) {
	values, err := providerSettings(db, ChannelQQ)
	if err != nil || values["app_id"] == "" || values["app_secret"] == "" {
		return sendResult{}, &deliveryError{Code: "PROVIDER_CONFIG_INVALID", Message: "QQ 机器人尚未配置", Permanent: true}
	}
	token, err := qqAccessToken(ctx, values)
	if err != nil {
		return sendResult{}, err
	}
	result, err := sendQQMessage(ctx, token, values, openID, robotNotificationText(db, item))
	if err != nil {
		return sendResult{}, doNotRetryExternalSend(err)
	}
	return result, nil
}

func qqAccessToken(ctx context.Context, values map[string]string) (string, error) {
	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   string `json:"expires_in"`
	}
	if err := postJSON(ctx, "https://bots.qq.com/app/getAppAccessToken", "", map[string]string{
		"appId": values["app_id"], "clientSecret": values["app_secret"],
	}, &tokenResp); err != nil {
		return "", err
	}
	if tokenResp.AccessToken == "" {
		return "", &deliveryError{Code: "QQ_TOKEN_FAILED", Message: "获取 QQ 机器人访问凭证失败"}
	}
	return tokenResp.AccessToken, nil
}

func sendQQText(ctx context.Context, token string, values map[string]string, openID, content string) error {
	_, err := sendQQMessage(ctx, token, values, openID, content)
	return err
}

func sendQQMessage(ctx context.Context, token string, values map[string]string, openID, content string) (sendResult, error) {
	base := strings.TrimRight(values["api_base"], "/")
	if base == "" {
		base = "https://api.bot.qq.com"
	}
	endpoint := base + "/v2/users/" + url.PathEscape(openID) + "/messages"
	var out struct {
		ID string `json:"id"`
	}
	if err := postJSON(ctx, endpoint, "QQBot "+token, map[string]interface{}{
		"content": content, "msg_type": 0,
	}, &out); err != nil {
		return sendResult{}, err
	}
	return sendResult{ExternalID: out.ID}, nil
}

func postJSON(ctx context.Context, endpoint, token string, payload, out interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		if strings.Contains(token, " ") {
			req.Header.Set("Authorization", token)
		} else {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}
	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return &deliveryError{Code: "PROVIDER_UNAVAILABLE", Message: "通知服务暂时不可用：" + safeError(err)}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		permanent := resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests
		return &deliveryError{Code: "PROVIDER_HTTP_" + strconv.Itoa(resp.StatusCode), Message: fmt.Sprintf("通知服务返回 %d", resp.StatusCode), Permanent: permanent}
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return &deliveryError{Code: "PROVIDER_RESPONSE_INVALID", Message: "通知服务响应格式无效"}
		}
	}
	return nil
}

func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}

func safeError(err error) string {
	text := err.Error()
	if len(text) > 160 {
		return text[:160]
	}
	return text
}

func shanghai() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*60*60)
	}
	return loc
}
