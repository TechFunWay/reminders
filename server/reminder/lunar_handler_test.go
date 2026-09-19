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

func TestHandleLunarCalendarReturnsLabeledMonth(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/reminder/lunar?year=2026&month=9", nil)
	context.Set("userID", 1)
	handleLunarCalendar(appDB)(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Data struct {
			Year  int `json:"year"`
			Month int `json:"month"`
			Days  []struct {
				Year  int    `json:"year"`
				Month int    `json:"month"`
				Day   int    `json:"day"`
				Lunar string `json:"lunar"`
				Term  string `json:"term"`
			} `json:"days"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.Year != 2026 || body.Data.Month != 9 {
		t.Fatalf("unexpected query echo: %d-%d", body.Data.Year, body.Data.Month)
	}
	if len(body.Data.Days) < 35 || len(body.Data.Days)%7 != 0 {
		t.Fatalf("expected whole weeks of cells, got %d", len(body.Data.Days))
	}

	byDay := map[int]string{}
	terms := map[int]string{}
	for _, day := range body.Data.Days {
		if day.Month == 9 && day.Year == 2026 {
			byDay[day.Day] = day.Lunar
			if day.Term != "" {
				terms[day.Day] = day.Term
			}
		}
	}
	// 2026-09-25 是农历八月十五中秋节；八月初一是 9 月 11 日。
	if byDay[25] != "十五" {
		t.Fatalf("expected mid-autumn day label 十五, got %q", byDay[25])
	}
	if byDay[11] != "八月" {
		t.Fatalf("expected lunar month start 9/11 to show month name 八月, got %q", byDay[11])
	}
	if byDay[1] != "二十" {
		t.Fatalf("expected 9/1 to show 七月二十, got %q", byDay[1])
	}
	// 相邻月的格子也在网格里，用于补齐整周。
	foundAdjacent := false
	for _, day := range body.Data.Days {
		if (day.Month == 8 || day.Month == 10) && day.Year == 2026 {
			foundAdjacent = true
		}
	}
	if !foundAdjacent {
		t.Fatal("expected leading/trailing cells from adjacent months")
	}
}

func TestHandleLunarCalendarRejectsOutOfRangeYear(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/reminder/lunar?year=2200&month=1", nil)
	context.Set("userID", 1)
	handleLunarCalendar(appDB)(context)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}

func TestLunarCalendarMonthMondayFirstGrid(t *testing.T) {
	loc := shanghai()
	cells, err := lunar.CalendarMonth(2026, time.September, loc)
	if err != nil {
		t.Fatal(err)
	}
	// 2026-09-01 是周二，周一开头的网格应先补一格 8 月 31 日。
	if cells[0].Month != 8 || cells[0].Day != 31 || cells[0].InMonth {
		t.Fatalf("expected leading 8/31 cell, got %d-%d-%d inMonth=%v", cells[0].Year, cells[0].Month, cells[0].Day, cells[0].InMonth)
	}
	if cells[1].Month != int(time.September) || cells[1].Day != 1 || !cells[1].InMonth {
		t.Fatalf("expected 2026-09-01 as second cell, got %d-%d-%d", cells[1].Year, cells[1].Month, cells[1].Day)
	}
}
