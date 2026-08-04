package reminder

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"smallgo/server/logger"

	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

const qqC2CIntent = 1 << 25 // GROUP_AND_C2C_EVENT

var qqGatewayState struct {
	sync.Mutex
	running bool
}

// ensureQQGateway is called by the scheduler. It deliberately uses QQ's
// outbound gateway connection, so localhost:8906 needs no public callback.
func ensureQQGateway(db *gorm.DB) {
	if !qqConfigured(db) {
		return
	}
	qqGatewayState.Lock()
	if qqGatewayState.running {
		qqGatewayState.Unlock()
		return
	}
	qqGatewayState.running = true
	qqGatewayState.Unlock()
	go func() {
		defer func() { qqGatewayState.Lock(); qqGatewayState.running = false; qqGatewayState.Unlock() }()
		runQQGateway(db)
	}()
}

func runQQGateway(db *gorm.DB) {
	for {
		settings, err := providerSettings(db, ChannelQQ)
		if err != nil || settings["app_id"] == "" || settings["app_secret"] == "" {
			return
		}
		if err := connectQQGateway(db, settings); err != nil {
			logger.Warn("QQ gateway disconnected: %v", err)
		}
		time.Sleep(5 * time.Second)
	}
}

func connectQQGateway(db *gorm.DB, settings map[string]string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	token, err := qqAccessToken(ctx, settings)
	if err != nil {
		return err
	}
	// QQ's gateway is separate from the v2 HTTP message API. Keeping this
	// official address fixed avoids a user-entered message API base breaking
	// the bot's online connection.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.sgroup.qq.com/gateway", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "QQBot "+token)
	res, err := (&http.Client{Timeout: 12 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	var gateway struct {
		URL string `json:"url"`
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("gateway request returned %d", res.StatusCode)
	}
	if err := json.NewDecoder(res.Body).Decode(&gateway); err != nil {
		return err
	}
	if gateway.URL == "" {
		return fmt.Errorf("QQ gateway URL is empty")
	}
	conn, _, err := websocket.DefaultDialer.Dial(gateway.URL, nil)
	if err != nil {
		return err
	}
	defer conn.Close()
	var hello struct {
		Op int `json:"op"`
		D  struct {
			HeartbeatInterval int `json:"heartbeat_interval"`
		} `json:"d"`
	}
	if err := conn.ReadJSON(&hello); err != nil {
		return err
	}
	if hello.Op != 10 || hello.D.HeartbeatInterval <= 0 {
		return fmt.Errorf("unexpected QQ gateway hello")
	}
	identify := map[string]interface{}{"op": 2, "d": map[string]interface{}{"token": "QQBot " + token, "intents": qqC2CIntent, "shard": []int{0, 1}}}
	if err := conn.WriteJSON(identify); err != nil {
		return err
	}
	logger.Info("QQ gateway connected; waiting for private messages")
	var writeMu sync.Mutex
	var sequence interface{}
	done := make(chan struct{})
	defer close(done)
	go func() {
		ticker := time.NewTicker(time.Duration(hello.D.HeartbeatInterval) * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				writeMu.Lock()
				_ = conn.WriteJSON(map[string]interface{}{"op": 1, "d": sequence})
				writeMu.Unlock()
			}
		}
	}()
	for {
		var packet struct {
			Op int             `json:"op"`
			D  json.RawMessage `json:"d"`
			S  interface{}     `json:"s"`
			T  string          `json:"t"`
		}
		if err := conn.ReadJSON(&packet); err != nil {
			return err
		}
		if packet.S != nil {
			sequence = packet.S
		}
		if packet.Op == 0 && packet.T == "C2C_MESSAGE_CREATE" {
			handleQQC2CMessage(db, token, settings, packet.D)
		}
	}
}

func handleQQC2CMessage(db *gorm.DB, token string, settings map[string]string, raw json.RawMessage) {
	var event struct {
		Content string `json:"content"`
		ID      string `json:"id"`
		Author  struct {
			OpenID string `json:"user_openid"`
		} `json:"author"`
	}
	if err := json.Unmarshal(raw, &event); err != nil || event.Author.OpenID == "" {
		return
	}
	parts := strings.Fields(strings.TrimSpace(event.Content))
	if len(parts) != 2 || parts[0] != "/绑定" {
		return
	}
	var code QQBindCode
	if err := db.Where("code = ? AND expires_at > ?", parts[1], time.Now()).First(&code).Error; err != nil {
		_ = sendQQText(context.Background(), token, settings, event.Author.OpenID, "绑定码无效或已过期，请回到 Reminder 重新获取。")
		return
	}
	encrypted, err := encryptTarget(db, event.Author.OpenID)
	if err != nil {
		return
	}
	now := time.Now()
	binding := ChannelBinding{UserID: code.UserID, Channel: ChannelQQ, Target: encrypted, TargetMasked: maskTarget(ChannelQQ, event.Author.OpenID), Status: "active", VerifiedAt: &now}
	if err := db.Where("user_id = ? AND channel = ?", code.UserID, ChannelQQ).Assign(map[string]interface{}{"target": encrypted, "target_masked": binding.TargetMasked, "status": "active", "verified_at": &now, "last_error_code": ""}).FirstOrCreate(&binding).Error; err != nil {
		return
	}
	_ = db.Delete(&code).Error
	_ = sendQQText(context.Background(), token, settings, event.Author.OpenID, "绑定成功！之后到点的提醒会发送到这里。")
}

func newQQBindCode() (string, error) {
	var raw [4]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", (uint32(raw[0])<<24|uint32(raw[1])<<16|uint32(raw[2])<<8|uint32(raw[3]))%1000000), nil
}
