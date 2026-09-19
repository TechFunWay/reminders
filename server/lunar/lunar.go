// Package lunar wraps the Chinese lunar calendar conversions used by the
// yearly-lunar reminder recurrence. It converts between Gregorian dates and
// (lunar year, lunar month, lunar day) triples, where a negative lunar month
// denotes the leap month (e.g. -6 is the leap sixth month).
package lunar

import (
	"sync"
	"time"

	"github.com/6tail/lunar-go/calendar"
)

// libMu serializes all calls into lunar-go. The library keeps a single-entry
// global year cache whose read path is not synchronized, so concurrent HTTP
// requests could race without this lock. Conversions are sub-millisecond.
var libMu sync.Mutex

// Date is a lunar calendar date. Month is 1-12, negative for the leap month
// of that number; Day is 1-30.
type Date struct {
	Year  int
	Month int
	Day   int
}

// FromTime converts a Gregorian time to its lunar date using the calendar day
// of t in its own location. The time-of-day part is ignored.
func FromTime(t time.Time) Date {
	libMu.Lock()
	defer libMu.Unlock()
	l := calendar.NewLunarFromDate(t)
	return Date{Year: l.GetYear(), Month: l.GetMonth(), Day: l.GetDay()}
}

// FromYmd builds a lunar date directly.
func FromYmd(year, month, day int) Date {
	return Date{Year: year, Month: month, Day: day}
}

// ToSolar converts a lunar date to the Gregorian day it falls on, in loc.
// A negative month denotes the leap month; when the year has no such leap
// month the regular month is used instead, which is the customary way lunar
// birthdays are observed.
func ToSolar(year, month, day int, loc *time.Location) (time.Time, error) {
	libMu.Lock()
	defer libMu.Unlock()
	l, err := toLunar(year, month, day, loc)
	if err != nil && month < 0 {
		l, err = toLunar(year, -month, day, loc)
	}
	if err == nil {
		return solarDay(l, loc), nil
	}
	// The requested lunar day does not exist in that year (a day 30 in a
	// 29-day month). Fall back to the last day of the month so a "30th day"
	// birthday still fires, mirroring the Gregorian month-end clamp in the
	// yearly rule.
	last, lastErr := lastDayOfLunarMonth(year, month, loc)
	if lastErr != nil {
		return time.Time{}, lastErr
	}
	return last, nil
}

// NextOccurrence returns the next Gregorian occurrence of the anchor's lunar
// month/day strictly after from, keeping from's time-of-day. The anchor's
// year is only a fallback for validation; the search always starts from the
// lunar year that contains from, because a late lunar month (e.g. the twelfth)
// recurs before the anchor's Gregorian anniversary would.
func NextOccurrence(anchor Date, from time.Time, loc *time.Location) (time.Time, error) {
	startLunarYear := FromTime(from.In(loc)).Year
	for lunarYear := startLunarYear; lunarYear <= startLunarYear+200; lunarYear++ {
		gregorian, err := ToSolar(lunarYear, anchor.Month, anchor.Day, loc)
		if err != nil {
			return time.Time{}, err
		}
		withTime := time.Date(gregorian.Year(), gregorian.Month(), gregorian.Day(),
			from.Hour(), from.Minute(), from.Second(), from.Nanosecond(), from.Location())
		if withTime.After(from.In(loc)) {
			return withTime, nil
		}
	}
	return time.Time{}, errNoOccurrence
}

// NextMonthlyOccurrence returns the next Gregorian day whose lunar day equals
// lunarDay, strictly after from, keeping from's time-of-day. Day 30 falls
// back to the month's last day when a lunar month only has 29. The search
// walks forward one lunar month at a time from the month containing from.
func NextMonthlyOccurrence(lunarDay int, from time.Time, loc *time.Location) (time.Time, error) {
	start := FromTime(from.In(loc))
	// Negative months denote leap months; walking ±1 through the signed
	// month number skips in and out of leap months naturally.
	for i := 0; i < 400; i++ {
		gregorian, err := ToSolar(start.Year, start.Month, lunarDay, loc)
		if err == nil {
			withTime := time.Date(gregorian.Year(), gregorian.Month(), gregorian.Day(),
				from.Hour(), from.Minute(), from.Second(), from.Nanosecond(), from.Location())
			if withTime.After(from.In(loc)) {
				return withTime, nil
			}
		}
		year, next := nextLunarMonth(start.Year, start.Month)
		start = Date{Year: year, Month: next, Day: 1}
	}
	return time.Time{}, errNoOccurrence
}

// nextLunarMonth returns the lunar year/month following the given signed
// month, crossing the lunar year boundary (month 12 → next year 1).
func nextLunarMonth(year, month int) (int, int) {
	abs := month
	if abs < 0 {
		abs = -abs
	}
	if abs >= 12 {
		return year + 1, 1
	}
	return year, abs + 1
}

type occurrenceError string

func (e occurrenceError) Error() string { return string(e) }

const errNoOccurrence = occurrenceError("no lunar occurrence found")

// toLunar validates and converts a lunar Ymd to the library representation,
// guarding the panics lunar-go raises for impossible dates.
func toLunar(year, month, day int, _ *time.Location) (l *calendar.Lunar, err error) {
	defer func() {
		if recover() != nil {
			l, err = nil, occurrenceError("lunar date not in range")
		}
	}()
	l = calendar.NewLunarFromYmd(year, month, day)
	if l.GetYear() != year || l.GetMonth() != month || l.GetDay() != day {
		return nil, occurrenceError("lunar date not in range")
	}
	return l, nil
}

// lastDayOfLunarMonth returns the Gregorian date of the last day (29 or 30)
// of the given lunar month. Leap months fall back to the regular month.
func lastDayOfLunarMonth(year, month int, loc *time.Location) (time.Time, error) {
	lm, err := lunarMonth(year, month)
	if err != nil && month < 0 {
		lm, err = lunarMonth(year, -month)
	}
	if err != nil {
		return time.Time{}, err
	}
	l, err := toLunar(year, month, lm.GetDayCount(), loc)
	if err != nil {
		return time.Time{}, err
	}
	return solarDay(l, loc), nil
}

// solarDay converts a library Lunar to a Gregorian midnight in loc.
func solarDay(l *calendar.Lunar, loc *time.Location) time.Time {
	s := l.GetSolar()
	return time.Date(s.GetYear(), time.Month(s.GetMonth()), s.GetDay(), 0, 0, 0, 0, loc)
}

// lunarMonth exposes the day count of a lunar month, guarding panics for
// years/months the library does not know.
func lunarMonth(year, month int) (lm *calendar.LunarMonth, err error) {
	defer func() {
		if recover() != nil {
			lm, err = nil, occurrenceError("lunar month not found")
		}
	}()
	lm = calendar.NewLunarMonthFromYm(year, month)
	if lm == nil {
		return nil, occurrenceError("lunar month not found")
	}
	return lm, nil
}

// DayInfo describes one Gregorian day with its lunar labels, used by the
// calendar picker.
type DayInfo struct {
	// Gregorian year/month/day the cell represents.
	Year  int `json:"year"`
	Month int `json:"month"`
	Day   int `json:"day"`
	// Lunar is the in-month lunar date label: 初一 shows the month name
	// (e.g. 八月), other days show 初二/十五/三十 style day names.
	Lunar string `json:"lunar"`
	// LunarMonth/LunarDay are the raw lunar month (negative = leap) and day.
	LunarMonth int `json:"lunar_month"`
	LunarDay   int `json:"lunar_day"`
	// LunarYearName is the ganzhi + zodiac year label, e.g. 丙午马年.
	LunarYearName string `json:"lunar_year_name,omitempty"`
	// Term is the solar term name when the day is one (清明, 冬至…).
	Term string `json:"term,omitempty"`
	// Festival is the lunar or solar festival name (春节, 中秋, 国庆…).
	Festival string `json:"festival,omitempty"`
	// InMonth marks cells belonging to the queried Gregorian month.
	InMonth bool `json:"in_month"`
}

// monthName returns the Chinese name (正月…腊月) of a lunar month; negative
// months are prefixed with 闰.
func monthName(month int) string {
	names := [13]string{"", "正月", "二月", "三月", "四月", "五月", "六月",
		"七月", "八月", "九月", "十月", "冬月", "腊月"}
	m := month
	prefix := ""
	if m < 0 {
		m, prefix = -m, "闰"
	}
	if m < 1 || m > 12 {
		return prefix
	}
	return prefix + names[m]
}

var dayNames = [31]string{"", "初一", "初二", "初三", "初四", "初五", "初六", "初七", "初八", "初九", "初十",
	"十一", "十二", "十三", "十四", "十五", "十六", "十七", "十八", "十九", "二十",
	"廿一", "廿二", "廿三", "廿四", "廿五", "廿六", "廿七", "廿八", "廿九", "三十"}

func dayName(day int) string {
	if day < 1 || day > 30 {
		return ""
	}
	return dayNames[day]
}

// YearName returns the ganzhi + zodiac label of a lunar year, e.g. 丙午马年.
func YearName(lunarYear int) string {
	libMu.Lock()
	defer libMu.Unlock()
	l := calendar.NewLunarFromYmd(lunarYear, 1, 1)
	return l.GetYearInGanZhi() + l.GetYearShengXiao() + "年"
}

// CalendarMonth renders a Gregorian month as calendar picker cells. Weeks
// start on Monday; leading and trailing cells from adjacent months are
// included so the grid is always a whole number of weeks.
func CalendarMonth(year int, month time.Month, loc *time.Location) ([]DayInfo, error) {
	if month < time.January || month > time.December {
		return nil, occurrenceError("month out of range")
	}
	first := time.Date(year, month, 1, 0, 0, 0, 0, loc)
	// weekday of the 1st, Monday-based (0 = Monday).
	lead := (int(first.Weekday()) + 6) % 7
	gridStart := first.AddDate(0, 0, -lead)
	daysInMonth := time.Date(year, month+1, 0, 0, 0, 0, 0, loc).Day()
	cells := (daysInMonth + lead + 6) / 7 * 7

	libMu.Lock()
	defer libMu.Unlock()

	out := make([]DayInfo, 0, cells)
	for i := 0; i < cells; i++ {
		day := gridStart.AddDate(0, 0, i)
		info := dayInfo(day, loc, month)
		out = append(out, info)
	}
	return out, nil
}

// dayInfo labels one Gregorian day. Callers must hold libMu.
func dayInfo(day time.Time, loc *time.Location, queryMonth time.Month) DayInfo {
	// Clone through the library with noon to avoid DST edge cases.
	solar := calendar.NewSolarFromDate(day.Add(12 * time.Hour))
	lunarDay := calendar.NewLunarFromSolar(solar)

	info := DayInfo{
		Year:       day.Year(),
		Month:      int(day.Month()),
		Day:        day.Day(),
		LunarMonth: lunarDay.GetMonth(),
		LunarDay:   lunarDay.GetDay(),
		InMonth:    day.Month() == queryMonth,
	}
	// 初一 shows the month name instead of the day, the classic calendar style.
	if lunarDay.GetDay() == 1 {
		info.Lunar = monthName(lunarDay.GetMonth())
	} else {
		info.Lunar = dayName(lunarDay.GetDay())
	}
	info.LunarYearName = lunarDay.GetYearInGanZhi() + lunarDay.GetYearShengXiao() + "年"
	info.Term = lunarDay.GetJieQi()
	if f := lunarDay.GetFestivals(); f.Len() > 0 {
		info.Festival = f.Front().Value.(string)
	}
	if info.Festival == "" {
		if f := solar.GetFestivals(); f.Len() > 0 {
			info.Festival = f.Front().Value.(string)
		}
	}
	return info
}
