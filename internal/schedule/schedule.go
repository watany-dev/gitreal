package schedule

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

type Schedule interface {
	Next(base time.Time, rng *rand.Rand) time.Time
}

type HourlySchedule struct{}

func (HourlySchedule) Next(base time.Time, rng *rand.Rand) time.Time {
	windowStart := base.Truncate(time.Hour)
	offset := time.Duration(rng.Intn(3600)) * time.Second
	slot := windowStart.Add(offset)
	if !slot.After(base) {
		slot = windowStart.Add(time.Hour + time.Duration(rng.Intn(3600))*time.Second)
	}

	return slot
}

type DailySchedule struct {
	Start time.Duration
	End   time.Duration
}

func (s DailySchedule) Next(base time.Time, rng *rand.Rand) time.Time {
	loc := base.Location()
	midnight := time.Date(base.Year(), base.Month(), base.Day(), 0, 0, 0, 0, loc)
	windowStart := midnight.Add(s.Start)
	windowEnd := midnight.Add(s.End)
	span := windowEnd.Sub(windowStart)
	if span <= 0 {
		return base.Add(24 * time.Hour)
	}

	if base.Before(windowStart) {
		offset := time.Duration(rng.Int63n(int64(span)))
		return windowStart.Add(offset)
	}

	if base.Before(windowEnd) {
		remaining := windowEnd.Sub(base)
		if remaining > time.Second {
			offset := time.Duration(rng.Int63n(int64(remaining)))
			return base.Add(offset).Add(time.Second)
		}
	}

	tomorrowStart := windowStart.Add(24 * time.Hour)
	offset := time.Duration(rng.Int63n(int64(span)))
	return tomorrowStart.Add(offset)
}

type IntervalSchedule struct {
	Interval time.Duration
}

func (s IntervalSchedule) Next(base time.Time, rng *rand.Rand) time.Time {
	if s.Interval <= 0 {
		return base.Add(time.Hour)
	}

	half := int64(s.Interval / 2)
	if half <= 0 {
		return base.Add(s.Interval)
	}

	jitter := time.Duration(rng.Int63n(half))
	return base.Add(s.Interval/2 + jitter)
}

func ParseClock(value string) (time.Duration, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("clock %q: expected HH:MM", value)
	}

	hours, err := strconv.Atoi(parts[0])
	if err != nil || hours < 0 || hours > 23 {
		return 0, fmt.Errorf("clock %q: hour out of range", value)
	}

	minutes, err := strconv.Atoi(parts[1])
	if err != nil || minutes < 0 || minutes > 59 {
		return 0, fmt.Errorf("clock %q: minute out of range", value)
	}

	return time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute, nil
}
