package challenge

import (
	"testing"
	"testing/quick"
)

func TestNormalizeGraceSeconds(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		input int
		want  int
	}{
		{name: "below minimum", input: -10, want: MinGraceSeconds},
		{name: "minimum", input: MinGraceSeconds, want: MinGraceSeconds},
		{name: "default", input: DefaultGraceSeconds, want: DefaultGraceSeconds},
		{name: "maximum", input: MaxGraceSeconds, want: MaxGraceSeconds},
		{name: "above maximum", input: MaxGraceSeconds + 1, want: MaxGraceSeconds},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := NormalizeGraceSeconds(tc.input); got != tc.want {
				t.Fatalf("NormalizeGraceSeconds(%d) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

func TestNormalizeGraceSecondsProperties(t *testing.T) {
	t.Parallel()

	if err := quick.Check(func(input int) bool {
		got := NormalizeGraceSeconds(input)
		return got >= MinGraceSeconds && got <= MaxGraceSeconds
	}, nil); err != nil {
		t.Fatalf("range property failed: %v", err)
	}

	if err := quick.Check(func(input int) bool {
		got := NormalizeGraceSeconds(input)
		return NormalizeGraceSeconds(got) == got
	}, nil); err != nil {
		t.Fatalf("idempotency property failed: %v", err)
	}
}
