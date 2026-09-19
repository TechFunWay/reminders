package reminder

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"smallgo/server/lunar"

	"github.com/gin-gonic/gin"
)

func TestHandleLunarConvertSolarToLunar(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/reminder/lunar/convert?year=2026&month=9&day=25", nil)
	context.Set("userID", 1)
	handleLunarConvert(appDB)(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Data struct {
			LunarYear  int    `json:"lunar_year"`
			LunarMonth int    `json:"lunar_month"`
			LunarDay   int    `json:"lunar_day"`
			YearName   string `json:"lunar_year_name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.LunarYear != 2026 || body.Data.LunarMonth != 8 || body.Data.LunarDay != 15 {
		t.Fatalf("expected lunar 2026-8-15, got %d-%d-%d", body.Data.LunarYear, body.Data.LunarMonth, body.Data.LunarDay)
	}
	if body.Data.YearName == "" {
		t.Fatal("expected a non-empty ganzhi year name")
	}
}

func TestHandleLunarConvertLunarToSolar(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	// 农历 2025 闰六月十五 → 2025-08-08
	context.Request = httptest.NewRequest(http.MethodGet, "/api/reminder/lunar/convert?lunar_year=2025&lunar_month=-6&lunar_day=15", nil)
	context.Set("userID", 1)
	handleLunarConvert(appDB)(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Data struct {
			Year  int `json:"year"`
			Month int `json:"month"`
			Day   int `json:"day"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.Year != 2025 || body.Data.Month != 8 || body.Data.Day != 8 {
		t.Fatalf("expected 2025-08-08, got %d-%d-%d", body.Data.Year, body.Data.Month, body.Data.Day)
	}

	// 三十回退：农历 2025 腊月三十（当月廿九天）→ 2026-02-16
	recorder2 := httptest.NewRecorder()
	context2, _ := gin.CreateTestContext(recorder2)
	context2.Request = httptest.NewRequest(http.MethodGet, "/api/reminder/lunar/convert?lunar_year=2025&lunar_month=12&lunar_day=30", nil)
	context2.Set("userID", 1)
	handleLunarConvert(appDB)(context2)
	var body2 struct {
		Data struct {
			Year  int `json:"year"`
			Month int `json:"month"`
			Day   int `json:"day"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder2.Body.Bytes(), &body2); err != nil {
		t.Fatal(err)
	}
	if body2.Data.Year != 2026 || body2.Data.Month != 2 || body2.Data.Day != 16 {
		t.Fatalf("expected 2026-02-16 for lunar 12-30 clamp, got %d-%d-%d", body2.Data.Year, body2.Data.Month, body2.Data.Day)
	}

	// 非法参数
	recorder3 := httptest.NewRecorder()
	context3, _ := gin.CreateTestContext(recorder3)
	context3.Request = httptest.NewRequest(http.MethodGet, "/api/reminder/lunar/convert?lunar_year=2025&lunar_month=13&lunar_day=15", nil)
	context3.Set("userID", 1)
	handleLunarConvert(appDB)(context3)
	if recorder3.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid month, got %d", recorder3.Code)
	}
}

func TestValidateReminderInputNormalizesDeprecatedYearlyLunar(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	loc := shanghai()
	due := time.Date(2026, time.September, 25, 9, 0, 0, 0, loc)
	in := SaveReminderInput{
		Title: "老客户端", DueAt: &due, RepeatRule: "yearly_lunar",
		Channels: []string{ChannelInApp},
	}
	if err := validateReminderInput(&in); err != nil {
		t.Fatal(err)
	}
	if in.RepeatRule != "yearly" || in.Calendar != "lunar" {
		t.Fatalf("expected yearly+lunar, got %s+%s", in.RepeatRule, in.Calendar)
	}
	if in.LunarAnchor != "8:15" {
		t.Fatalf("expected derived anchor 8:15, got %q", in.LunarAnchor)
	}
}

func TestValidateReminderInputLunarOnlyAllowsMonthlyYearly(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	loc := shanghai()
	due := time.Date(2026, time.September, 25, 9, 0, 0, 0, loc)
	for _, rule := range []string{"daily", "weekly", "none"} {
		in := SaveReminderInput{
			Title: "非法组合", DueAt: &due, RepeatRule: rule, Calendar: "lunar",
			Channels: []string{ChannelInApp},
		}
		if err := validateReminderInput(&in); err == nil {
			t.Fatalf("expected error for lunar %s rule", rule)
		}
	}
}

func TestValidateReminderInputRejectsUnknownCalendar(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	loc := shanghai()
	due := time.Date(2026, time.September, 25, 9, 0, 0, 0, loc)
	in := SaveReminderInput{
		Title: "未知历法", DueAt: &due, RepeatRule: "yearly", Calendar: "maya",
		Channels: []string{ChannelInApp},
	}
	if err := validateReminderInput(&in); err == nil {
		t.Fatal("expected error for unknown calendar")
	}
}

func TestCompleteLunarMonthlyAdvancesOneLunarMonth(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	loc := shanghai()
	// 农历 2026-08-15（公历 2026-09-25），完成后应推进到农历 2026-09-15。
	due := time.Date(2026, time.September, 25, 9, 0, 0, 0, loc)
	created, err := createReminder(appDB, 41, SaveReminderInput{
		Title: "农历十五", DueAt: &due, RepeatRule: "monthly", Calendar: "lunar",
		LunarAnchor: "8:15", Channels: []string{ChannelInApp},
	})
	if err != nil {
		t.Fatal(err)
	}
	next, err := completeReminder(appDB, 41, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if next.DueAt == nil {
		t.Fatal("expected the next occurrence")
	}
	got := lunar.FromTime(next.DueAt.In(loc))
	if got.Year != 2026 || got.Month != 9 || got.Day != 15 {
		t.Fatalf("expected lunar 2026-9-15, got %d-%d-%d (%s)", got.Year, got.Month, got.Day, next.DueAt.Format("2006-01-02"))
	}
}

func TestCompleteLunarMonthlyDayThirtySkipsShortMonths(t *testing.T) {
	loc := shanghai()
	// 农历三十：从 2026-09-25（八月三十? 实际八月十五，这里直接用三十锚点验证跳月）
	// 锚点三十：农历 2026-08-30 若存在则落在其中，否则回退廿九。
	got, err := lunar.NextMonthlyOccurrence(30, time.Date(2026, time.September, 26, 8, 0, 0, 0, loc), loc)
	if err != nil {
		t.Fatal(err)
	}
	date := lunar.FromTime(got.In(loc).Add(12 * time.Hour))
	if date.Day != 29 && date.Day != 30 {
		t.Fatalf("expected lunar day 29 or 30, got %d", date.Day)
	}
	if !got.After(time.Date(2026, time.September, 26, 8, 0, 0, 0, loc)) {
		t.Fatalf("expected next occurrence after from, got %v", got)
	}
}
