package reminder

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"smallgo/server/lunar"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var repeatRules = map[string]bool{
	"none": true, "daily": true, "weekly": true, "monthly": true, "yearly": true, "yearly_lunar": true,
}

type SaveReminderInput struct {
	Title      string     `json:"title"`
	Notes      string     `json:"notes"`
	ListID     uint       `json:"list_id"`
	Priority   int        `json:"priority"`
	DueAt      *time.Time `json:"due_at"`
	AllDay     bool       `json:"all_day"`
	RepeatRule string     `json:"repeat_rule"`
	// Calendar selects "solar" (default) or "lunar" recurrence for the
	// monthly/yearly rules. The deprecated repeat_rule value "yearly_lunar"
	// is normalized to RepeatRule=yearly + Calendar=lunar on save.
	Calendar string   `json:"calendar"`
	Channels []string `json:"channels"`
	// RepeatNotifyMinutes re-notifies an uncompleted reminder on this interval
	// (in minutes) after each due time; 0 disables it.
	RepeatNotifyMinutes int `json:"repeat_notify_minutes"`
	// ChannelTargets optionally maps a multi-target channel (email) to the
	// ChannelBinding IDs this reminder delivers to. Absent or empty means all.
	ChannelTargets map[string][]uint `json:"channel_targets"`
	// LunarAnchor optionally pins the lunar month/day a lunar reminder
	// recurs on, encoded "M:D" (negative M = leap month). When empty, the
	// anchor is derived from DueAt on save.
	LunarAnchor string `json:"lunar_anchor"`
	Version     uint   `json:"version"`
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
	// "yearly_lunar" is the deprecated pre-v0.3.2 spelling of the lunar
	// yearly rule; normalize it so old clients keep working.
	if in.RepeatRule == "yearly_lunar" {
		in.RepeatRule = "yearly"
		in.Calendar = "lunar"
	}
	switch in.Calendar {
	case "", "solar":
		in.Calendar = "solar"
		in.LunarAnchor = ""
	case "lunar":
		if in.RepeatRule != "monthly" && in.RepeatRule != "yearly" {
			return errors.New("农历循环仅支持每月或每年")
		}
		if in.DueAt == nil {
			return errors.New("农历循环需要先选择提醒日期")
		}
		anchor, err := normalizeLunarAnchor(in.LunarAnchor, *in.DueAt)
		if err != nil {
			return err
		}
		in.LunarAnchor = anchor
	default:
		return errors.New("历法无效，仅支持公历或农历")
	}
	if in.RepeatNotifyMinutes < 0 || in.RepeatNotifyMinutes > 60*24*31 {
		return errors.New("过期提醒频率无效，最长 31 天")
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
	// Per-reminder mailbox selection only applies when the email channel is
	// selected; every other key is ignored.
	if in.ChannelTargets != nil {
		cleaned := map[string][]uint{}
		for _, ch := range in.Channels {
			if ch != ChannelEmail {
				continue
			}
			if ids := dedupeUint(in.ChannelTargets[ch]); len(ids) > 0 {
				cleaned[ch] = ids
			}
		}
		in.ChannelTargets = cleaned
	}
	return nil
}

func dedupeUint(values []uint) []uint {
	seen := map[uint]bool{}
	clean := make([]uint, 0, len(values))
	for _, value := range values {
		if value > 0 && !seen[value] {
			seen[value] = true
			clean = append(clean, value)
		}
	}
	sort.Slice(clean, func(i, j int) bool { return clean[i] < clean[j] })
	return clean
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
			RepeatRule: in.RepeatRule, Calendar: in.Calendar, LunarAnchor: in.LunarAnchor,
			RepeatNotifyMinutes: in.RepeatNotifyMinutes, Version: 1,
		}
		if err := tx.Create(&created).Error; err != nil {
			return err
		}
		if err := replaceChannels(tx, created, in); err != nil {
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
		existing.Calendar = in.Calendar
		existing.LunarAnchor = in.LunarAnchor
		existing.RepeatNotifyMinutes = in.RepeatNotifyMinutes
		existing.SnoozedUntil = nil
		existing.Version++
		if err := tx.Save(&existing).Error; err != nil {
			return err
		}
		if err := replaceChannels(tx, existing, in); err != nil {
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

func replaceChannels(tx *gorm.DB, reminder Reminder, in SaveReminderInput) error {
	if err := tx.Where("reminder_id = ? AND user_id = ?", reminder.ID, reminder.UserID).Delete(&ReminderChannel{}).Error; err != nil {
		return err
	}
	rows := make([]ReminderChannel, 0, len(in.Channels))
	for _, ch := range in.Channels {
		row := ReminderChannel{ReminderID: reminder.ID, UserID: reminder.UserID, Channel: ch, Enabled: true}
		if ch == ChannelEmail && len(in.ChannelTargets[ch]) > 0 {
			ids, err := ownedEmailBindingIDs(tx, reminder.UserID, in.ChannelTargets[ch])
			if err != nil {
				return err
			}
			if len(ids) > 0 {
				raw, err := json.Marshal(ids)
				if err != nil {
					return err
				}
				row.Targets = string(raw)
			}
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil
	}
	return tx.Create(&rows).Error
}

// ownedEmailBindingIDs keeps only the IDs that belong to the user's own email
// bindings, so a crafted request cannot reference another account's targets.
func ownedEmailBindingIDs(tx *gorm.DB, userID uint, wanted []uint) ([]uint, error) {
	var ids []uint
	if err := tx.Model(&ChannelBinding{}).
		Where("user_id = ? AND channel = ? AND id IN ?", userID, ChannelEmail, wanted).
		Order("id ASC").Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
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
		if ch.Channel == ChannelEmail {
			if ids := parseChannelTargets(ch.Targets); len(ids) > 0 {
				dto.ChannelTargets = map[string][]uint{ChannelEmail: ids}
			}
		}
	}
	return dto, nil
}

func parseChannelTargets(raw string) []uint {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var ids []uint
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		return nil
	}
	return ids
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
	if len(rows) == 0 {
		return []ReminderDTO{}, nil
	}

	reminderIDs := make([]uint, 0, len(rows))
	listIDs := make([]uint, 0, len(rows))
	seenLists := make(map[uint]struct{})
	for _, row := range rows {
		reminderIDs = append(reminderIDs, row.ID)
		if _, ok := seenLists[row.ListID]; !ok {
			seenLists[row.ListID] = struct{}{}
			listIDs = append(listIDs, row.ListID)
		}
	}

	var channelRows []ReminderChannel
	if err := db.Where("user_id = ? AND reminder_id IN ? AND enabled = ?", userID, reminderIDs, true).
		Order("reminder_id ASC, id ASC").Find(&channelRows).Error; err != nil {
		return nil, err
	}
	channelsByReminder := make(map[uint][]string, len(rows))
	targetsByReminder := map[uint]map[string][]uint{}
	for _, channel := range channelRows {
		channelsByReminder[channel.ReminderID] = append(channelsByReminder[channel.ReminderID], channel.Channel)
		if channel.Channel == ChannelEmail {
			if ids := parseChannelTargets(channel.Targets); len(ids) > 0 {
				targetsByReminder[channel.ReminderID] = map[string][]uint{ChannelEmail: ids}
			}
		}
	}

	var lists []List
	if err := db.Where("user_id = ? AND id IN ?", userID, listIDs).Find(&lists).Error; err != nil {
		return nil, err
	}
	listNames := make(map[uint]string, len(lists))
	for _, list := range lists {
		listNames[list.ID] = list.Name
	}

	result := make([]ReminderDTO, 0, len(rows))
	for _, row := range rows {
		result = append(result, ReminderDTO{
			Reminder:       row,
			Channels:       channelsByReminder[row.ID],
			ChannelTargets: targetsByReminder[row.ID],
			ListName:       listNames[row.ListID],
		})
	}
	return result, nil
}

// retireDueNotifications 把一条提醒仍未读的到点通知标为已读，供完成、稍后提醒、
// 删除三种处理复用（与提醒的改动放在同一事务里）。这条「到点了」的提醒既然已被
// 处理，通知中心不该再挂着未读角标，其它设备上依赖已读状态的到点补弹（前端
// MainLayout 的 recoverMissedAlerts）也不能再把它弹出来。
func retireDueNotifications(tx *gorm.DB, userID, reminderID uint, at time.Time) error {
	return tx.Model(&Notification{}).
		Where("user_id = ? AND reminder_id = ? AND type = ? AND read_at IS NULL", userID, reminderID, "reminder_due").
		Update("read_at", &at).Error
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
		if err := retireDueNotifications(tx, userID, reminderID, now); err != nil {
			return err
		}
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
			next, err := nextOccurrenceWithCalendar(*item.DueAt, item.RepeatRule, item.Calendar, item.LunarAnchor)
			if err != nil {
				return err
			}
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
		if err := retireDueNotifications(tx, userID, reminderID, now); err != nil {
			return err
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
		if err := retireDueNotifications(tx, userID, reminderID, time.Now()); err != nil {
			return err
		}
		return rebuildJobs(tx, item, channels)
	})
	if err != nil {
		return ReminderDTO{}, err
	}
	return getReminder(db, userID, reminderID)
}

// nextOccurrenceWithCalendar dispatches to the lunar calendar when the
// reminder follows it, and to the plain Gregorian rules otherwise. Lunar
// recurrence never fails hard here: if the anchor is missing or unreadable
// the reminder simply keeps its current due date, which leaves the delivery
// jobs untouched.
func nextOccurrenceWithCalendar(t time.Time, rule, calendar, anchor string) (time.Time, error) {
	if calendar == "lunar" {
		parsed, ok := parseLunarAnchor(anchor)
		if !ok {
			return t, nil
		}
		loc, _ := time.LoadLocation("Asia/Shanghai")
		switch rule {
		case "monthly":
			return lunar.NextMonthlyOccurrence(parsed.Day, t, loc)
		case "yearly":
			return lunar.NextOccurrence(parsed, t, loc)
		}
	}
	return nextOccurrence(t, rule), nil
}

// parseLunarAnchor decodes the stored "M:D" lunar anchor. A missing colon,
// out-of-range month, or non-positive day means the anchor was never stored
// or is corrupt; the caller falls back to keeping the current due date.
func parseLunarAnchor(anchor string) (lunar.Date, bool) {
	monthStr, dayStr, found := strings.Cut(anchor, ":")
	if !found {
		return lunar.Date{}, false
	}
	month, errMonth := strconv.Atoi(monthStr)
	day, errDay := strconv.Atoi(dayStr)
	if errMonth != nil || errDay != nil {
		return lunar.Date{}, false
	}
	absMonth := month
	if absMonth < 0 {
		absMonth = -absMonth
	}
	if absMonth < 1 || absMonth > 12 || day < 1 || day > 30 {
		return lunar.Date{}, false
	}
	return lunar.Date{Year: 0, Month: month, Day: day}, true
}

// normalizeLunarAnchor validates a client-provided lunar anchor or derives
// one from the reminder's due date when the client did not supply one.
func normalizeLunarAnchor(anchor string, due time.Time) (string, error) {
	anchor = strings.TrimSpace(anchor)
	if anchor == "" {
		loc, _ := time.LoadLocation("Asia/Shanghai")
		converted := lunar.FromTime(due.In(loc))
		return fmt.Sprintf("%d:%d", converted.Month, converted.Day), nil
	}
	parsed, ok := parseLunarAnchor(anchor)
	if !ok {
		return "", errors.New("农历锚点格式无效")
	}
	return fmt.Sprintf("%d:%d", parsed.Month, parsed.Day), nil
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
