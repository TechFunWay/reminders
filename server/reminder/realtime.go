package reminder

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"smallgo/server/response"

	"github.com/gin-gonic/gin"
)

// realtimeEvent 是推给浏览器的一条实时事件，两类载荷：
//
//	notification.created —— 站内通知（提醒到点、渠道投递失败），前端据此弹窗提醒；
//	reminders.changed    —— 数据变更（提醒、清单、通知已读、渠道绑定），用于跨设备
//	                        自动刷新：手机端加了提醒，PC 端不必手动刷新就能看到。
//
// Scope/Action/TargetID 描述「哪类数据、发生了什么、哪一条」，Revision 是该用户的
// 变更修订号（见 publishDataChanged）。
type realtimeEvent struct {
	Type         string        `json:"type"`
	Notification *Notification `json:"notification,omitempty"`
	Scope        string        `json:"scope,omitempty"`
	Action       string        `json:"action,omitempty"`
	TargetID     uint          `json:"target_id,omitempty"`
	Revision     uint64        `json:"revision,omitempty"`
	SentAt       time.Time     `json:"sent_at"`
}

// realtimeClient 是一条 SSE 长连接。clientID 是浏览器标签页标识，用于发布时跳过
// 发起变更的那一个标签页。
type realtimeClient struct {
	events   chan realtimeEvent
	clientID string
}

type realtimeBroker struct {
	mu      sync.RWMutex
	clients map[uint]map[*realtimeClient]struct{}
	// revision 是每个用户的变更修订号，单调递增，用于轮询兜底（见 currentRevision）。
	revision map[uint]uint64
}

var reminderRealtime = realtimeBroker{
	clients:  make(map[uint]map[*realtimeClient]struct{}),
	revision: make(map[uint]uint64),
}

func (b *realtimeBroker) subscribe(userID uint, clientID string) (<-chan realtimeEvent, func()) {
	client := &realtimeClient{events: make(chan realtimeEvent, 8), clientID: clientID}
	b.mu.Lock()
	if b.clients[userID] == nil {
		b.clients[userID] = make(map[*realtimeClient]struct{})
	}
	b.clients[userID][client] = struct{}{}
	b.mu.Unlock()

	return client.events, func() {
		b.mu.Lock()
		delete(b.clients[userID], client)
		if len(b.clients[userID]) == 0 {
			delete(b.clients, userID)
		}
		b.mu.Unlock()
	}
}

// publish 把事件发给该用户的所有在线标签页，originClientID 对应的那一个除外。
func (b *realtimeBroker) publish(userID uint, event realtimeEvent, originClientID string) {
	if event.SentAt.IsZero() {
		event.SentAt = time.Now()
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for client := range b.clients[userID] {
		// 跳过发起变更的标签页：它已经在本地更新过界面，再收一次自己的回声只会
		// 白白多刷一遍，弱网下还可能与正在编辑的表单抢状态。
		if originClientID != "" && client.clientID == originClientID {
			continue
		}
		select {
		case client.events <- event:
		default:
			// Keep the newest state change if a slow browser has filled its
			// small buffer. The browser refreshes canonical data after events.
			select {
			case <-client.events:
			default:
			}
			select {
			case client.events <- event:
			default:
			}
		}
	}
}

func (b *realtimeBroker) bumpRevision(userID uint) uint64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.revision[userID]++
	return b.revision[userID]
}

func (b *realtimeBroker) currentRevision(userID uint) uint64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.revision[userID]
}

func publishNotification(notification Notification) {
	// 新通知也会改变通知中心与未读数，修订号同样要推进，轮询兜底才看得到。
	revision := reminderRealtime.bumpRevision(notification.UserID)
	reminderRealtime.publish(notification.UserID, realtimeEvent{
		Type:         "notification.created",
		Notification: &notification,
		Revision:     revision,
	}, "")
}

// publishDataChanged 广播一次数据变更并推进修订号。
//
// 修订号是跨设备同步的兜底：SSE 长连接要穿过飞牛统一网关那层代理，万一被缓冲，
// 事件就迟迟到不了前端。前端定时问一次修订号，发现变了就重新拉数据——不依赖长
// 连接一定通畅，也能做到「另一台设备改过了，这边自动跟上」。
func publishDataChanged(userID uint, scope, action string, targetID uint, originClientID string) {
	if userID == 0 {
		return
	}
	revision := reminderRealtime.bumpRevision(userID)
	reminderRealtime.publish(userID, realtimeEvent{
		Type:     "reminders.changed",
		Scope:    scope,
		Action:   action,
		TargetID: targetID,
		Revision: revision,
	}, originClientID)
}

// handleRevision 是跨设备同步的轻量探针：只回一个数字，前端定时问「我这边的数据
// 还是最新的吗」。比拉全量列表便宜得多，手机端和弱网下都不心疼。
func handleRevision() gin.HandlerFunc {
	return func(c *gin.Context) {
		response.Success(c, gin.H{"revision": reminderRealtime.currentRevision(currentUserID(c))})
	}
}

func handleRealtimeEvents() gin.HandlerFunc {
	return func(c *gin.Context) {
		flusher, ok := c.Writer.(http.Flusher)
		if !ok {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "当前连接不支持实时推送"})
			return
		}

		c.Header("Content-Type", "text/event-stream; charset=utf-8")
		c.Header("Cache-Control", "no-cache, no-transform")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")
		c.Status(http.StatusOK)

		events, unsubscribe := reminderRealtime.subscribe(currentUserID(c), strings.TrimSpace(c.Query("client_id")))
		defer unsubscribe()

		ready := realtimeEvent{Type: "connected", SentAt: time.Now()}
		if err := writeSSE(c.Writer, ready); err != nil {
			return
		}
		flusher.Flush()

		heartbeat := time.NewTicker(20 * time.Second)
		defer heartbeat.Stop()
		for {
			select {
			case <-c.Request.Context().Done():
				return
			case event := <-events:
				if err := writeSSE(c.Writer, event); err != nil {
					return
				}
				flusher.Flush()
			case <-heartbeat.C:
				if _, err := fmt.Fprint(c.Writer, ": heartbeat\n\n"); err != nil {
					return
				}
				flusher.Flush()
			}
		}
	}
}

func writeSSE(w http.ResponseWriter, event realtimeEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: reminder\ndata: %s\n\n", data)
	return err
}
