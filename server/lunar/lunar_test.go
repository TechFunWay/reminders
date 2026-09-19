package lunar

import (
	"testing"
	"time"
)

func fixedCST() *time.Location {
	return time.FixedZone("CST", 8*3600)
}

func TestFromTimeKnownDates(t *testing.T) {
	loc := fixedCST()
	cases := []struct {
		name               string
		year               int
		gregorianMonth     time.Month
		day                int
		wantYear           int
		wantMonth, wantDay int
	}{
		{"中秋节", 2026, time.September, 25, 2026, 8, 15},
		{"春节", 2026, time.February, 17, 2026, 1, 1},
		{"除夕（腊月廿九）", 2026, time.February, 16, 2025, 12, 29},
		{"闰六月十五", 2025, time.August, 8, 2025, -6, 15},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FromTime(time.Date(tc.year, tc.gregorianMonth, tc.day, 12, 0, 0, 0, loc))
			if got.Year != tc.wantYear || got.Month != tc.wantMonth || got.Day != tc.wantDay {
				t.Fatalf("expected lunar %d-%d-%d, got %d-%d-%d",
					tc.wantYear, tc.wantMonth, tc.wantDay, got.Year, got.Month, got.Day)
			}
		})
	}
}

func TestToSolarKnownDates(t *testing.T) {
	loc := fixedCST()
	cases := []struct {
		name             string
		year, month, day int
		want             string
	}{
		{"农历2026八月十五（中秋）", 2026, 8, 15, "2026-09-25"},
		{"农历2026正月初一（春节）", 2026, 1, 1, "2026-02-17"},
		{"农历2025闰六月十五", 2025, -6, 15, "2025-08-08"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ToSolar(tc.year, tc.month, tc.day, loc)
			if err != nil {
				t.Fatal(err)
			}
			if got.Format("2006-01-02") != tc.want {
				t.Fatalf("expected %s, got %s", tc.want, got.Format("2006-01-02"))
			}
		})
	}
}

func TestToSolarDayThirtyFallsBackToLastDay(t *testing.T) {
	loc := fixedCST()
	// 农历2025年腊月只有二十九天，三十应回退到除夕 2026-02-16。
	got, err := ToSolar(2025, 12, 30, loc)
	if err != nil {
		t.Fatal(err)
	}
	if want := "2026-02-16"; got.Format("2006-01-02") != want {
		t.Fatalf("expected %s, got %s", want, got.Format("2006-01-02"))
	}
}

func TestToSolarLeapMonthFallsBackToRegularMonth(t *testing.T) {
	loc := fixedCST()
	// 2026 年没有闰六月，闰六月十五应回退为六月十五。
	got, err := ToSolar(2026, -6, 15, loc)
	if err != nil {
		t.Fatal(err)
	}
	lunarDate := FromTime(got)
	if lunarDate.Year != 2026 || lunarDate.Month != 6 || lunarDate.Day != 15 {
		t.Fatalf("expected lunar 2026-6-15, got %d-%d-%d", lunarDate.Year, lunarDate.Month, lunarDate.Day)
	}
}

func TestNextOccurrenceAdvancesYearly(t *testing.T) {
	loc := fixedCST()
	anchor := Date{Year: 1990, Month: 8, Day: 15}
	from := time.Date(2026, 9, 20, 8, 30, 0, 0, loc)
	got, err := NextOccurrence(anchor, from, loc)
	if err != nil {
		t.Fatal(err)
	}
	// 2026 年八月十五（中秋）是 9 月 25 日。
	want := time.Date(2026, time.September, 25, 8, 30, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	// 再完成一次应推进到 2027 年的八月十五。
	next, err := NextOccurrence(anchor, got, loc)
	if err != nil {
		t.Fatal(err)
	}
	check := FromTime(next)
	if check.Year != 2027 || check.Month != 8 || check.Day != 15 {
		t.Fatalf("expected lunar 2027-8-15, got %d-%d-%d", check.Year, check.Month, check.Day)
	}
}

func TestNextOccurrenceBeforeDateUsesSameYear(t *testing.T) {
	loc := fixedCST()
	anchor := Date{Year: 2000, Month: 8, Day: 15}
	from := time.Date(2026, 1, 1, 9, 0, 0, 0, loc)
	got, err := NextOccurrence(anchor, from, loc)
	if err != nil {
		t.Fatal(err)
	}
	if want := "2026-09-25"; got.Format("2006-01-02") != want {
		t.Fatalf("expected %s, got %s", want, got.Format("2006-01-02"))
	}
}

func TestNextOccurrenceLunarTwelfthMonthAcrossGregorianYear(t *testing.T) {
	loc := fixedCST()
	// 腊月初八锚点：2026 年 1 月已过腊月初八（2026-01-26 属乙巳年腊月初八，未来），验证落在当年。
	anchor := Date{Year: 2000, Month: 12, Day: 8}
	from := time.Date(2026, 9, 1, 8, 0, 0, 0, loc)
	got, err := NextOccurrence(anchor, from, loc)
	if err != nil {
		t.Fatal(err)
	}
	check := FromTime(got)
	if check.Month != 12 || check.Day != 8 {
		t.Fatalf("expected lunar 12-8, got %d-%d", check.Month, check.Day)
	}
}

func TestNextOccurrenceLeapMonthAnchorWithoutLeapYear(t *testing.T) {
	loc := fixedCST()
	anchor := Date{Year: 2025, Month: -6, Day: 15}
	from := time.Date(2026, 9, 20, 8, 0, 0, 0, loc)
	got, err := NextOccurrence(anchor, from, loc)
	if err != nil {
		t.Fatal(err)
	}
	// 2027 年没有闰六月，应回退到六月十五（2027-07-18）。
	if want := "2027-07-18"; got.Format("2006-01-02") != want {
		t.Fatalf("expected %s, got %s", want, got.Format("2006-01-02"))
	}
}
