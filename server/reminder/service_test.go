package reminder

import (
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"smallgo/server/database"

	"gorm.io/gorm"
)

func testDB(t *testing.T) func() {
	t.Helper()
	db, err := database.InitDB(filepath.Join(t.TempDir(), "reminder-test.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := database.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	appDB = db
	return func() { _ = database.CloseDB(db) }
}

func TestCreateAndUpdateReminderRebuildsJobIdempotently(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	due := time.Now().Add(2 * time.Hour).Truncate(time.Second)
	created, err := createReminder(appDB, 42, SaveReminderInput{
		Title: "测试提醒", DueAt: &due, RepeatRule: "none",
		Channels: []string{ChannelInApp, ChannelEmail},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(created.Channels) != 2 {
		t.Fatalf("expected 2 channels, got %v", created.Channels)
	}

	created.Notes = "更新备注但不改变时间"
	updated, err := updateReminder(appDB, 42, created.ID, SaveReminderInput{
		Title: created.Title, Notes: created.Notes, ListID: created.ListID,
		DueAt: &due, RepeatRule: "none", Channels: created.Channels, Version: created.Version,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Version != created.Version+1 {
		t.Fatalf("expected version %d, got %d", created.Version+1, updated.Version)
	}

	var activeJobs int64
	if err := appDB.Model(&DeliveryJob{}).
		Where("reminder_id = ? AND status = ?", created.ID, "pending").
		Count(&activeJobs).Error; err != nil {
		t.Fatal(err)
	}
	if activeJobs != 2 {
		t.Fatalf("expected 2 active jobs, got %d", activeJobs)
	}
}

func TestCompleteRecurringReminderAdvancesFromScheduledTime(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	loc := shanghai()
	due := time.Date(2026, time.January, 31, 9, 0, 0, 0, loc)
	created, err := createReminder(appDB, 7, SaveReminderInput{
		Title: "月末提醒", DueAt: &due, RepeatRule: "monthly", Channels: []string{ChannelInApp},
	})
	if err != nil {
		t.Fatal(err)
	}
	next, err := completeReminder(appDB, 7, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, time.February, 28, 9, 0, 0, 0, loc)
	if next.DueAt == nil || !next.DueAt.Equal(want) {
		t.Fatalf("expected %v, got %v", want, next.DueAt)
	}
	if next.CompletedAt != nil {
		t.Fatal("recurring reminder should remain active")
	}

	var histories int64
	if err := appDB.Model(&Completion{}).Where("reminder_id = ?", created.ID).Count(&histories).Error; err != nil {
		t.Fatal(err)
	}
	if histories != 1 {
		t.Fatalf("expected completion history, got %d", histories)
	}
}

func TestYearlyLeapDayUsesEndOfFebruary(t *testing.T) {
	loc := shanghai()
	start := time.Date(2024, time.February, 29, 8, 30, 0, 0, loc)
	got := nextOccurrence(start, "yearly")
	want := time.Date(2025, time.February, 28, 8, 30, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestInAppDeliveryCreatesNotificationOnce(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	events, unsubscribe := reminderRealtime.subscribe(9)
	defer unsubscribe()

	due := time.Now().Add(-time.Minute).Truncate(time.Second)
	created, err := createReminder(appDB, 9, SaveReminderInput{
		Title: "已经到期", DueAt: &due, RepeatRule: "none", Channels: []string{ChannelInApp},
	})
	if err != nil {
		t.Fatal(err)
	}

	dispatchDueJobs(appDB)
	dispatchDueJobs(appDB)

	var notifications int64
	if err := appDB.Model(&Notification{}).Where("user_id = ? AND reminder_id = ?", 9, created.ID).Count(&notifications).Error; err != nil {
		t.Fatal(err)
	}
	if notifications != 1 {
		t.Fatalf("expected one notification, got %d", notifications)
	}
	var job DeliveryJob
	if err := appDB.Where("reminder_id = ? AND channel = ?", created.ID, ChannelInApp).First(&job).Error; err != nil {
		t.Fatal(err)
	}
	if job.Status != "succeeded" || job.AttemptCount != 1 {
		t.Fatalf("unexpected job state: status=%s attempts=%d", job.Status, job.AttemptCount)
	}
	select {
	case event := <-events:
		if event.Type != "notification.created" || event.Notification == nil || event.Notification.ReminderID == nil || *event.Notification.ReminderID != created.ID {
			t.Fatalf("unexpected realtime event: %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("expected a realtime notification event")
	}
}

func TestReminderOwnershipIsolation(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	created, err := createReminder(appDB, 100, SaveReminderInput{
		Title: "用户 A 的提醒", RepeatRule: "none", Channels: []string{ChannelInApp},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := getReminder(appDB, 200, created.ID); err == nil {
		t.Fatal("another user must not read the reminder")
	}
	if err := deleteReminder(appDB, 200, created.ID); err == nil {
		t.Fatal("another user must not delete the reminder")
	}
}

func TestListRemindersLoadsRelatedDataInConstantQueries(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	const userID = 55
	for i := 0; i < 30; i++ {
		_, err := createReminder(appDB, userID, SaveReminderInput{
			Title: "批量提醒", RepeatRule: "none", Channels: []string{ChannelInApp, ChannelEmail},
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	var queryCount atomic.Int64
	if err := appDB.Callback().Query().Before("gorm:query").Register("test:list-query-count", func(*gorm.DB) {
		queryCount.Add(1)
	}); err != nil {
		t.Fatal(err)
	}

	items, err := listReminders(appDB, userID, "all", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 30 {
		t.Fatalf("expected 30 reminders, got %d", len(items))
	}
	if got := queryCount.Load(); got != 3 {
		t.Fatalf("expected 3 fixed queries, got %d", got)
	}
	for _, item := range items {
		if item.ListName == "" || len(item.Channels) != 2 {
			t.Fatalf("missing related data: list=%q channels=%v", item.ListName, item.Channels)
		}
	}
}
