package reminder

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"smallgo/server/scheduler"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TestBackgroundJobsAvoidAggressiveDatabasePolling(t *testing.T) {
	want := map[string]time.Duration{
		"reminder-delivery":   time.Minute,
		"reminder-qq-gateway": 30 * time.Second,
	}
	for _, job := range scheduler.All() {
		if interval, ok := want[job.Name]; ok {
			if job.Interval != interval {
				t.Fatalf("job %s interval = %s, want %s", job.Name, job.Interval, interval)
			}
			delete(want, job.Name)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing reminder jobs: %#v", want)
	}
}

func TestListListsUsesFixedQueryCount(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	const userID = 71
	lists := []List{
		{UserID: userID, Name: "默认", IsDefault: true},
		{UserID: userID, Name: "工作"},
		{UserID: userID, Name: "生活"},
	}
	if err := appDB.Create(&lists).Error; err != nil {
		t.Fatal(err)
	}
	for _, list := range lists {
		if err := appDB.Create(&Reminder{UserID: userID, ListID: list.ID, Title: list.Name, RepeatRule: "none"}).Error; err != nil {
			t.Fatal(err)
		}
	}

	var queryCount atomic.Int64
	if err := appDB.Callback().Query().Before("gorm:query").Register("test:list-count-query-count", func(*gorm.DB) {
		queryCount.Add(1)
	}); err != nil {
		t.Fatal(err)
	}
	if err := appDB.Callback().Row().Before("gorm:row").Register("test:list-count-row-query-count", func(*gorm.DB) {
		queryCount.Add(1)
	}); err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/reminder/lists", nil)
	context.Set("userID", uint(userID))
	handleListLists(appDB)(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if got := queryCount.Load(); got != 3 {
		t.Fatalf("expected 3 fixed queries, got %d", got)
	}
}

func TestSummaryUsesTwoAggregateQueries(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	const userID = 72
	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)
	completedAt := now
	rows := []Reminder{
		{UserID: userID, ListID: 1, Title: "已过期", DueAt: &past, RepeatRule: "none"},
		{UserID: userID, ListID: 1, Title: "待处理", DueAt: &future, RepeatRule: "none"},
		{UserID: userID, ListID: 1, Title: "已完成", DueAt: &past, CompletedAt: &completedAt, RepeatRule: "none"},
	}
	if err := appDB.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if err := appDB.Create(&Notification{UserID: userID, Type: "reminder_due", Title: "未读"}).Error; err != nil {
		t.Fatal(err)
	}

	var queryCount atomic.Int64
	if err := appDB.Callback().Query().Before("gorm:query").Register("test:summary-query-count", func(*gorm.DB) {
		queryCount.Add(1)
	}); err != nil {
		t.Fatal(err)
	}
	if err := appDB.Callback().Row().Before("gorm:row").Register("test:summary-row-query-count", func(*gorm.DB) {
		queryCount.Add(1)
	}); err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/reminder/summary", nil)
	context.Set("userID", uint(userID))
	handleSummary(appDB)(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if got := queryCount.Load(); got != 2 {
		t.Fatalf("expected 2 aggregate queries, got %d", got)
	}
	var payload struct {
		Data map[string]int64 `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data["all"] != 2 || payload.Data["completed"] != 1 || payload.Data["overdue"] != 1 || payload.Data["unread"] != 1 {
		t.Fatalf("unexpected summary: %#v", payload.Data)
	}
}
