package schedule

import (
	"math/rand"
	"testing"
	"time"
)

func TestHourlySchedule(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(1))
	base := time.Date(2026, 5, 1, 12, 15, 0, 0, time.UTC)
	slot := HourlySchedule{}.Next(base, rng)

	if !slot.After(base) {
		t.Fatalf("Hourly.Next() = %s, want after %s", slot, base)
	}
	if slot.After(base.Add(2 * time.Hour)) {
		t.Fatalf("Hourly.Next() = %s, want within two hours", slot)
	}

	base = time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	slot = HourlySchedule{}.Next(base, rand.New(rand.NewSource(2)))
	if !slot.After(base) {
		t.Fatalf("Hourly.Next() at hour boundary = %s, want after %s", slot, base)
	}
}

func TestDailySchedule(t *testing.T) {
	t.Parallel()

	sched := DailySchedule{Start: 9 * time.Hour, End: 22 * time.Hour}
	rng := rand.New(rand.NewSource(1))

	t.Run("base before window picks today", func(t *testing.T) {
		base := time.Date(2026, 5, 1, 6, 0, 0, 0, time.UTC)
		got := sched.Next(base, rng)
		if got.Day() != base.Day() {
			t.Fatalf("Next() day = %d, want %d (today)", got.Day(), base.Day())
		}
		if got.Hour() < 9 || got.Hour() >= 22 {
			t.Fatalf("Next() hour = %d, want in [9, 22)", got.Hour())
		}
	})

	t.Run("base inside window picks rest of today", func(t *testing.T) {
		base := time.Date(2026, 5, 1, 14, 0, 0, 0, time.UTC)
		got := sched.Next(base, rng)
		if !got.After(base) {
			t.Fatalf("Next() = %s, want after base %s", got, base)
		}
		if got.Day() != base.Day() {
			t.Fatalf("Next() day = %d, want %d (today)", got.Day(), base.Day())
		}
	})

	t.Run("base after window picks tomorrow", func(t *testing.T) {
		base := time.Date(2026, 5, 1, 23, 30, 0, 0, time.UTC)
		got := sched.Next(base, rng)
		if got.Day() != base.Day()+1 {
			t.Fatalf("Next() day = %d, want %d (tomorrow)", got.Day(), base.Day()+1)
		}
		if got.Hour() < 9 || got.Hour() >= 22 {
			t.Fatalf("Next() hour = %d, want in [9, 22)", got.Hour())
		}
	})

	t.Run("zero span falls back to next day", func(t *testing.T) {
		bad := DailySchedule{Start: 12 * time.Hour, End: 12 * time.Hour}
		base := time.Date(2026, 5, 1, 6, 0, 0, 0, time.UTC)
		got := bad.Next(base, rng)
		if !got.After(base) {
			t.Fatalf("Next() = %s, want after base %s", got, base)
		}
	})
}

func TestIntervalSchedule(t *testing.T) {
	t.Parallel()

	sched := IntervalSchedule{Interval: 30 * time.Minute}
	rng := rand.New(rand.NewSource(7))
	base := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

	for i := 0; i < 50; i++ {
		got := sched.Next(base, rng)
		delta := got.Sub(base)
		if delta < 15*time.Minute || delta > 30*time.Minute {
			t.Fatalf("iteration %d: delta = %s, want in [15min, 30min]", i, delta)
		}
	}

	zero := IntervalSchedule{Interval: 0}
	got := zero.Next(base, rng)
	if got.Sub(base) != time.Hour {
		t.Fatalf("zero interval Next() delta = %s, want 1h fallback", got.Sub(base))
	}
}

func TestParseClock(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{in: "00:00", want: 0},
		{in: "09:00", want: 9 * time.Hour},
		{in: "22:30", want: 22*time.Hour + 30*time.Minute},
		{in: "23:59", want: 23*time.Hour + 59*time.Minute},
		{in: "24:00", wantErr: true},
		{in: "09:60", wantErr: true},
		{in: "9:00", want: 9 * time.Hour},
		{in: "bad", wantErr: true},
		{in: "", wantErr: true},
		{in: "09:00:00", wantErr: true},
	}

	for _, tc := range cases {
		got, err := ParseClock(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("ParseClock(%q) err = nil, want error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("ParseClock(%q) err = %v, want nil", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("ParseClock(%q) = %s, want %s", tc.in, got, tc.want)
		}
	}
}
