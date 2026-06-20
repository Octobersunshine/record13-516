package timeutil

import (
	"fmt"
	"time"
)

var (
	CSTLocation *time.Location
)

func init() {
	var err error
	CSTLocation, err = time.LoadLocation("Asia/Shanghai")
	if err != nil {
		CSTLocation = time.FixedZone("CST", 8*3600)
	}
}

func Now() time.Time {
	return time.Now().In(CSTLocation)
}

func ParseTime(layout, value string) (time.Time, error) {
	return time.ParseInLocation(layout, value, CSTLocation)
}

func ParseTimeMultiLayout(value string, layouts []string) (time.Time, error) {
	var lastErr error
	for _, layout := range layouts {
		t, err := time.ParseInLocation(layout, value, CSTLocation)
		if err == nil {
			return t, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return time.Time{}, lastErr
	}
	return time.Time{}, fmt.Errorf("parse time failed: %s", value)
}

func ParseDate(dateStr string) (time.Time, error) {
	layouts := []string{
		"2006-01-02",
		"2006/01/02",
		"20060102",
	}
	return ParseTimeMultiLayout(dateStr, layouts)
}

func StartOfDay(t time.Time) time.Time {
	t = t.In(CSTLocation)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, CSTLocation)
}

func EndOfDay(t time.Time) time.Time {
	t = t.In(CSTLocation)
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 999999999, CSTLocation)
}

func DateRange(dateStr string) (start, end time.Time, err error) {
	date, err := ParseDate(dateStr)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	start = StartOfDay(date)
	end = EndOfDay(date)
	return start, end, nil
}

type DateRangeOption struct {
	GracePeriodMinutes int
}

func DateRangeWithGrace(dateStr string, opt DateRangeOption) (start, end time.Time, err error) {
	start, end, err = DateRange(dateStr)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	grace := time.Duration(opt.GracePeriodMinutes) * time.Minute
	start = start.Add(-grace)
	end = end.Add(grace)
	return start, end, nil
}

func IsInRange(t, start, end time.Time) bool {
	if t.IsZero() {
		return false
	}
	t = t.In(CSTLocation)
	return !t.Before(start) && !t.After(end)
}

func IsSameDate(t1, t2 time.Time) bool {
	t1 = t1.In(CSTLocation)
	t2 = t2.In(CSTLocation)
	y1, m1, d1 := t1.Date()
	y2, m2, d2 := t2.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

func FormatDate(t time.Time) string {
	return t.In(CSTLocation).Format("2006-01-02")
}

func FormatDateTime(t time.Time) string {
	return t.In(CSTLocation).Format("2006-01-02 15:04:05")
}
