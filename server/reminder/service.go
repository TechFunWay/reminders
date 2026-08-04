package reminder

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var repeatRules = map[string]bool{
	"none": true, "daily": true, "weekly": true, "monthly": true, "yearly": true,
}

type SaveReminderInput struct {
	Title      string     `json:"title"`
	Notes      string     `json:"notes"`
	ListID     uint       `json:"list_id"`
	Priority   int        `json:"priority"`
	DueAt      *time.Time `json:"due_at"`
	AllDay     bool       `json:"all_day"`
	RepeatRule string     `json:"repeat_rule"`
	Channels   []string   `json:"channels"`
	Version    uint       `json:"version"`
}

func ensureDefaultList(tx *gorm.DB, userID uint) (List, error) {
	var list List
	err := tx.Where("user_id = ? AND is_default = ? AND deleted_at IS NULL", userID, true).First(&list).Error
	if err == nil {
		return list, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return list, err
	}
	list = List{UserID: userID, Name: "提醒", Color: "blue", Icon: "list", IsDefault: true}
	if err := tx.Create(&list).Error; err != nil {
		// Another request may have created it between lookup and insert.
		if findErr := tx.Where("user_id = ? AND is_default = ? AND deleted_at IS NULL", userID, true).First(&list).Error; findErr == nil {
			return list, nil
		}
		return list, err
	}
	return list, nil
}

func validateReminderInput(in *SaveReminderInput) error {
	in.Title = strings.TrimSpace(in.Title)
	in.Notes = strings.TrimSpace(in.Notes)
	if in.Title == "" {
		return errors.New("请输入提醒标题")
	}
	if len([]rune(in.Title)) > 200 {
		return errors.New("提醒标题不能超过 200 个字符")
	}
	if len([]rune(in.Notes)) > 5000 {
		return errors.New("备注不能超过 5000 个字符")
	}
	if in.Priority < 0 || in.Priority > 3 {
		return errors.New("优先级无效")
	}
	if in.RepeatRule == "" {
		in.RepeatRule = "none"
	}
	if !repeatRules[in.RepeatRule] {
		return errors.New("重复规则无效")
	}
	if len(in.Channels) == 0 {
		in.Channels = []string{ChannelInApp}
	}
	seen := map[string]bool{}
	clean := make([]string, 0, len(in.Channels))
	for _, ch := range in.Channels {
		ch = strings.ToLower(strings.TrimSpace(ch))
		if !supportedChannels[ch] {
			return fmt.Errorf("不支持的提醒渠道：%s", ch)
		}
		if !seen[ch] {
			seen[ch] = true
			clean = append(clean, ch)
		}
	}
	sort.Strings(clean)
	in.Channels = clean
	return nil
}

func createReminder(db *gorm.DB, userID uint, in SaveReminderInput) (ReminderDTO, error) {
	if err := validateReminderInput(&in); err != nil {
		return ReminderDTO{}, err
	}
	var created Reminder
	err := db.Transaction(func(tx *gorm.DB) error {
		if in.ListID == 0 {
			list, err := ensureDefaultList(tx, userID)
			if err != nil {
				return err
			}
			in.ListID = list.ID
		}
		if err := assertListOwner(tx, userID, in.ListID); err != nil {
			return err
		}
		created = Reminder{
			UserID: userID, ListID: in.ListID, Title: in.Title, Notes: in.Notes,
			Priority: in.Priority, DueAt: in.DueAt, AllDay: in.AllDay,
			RepeatRule: in.RepeatRule, Version: 1,
		}
		if err := tx.Create(&created).Error; err != nil {
			return err
		}
		if err := replaceChannels(tx, created, in.Channels); err != nil {
			return err
		}
		return rebuildJobs(tx, created, in.Channels)
	})
	if err != nil {
		return ReminderDTO{}, err
	}
	return getReminder(db, userID, created.ID)
}

func updateReminder(db *gorm.DB, userID, reminderID uint, in SaveReminderInput) (ReminderDTO, error) {
	if err := validateReminderInput(&in); err != nil {
		return ReminderDTO{}, err
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		var existing Reminder
		q := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ? AND deleted_at IS NULL", reminderID, userID)
		if err := q.First(&existing).Error; err != nil {
			return err
		}
		if in.Version > 0 && existing.Version != in.Version {
			return errVersionConflict
		}
		if in.ListID == 0 {
			in.ListID = existing.ListID
		}
		if err := assertListOwner(tx, userID, in.ListID); err != nil {
			return err
		}
		existing.Title = in.Title
		existing.Notes = in.Notes
		existing.ListID = in.ListID
		existing.Priority = in.Priority
		existing.DueAt = in.DueAt
		existing.AllDay = in.AllDay
		existing.RepeatRule = in.RepeatRule
		existing.SnoozedUntil = nil
		existing.Version++
		if err := tx.Save(&existing).Error; err != nil {
			return err
		}
		if err := replaceChannels(tx, existing, in.Channels); err != nil {
			return err
		}
		return rebuildJobs(tx, existing, in.Channels)
	})
	if err != nil {
		return ReminderDTO{}, err
	}
	return getReminder(db, userID, reminderID)
}

var errVersionConflict = errors.New("提醒已在其他位置更新，请刷新后重试")

func assertListOwner(tx *gorm.DB, userID, listID uint) error {
	var count int64
	if err := tx.Model(&List{}).Where("id = ? AND user_id = ? AND deleted_at IS NULL", listID, userID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func replaceChannels(tx *gorm.DB, reminder Reminder, channels []string) error {
	if err := tx.Where("reminder_id = ? AND user_id = ?", reminder.ID, reminder.UserID).Delete(&ReminderChannel{}).Error; err != nil {
		return err
	}
	rows := make([]ReminderChannel, 0, len(channels))
	for _, ch := range channels {
		rows = append(rows, ReminderChannel{ReminderID: reminder.ID, UserID: reminder.UserID, Channel: ch, Enabled: true})
	}
	if len(rows) == 0 {
		return nil
	}
	return tx.Create(&rows).Error
}

func rebuildJobs(tx *gorm.DB, reminder Reminder, channels []string) error {
	if err := tx.Model(&DeliveryJob{}).
		Where("reminder_id = ? AND user_id = ? AND status IN ?", reminder.ID, reminder.UserID, []string{"pending", "retry"}).
		Update("status", "cancelled").Error; err != nil {
		return err
	}
	if reminder.DueAt == nil || reminder.CompletedAt != nil {
		return nil
	}
	// Keep all delivery timestamps in UTC. This matters for SQLite because the
	// scheduler's timestamp predicate compares the stored values as text.
	scheduledFor := reminder.DueAt.UTC()
	runAt := scheduledFor
	if reminder.SnoozedUntil != nil {
		runAt = reminder.SnoozedUntil.UTC()
	}
	for _, ch := range channels {
		raw := fmt.Sprintf("%d|%s|%s|%d", reminder.ID, reminder.DueAt.UTC().Format(time.RFC3339Nano), ch, reminder.Version)
		sum := sha256.Sum256([]byte(raw))
		job := DeliveryJob{
			UserID: reminder.UserID, ReminderID: reminder.ID, Channel: ch,
			ScheduledFor: scheduledFor, RunAt: runAt, Status: "pending",
			IdempotencyKey: hex.EncodeToString(sum[:]),
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "reminder_id"}, {Name: "channel"}, {Name: "scheduled_for"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"run_at": runAt, "status": "pending", "attempt_count": 0,
				"next_attempt_at": nil, "locked_at": nil, "worker_id": "",
				"idempotency_key": job.IdempotencyKey, "last_error_code": "",
				"last_error_message": "", "external_message_id": "",
			}),
		}).Create(&job).Error; err != nil {
			return err
		}
	}
	return nil
}

func getReminder(db *gorm.DB, userID, reminderID uint) (ReminderDTO, error) {
	var item Reminder
	if err := db.Where("id = ? AND user_id = ? AND deleted_at IS NULL", reminderID, userID).First(&item).Error; err != nil {
		return ReminderDTO{}, err
	}
	var channels []ReminderChannel
	if err := db.Where("reminder_id = ? AND user_id = ? AND enabled = ?", item.ID, userID, true).Find(&channels).Error; err != nil {
		return ReminderDTO{}, err
	}
	var list List
	_ = db.Where("id = ? AND user_id = ?", item.ListID, userID).First(&list).Error
	dto := ReminderDTO{Reminder: item, ListName: list.Name}
	for _, ch := range channels {
		dto.Channels = append(dto.Channels, ch.Channel)
	}
	return dto, nil
}

func listReminders(db *gorm.DB, userID uint, view, query string, listID uint) ([]ReminderDTO, error) {
	var rows []Reminder
	q := db.Where("user_id = ? AND deleted_at IS NULL", userID)
	now := time.Now()
	loc, _ := time.LoadLocation("Asia/Shanghai")
	localNow := now.In(loc)
	startToday := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, loc)
	endToday := startToday.AddDate(0, 0, 1)
	switch view {
	case "today":
		q = q.Where("completed_at IS NULL AND due_at IS NOT NULL AND due_at < ?", endToday)
	case "planned":
		q = q.Where("completed_at IS NULL AND due_at IS NOT NULL")
	case "completed":
		q = q.Where("completed_at IS NOT NULL")
	default:
		q = q.Where("completed_at IS NULL")
	}
	if listID > 0 {
		q = q.Where("list_id = ?", listID)
	}
	if query = strings.TrimSpace(query); query != "" {
		like := "%" + query + "%"
		q = q.Where("(title LIKE ? OR notes LIKE ?)", like, like)
	}
	if err := q.Order("CASE WHEN due_at IS NULL THEN 1 ELSE 0 END, due_at ASC, priority DESC, position ASC, id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]ReminderDTO, 0, len(rows))
	for _, row := range rows {
		dto, err := getReminder(db, userID, row.ID)
		if err != nil {
			return nil, err
		}
		result = append(result, dto)
	}
	return result, nil
}

func deleteReminder(db *gorm.DB, userID, reminderID uint) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var item Reminder
		if err := tx.Where("id = ? AND user_id = ? AND deleted_at IS NULL", reminderID, userID).First(&item).Error; err != nil {
			return err
		}
		if err := tx.Model(&DeliveryJob{}).Where("reminder_id = ? AND user_id = ? AND status IN ?", reminderID, userID, []string{"pending", "retry"}).Update("status", "cancelled").Error; err != nil {
			return err
		}
		now := time.Now()
		return tx.Model(&item).Update("deleted_at", &now).Error
	})
}

func completeReminder(db *gorm.DB, userID, reminderID uint) (ReminderDTO, error) {
	err := db.Transaction(func(tx *gorm.DB) error {
		var item Reminder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ? AND deleted_at IS NULL", reminderID, userID).First(&item).Error; err != nil {
			return err
		}
		now := time.Now()
		if item.RepeatRule != "" && item.RepeatRule != "none" && item.DueAt != nil {
			history := Completion{ReminderID: item.ID, UserID: userID, ScheduledFor: item.DueAt, CompletedAt: now, TitleSnapshot: item.Title}
			if err := tx.Create(&history).Error; err != nil {
				return err
			}
			next := nextOccurrence(*item.DueAt, item.RepeatRule)
			item.DueAt = &next
			item.CompletedAt = nil
			item.SnoozedUntil = nil
		} else {
			item.CompletedAt = &now
		}
		item.Version++
		if err := tx.Save(&item).Error; err != nil {
			return err
		}
		var channelRows []ReminderChannel
		if err := tx.Where("reminder_id = ? AND user_id = ? AND enabled = ?", item.ID, userID, true).Find(&channelRows).Error; err != nil {
			return err
		}
		channels := make([]string, 0, len(channelRows))
		for _, row := range channelRows {
			channels = append(channels, row.Channel)
		}
		return rebuildJobs(tx, item, channels)
	})
	if err != nil {
		return ReminderDTO{}, err
	}
	return getReminder(db, userID, reminderID)
}

func restoreReminder(db *gorm.DB, userID, reminderID uint) (ReminderDTO, error) {
	err := db.Transaction(func(tx *gorm.DB) error {
		var item Reminder
		if err := tx.Where("id = ? AND user_id = ? AND deleted_at IS NULL", reminderID, userID).First(&item).Error; err != nil {
			return err
		}
		item.CompletedAt = nil
		item.Version++
		if err := tx.Save(&item).Error; err != nil {
			return err
		}
		var channelRows []ReminderChannel
		if err := tx.Where("reminder_id = ? AND user_id = ? AND enabled = ?", item.ID, userID, true).Find(&channelRows).Error; err != nil {
			return err
		}
		channels := make([]string, 0, len(channelRows))
		for _, row := range channelRows {
			channels = append(channels, row.Channel)
		}
		return rebuildJobs(tx, item, channels)
	})
	if err != nil {
		return ReminderDTO{}, err
	}
	return getReminder(db, userID, reminderID)
}

func snoozeReminder(db *gorm.DB, userID, reminderID uint, until time.Time) (ReminderDTO, error) {
	if !until.After(time.Now()) {
		return ReminderDTO{}, errors.New("稍后提醒时间必须晚于当前时间")
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		var item Reminder
		if err := tx.Where("id = ? AND user_id = ? AND completed_at IS NULL AND deleted_at IS NULL", reminderID, userID).First(&item).Error; err != nil {
			return err
		}
		item.SnoozedUntil = &until
		item.Version++
		if err := tx.Save(&item).Error; err != nil {
			return err
		}
		var channelRows []ReminderChannel
		if err := tx.Where("reminder_id = ? AND user_id = ? AND enabled = ?", item.ID, userID, true).Find(&channelRows).Error; err != nil {
			return err
		}
		channels := make([]string, 0, len(channelRows))
		for _, row := range channelRows {
			channels = append(channels, row.Channel)
		}
		return rebuildJobs(tx, item, channels)
	})
	if err != nil {
		return ReminderDTO{}, err
	}
	return getReminder(db, userID, reminderID)
}

func nextOccurrence(t time.Time, rule string) time.Time {
	switch rule {
	case "daily":
		return t.AddDate(0, 0, 1)
	case "weekly":
		return t.AddDate(0, 0, 7)
	case "monthly":
		day := t.Day()
		firstNext := time.Date(t.Year(), t.Month()+1, 1, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
		lastDay := time.Date(firstNext.Year(), firstNext.Month()+1, 0, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location()).Day()
		if day > lastDay {
			day = lastDay
		}
		return time.Date(firstNext.Year(), firstNext.Month(), day, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
	case "yearly":
		year := t.Year() + 1
		day := t.Day()
		lastDay := time.Date(year, t.Month()+1, 0, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location()).Day()
		if day > lastDay {
			day = lastDay
		}
		return time.Date(year, t.Month(), day, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
	default:
		return t
	}
}
