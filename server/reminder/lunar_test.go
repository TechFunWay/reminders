package reminder

import (
	"testing"
	"time"

	"smallgo/server/lunar"
)

func TestValidateReminderInputAcceptsLunarCalendar(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	loc := shanghai()
	due := time.Date(2026, time.September, 25, 9, 0, 0, 0, loc)
	in := SaveReminderInput{
		Title: "农历生日提醒", DueAt: &due, RepeatRule: "yearly", Calendar: "lunar",
		Channels: []string{ChannelInApp},
	}
	if err := validateReminderInput(&in); err != nil {
		t.Fatal(err)
	}
	// 客户端未传锚点时应从 due_at 推导：农历 2026 年八月十五。
	if in.LunarAnchor != "8:15" {
		t.Fatalf("expected derived anchor 8:15, got %q", in.LunarAnchor)
	}
}

func TestValidateReminderInputKeepsExplicitLunarAnchor(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	loc := shanghai()
	due := time.Date(2026, time.September, 25, 9, 0, 0, 0, loc)
	in := SaveReminderInput{
		Title: "农历生日提醒", DueAt: &due, RepeatRule: "yearly", Calendar: "lunar",
		LunarAnchor: "-6:15", Channels: []string{ChannelInApp},
	}
	if err := validateReminderInput(&in); err != nil {
		t.Fatal(err)
	}
	if in.LunarAnchor != "-6:15" {
		t.Fatalf("expected anchor -6:15 preserved, got %q", in.LunarAnchor)
	}
}

func TestValidateReminderInputRejectsBadLunarAnchor(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	loc := shanghai()
	due := time.Date(2026, time.September, 25, 9, 0, 0, 0, loc)
	in := SaveReminderInput{
		Title: "农历生日提醒", DueAt: &due, RepeatRule: "yearly", Calendar: "lunar",
		LunarAnchor: "13:32", Channels: []string{ChannelInApp},
	}
	if err := validateReminderInput(&in); err == nil {
		t.Fatal("expected an error for an out-of-range lunar anchor")
	}
}

func TestLunarCalendarWithoutDueDateIsRejected(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	in := SaveReminderInput{
		Title: "农历生日提醒", RepeatRule: "yearly", Calendar: "lunar", Channels: []string{ChannelInApp},
	}
	if err := validateReminderInput(&in); err == nil {
		t.Fatal("expected an error when yearly_lunar has no due date")
	}
}

func TestCompleteLunarYearlyAdvancesToNextLunarBirthday(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	loc := shanghai()
	// 农历 2026 年八月十五对应公历 2026-09-25；下一次是农历 2027 年八月十五。
	due := time.Date(2026, time.September, 25, 9, 0, 0, 0, loc)
	created, err := createReminder(appDB, 31, SaveReminderInput{
		Title: "农历生日", DueAt: &due, RepeatRule: "yearly", Calendar: "lunar",
		Channels: []string{ChannelInApp},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.LunarAnchor != "8:15" {
		t.Fatalf("expected stored anchor 8:15, got %q", created.LunarAnchor)
	}

	next, err := completeReminder(appDB, 31, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if next.DueAt == nil {
		t.Fatal("expected the next occurrence to be scheduled")
	}
	lunarDate := lunar.FromTime(next.DueAt.In(loc))
	if lunarDate.Year != 2027 || lunarDate.Month != 8 || lunarDate.Day != 15 {
		t.Fatalf("expected next occurrence at lunar 2027-8-15, got %d-%d-%d (%s)",
			lunarDate.Year, lunarDate.Month, lunarDate.Day, next.DueAt.Format("2006-01-02"))
	}
	if next.DueAt.Hour() != 9 || next.DueAt.Minute() != 0 {
		t.Fatalf("expected the original 09:00 time-of-day, got %s", next.DueAt.Format("15:04"))
	}

	// 推进后应重建投递任务，只保留一个 pending 任务。
	var pending int64
	if err := appDB.Model(&DeliveryJob{}).
		Where("reminder_id = ? AND status = ?", created.ID, "pending").
		Count(&pending).Error; err != nil {
		t.Fatal(err)
	}
	if pending != 1 {
		t.Fatalf("expected 1 pending job after completion, got %d", pending)
	}
}

func TestCompleteLunarYearlyDayThirtyFallsBackToLastLunarDay(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	loc := shanghai()
	// 锚点腊月三十：农历 2025 年腊月只有廿九，应回退到除夕 2026-02-16，
	// 下一次是农历 2026 年腊月三十对应的公历日期。
	due := time.Date(2026, time.February, 16, 8, 0, 0, 0, loc)
	created, err := createReminder(appDB, 32, SaveReminderInput{
		Title: "除夕提醒", DueAt: &due, RepeatRule: "yearly", Calendar: "lunar",
		LunarAnchor: "12:30", Channels: []string{ChannelInApp},
	})
	if err != nil {
		t.Fatal(err)
	}
	next, err := completeReminder(appDB, 32, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if next.DueAt == nil {
		t.Fatal("expected the next occurrence to be scheduled")
	}
	lunarDate := lunar.FromTime(next.DueAt.In(loc))
	// 农历 2026 年腊月只有廿九天，"腊月三十"应回退到腊月最后一天（除夕）。
	if lunarDate.Year != 2026 || lunarDate.Month != 12 || lunarDate.Day != 29 {
		t.Fatalf("expected next occurrence at lunar 2026-12-29, got %d-%d-%d (%s)",
			lunarDate.Year, lunarDate.Month, lunarDate.Day, next.DueAt.Format("2006-01-02"))
	}
	if want := "2027-02-05"; next.DueAt.Format("2006-01-02") != want {
		t.Fatalf("expected %s, got %s", want, next.DueAt.Format("2006-01-02"))
	}
}

func TestParseLunarAnchorGuardsMalformedValues(t *testing.T) {
	bad := []string{"", "8", "abc:def", "13:15", "0:15", "8:0", "8:31", "-0:5"}
	for _, anchor := range bad {
		if _, ok := parseLunarAnchor(anchor); ok {
			t.Fatalf("anchor %q should not parse", anchor)
		}
	}
	good := map[string]lunar.Date{
		"8:15":  {Month: 8, Day: 15},
		"-6:15": {Month: -6, Day: 15},
		"12:30": {Month: 12, Day: 30},
	}
	for anchor, want := range good {
		got, ok := parseLunarAnchor(anchor)
		if !ok || got != want {
			t.Fatalf("anchor %q should parse to %v, got %v ok=%v", anchor, want, got, ok)
		}
	}
}
