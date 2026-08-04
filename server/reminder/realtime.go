package reminder

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type realtimeEvent struct {
	Type         string        `json:"type"`
	Notification *Notification `json:"notification,omitempty"`
	SentAt       time.Time     `json:"sent_at"`
}

type realtimeBroker struct {
	mu      sync.RWMutex
	clients map[uint]map[chan realtimeEvent]struct{}
}

var reminderRealtime = realtimeBroker{
	clients: make(map[uint]map[chan realtimeEvent]struct{}),
}

func (b *realtimeBroker) subscribe(userID uint) (<-chan realtimeEvent, func()) {
	events := make(chan realtimeEvent, 8)
	b.mu.Lock()
	if b.clients[userID] == nil {
		b.clients[userID] = make(map[chan realtimeEvent]struct{})
	}
	b.clients[userID][events] = struct{}{}
	b.mu.Unlock()

	return events, func() {
		b.mu.Lock()
		delete(b.clients[userID], events)
		if len(b.clients[userID]) == 0 {
			delete(b.clients, userID)
		}
		b.mu.Unlock()
	}
}

func (b *realtimeBroker) publish(userID uint, event realtimeEvent) {
	if event.SentAt.IsZero() {
		event.SentAt = time.Now()
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for client := range b.clients[userID] {
		select {
		case client <- event:
		default:
			// Keep the newest state change if a slow browser has filled its
			// small buffer. The browser refreshes canonical data after events.
			select {
			case <-client:
			default:
			}
			select {
			case client <- event:
			default:
			}
		}
	}
}

func publishNotification(notification Notification) {
	reminderRealtime.publish(notification.UserID, realtimeEvent{
		Type:         "notification.created",
		Notification: &notification,
	})
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

		events, unsubscribe := reminderRealtime.subscribe(currentUserID(c))
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
