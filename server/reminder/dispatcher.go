package reminder

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"smallgo/server/logger"

	"gorm.io/gorm"
)

var dispatchMu sync.Mutex

func dispatchDueJobs(db *gorm.DB) {
	if !dispatchMu.TryLock() {
		return
	}
	defer dispatchMu.Unlock()

	// Delivery jobs are persisted as UTC instants. SQLite compares timestamp
	// values as text in this query, so mixing local and UTC offsets can make a
	// future reminder appear due immediately.
	now := time.Now().UTC()
	staleBefore := now.Add(-5 * time.Minute)
	_ = db.Model(&DeliveryJob{}).
		Where("status = ? AND locked_at < ?", "processing", staleBefore).
		Updates(map[string]interface{}{"status": "retry", "next_attempt_at": now, "worker_id": "", "locked_at": nil}).Error

	var jobs []DeliveryJob
	if err := db.Where(
		"(status = ? AND run_at <= ?) OR (status = ? AND next_attempt_at <= ?)",
		"pending", now, "retry", now,
	).Order("run_at ASC, id ASC").Limit(100).Find(&jobs).Error; err != nil {
		logger.Error("reminder: scan delivery jobs: %v", err)
		return
	}
	for _, job := range jobs {
		processJob(db, job)
	}
}

func processJob(db *gorm.DB, candidate DeliveryJob) {
	now := time.Now().UTC()
	workerID := fmt.Sprintf("local-%d", now.UnixNano())
	result := db.Model(&DeliveryJob{}).
		Where("id = ? AND status IN ?", candidate.ID, []string{"pending", "retry"}).
		Updates(map[string]interface{}{"status": "processing", "locked_at": &now, "worker_id": workerID})
	if result.Error != nil || result.RowsAffected != 1 {
		return
	}

	var job DeliveryJob
	if err := db.First(&job, candidate.ID).Error; err != nil {
		return
	}
	var item Reminder
	if err := db.Where("id = ? AND user_id = ? AND deleted_at IS NULL", job.ReminderID, job.UserID).First(&item).Error; err != nil {
		finishCancelled(db, job.ID, "REMINDER_NOT_FOUND")
		return
	}
	if item.CompletedAt != nil || item.DueAt == nil || !sameInstant(*item.DueAt, job.ScheduledFor) {
		finishCancelled(db, job.ID, "REMINDER_CHANGED")
		return
	}
	var link ReminderChannel
	if err := db.Where("reminder_id = ? AND user_id = ? AND channel = ? AND enabled = ?", item.ID, item.UserID, job.Channel, true).First(&link).Error; err != nil {
		finishCancelled(db, job.ID, "CHANNEL_DISABLED")
		return
	}

	started := time.Now().UTC()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	sendResult, err := sendChannel(ctx, db, job.Channel, job.UserID, item, job.IdempotencyKey)
	cancel()
	finished := time.Now().UTC()
	attemptNo := job.AttemptCount + 1

	attempt := DeliveryAttempt{
		JobID: job.ID, AttemptNo: attemptNo, StartedAt: started, FinishedAt: finished,
		LatencyMS: finished.Sub(started).Milliseconds(),
	}
	if err == nil {
		attempt.Result = "success"
		attempt.ResponseSummary = "sent"
		_ = db.Transaction(func(tx *gorm.DB) error {
			if e := tx.Create(&attempt).Error; e != nil {
				return e
			}
			return tx.Model(&DeliveryJob{}).Where("id = ?", job.ID).Updates(map[string]interface{}{
				"status": "succeeded", "attempt_count": attemptNo,
				"external_message_id": sendResult.ExternalID, "locked_at": nil, "worker_id": "",
				"last_error_code": "", "last_error_message": "",
			}).Error
		})
		if job.Channel != ChannelInApp {
			_ = db.Model(&ChannelBinding{}).Where("user_id = ? AND channel = ?", job.UserID, job.Channel).Updates(map[string]interface{}{"last_error_code": "", "last_error_at": nil}).Error
		}
		return
	}

	var classified *deliveryError
	if !errors.As(err, &classified) {
		classified = &deliveryError{Code: "DELIVERY_FAILED", Message: safeError(err)}
	}
	if job.Channel != ChannelInApp {
		_ = db.Model(&ChannelBinding{}).Where("user_id = ? AND channel = ?", job.UserID, job.Channel).Updates(map[string]interface{}{"last_error_code": classified.Code + "：" + truncateRunes(classified.Message, 100), "last_error_at": &finished}).Error
	}
	attempt.ProviderCode = classified.Code
	attempt.ResponseSummary = truncateRunes(classified.Message, 150)
	if classified.Permanent || attemptNo >= 3 {
		attempt.Result = "permanent"
		var failureNotification *Notification
		txErr := db.Transaction(func(tx *gorm.DB) error {
			if e := tx.Create(&attempt).Error; e != nil {
				return e
			}
			if e := tx.Model(&DeliveryJob{}).Where("id = ?", job.ID).Updates(map[string]interface{}{
				"status": "failed", "attempt_count": attemptNo, "locked_at": nil, "worker_id": "",
				"last_error_code": classified.Code, "last_error_message": truncateRunes(classified.Message, 280),
			}).Error; e != nil {
				return e
			}
			if job.Channel != ChannelInApp {
				n := Notification{
					UserID: job.UserID, ReminderID: &job.ReminderID, Type: "channel_failed",
					Title: "“" + truncateRunes(item.Title, 40) + "”发送失败",
					Body:  channelLabel(job.Channel) + "：" + truncateRunes(classified.Message, 120),
				}
				if e := tx.Create(&n).Error; e != nil {
					return e
				}
				failureNotification = &n
			}
			return nil
		})
		if txErr == nil && failureNotification != nil {
			publishNotification(*failureNotification)
		}
		return
	}

	attempt.Result = "transient"
	delays := []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute}
	next := time.Now().UTC().Add(delays[attemptNo-1])
	_ = db.Transaction(func(tx *gorm.DB) error {
		if e := tx.Create(&attempt).Error; e != nil {
			return e
		}
		return tx.Model(&DeliveryJob{}).Where("id = ?", job.ID).Updates(map[string]interface{}{
			"status": "retry", "attempt_count": attemptNo, "next_attempt_at": &next,
			"locked_at": nil, "worker_id": "", "last_error_code": classified.Code,
			"last_error_message": truncateRunes(classified.Message, 280),
		}).Error
	})
}

func finishCancelled(db *gorm.DB, jobID uint, code string) {
	_ = db.Model(&DeliveryJob{}).Where("id = ?", jobID).Updates(map[string]interface{}{
		"status": "cancelled", "locked_at": nil, "worker_id": "", "last_error_code": code,
	}).Error
}

func sameInstant(a, b time.Time) bool {
	return a.UTC().Equal(b.UTC())
}

func channelLabel(channel string) string {
	switch channel {
	case ChannelEmail:
		return "电子邮件"
	case ChannelSMS:
		return "手机短信"
	case ChannelFeishu:
		return "飞书机器人"
	case ChannelQQ:
		return "QQ 机器人"
	default:
		return "站内消息"
	}
}
