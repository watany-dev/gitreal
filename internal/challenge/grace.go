package challenge

const (
	DefaultGraceSeconds = 120
	MinGraceSeconds     = 1
	MaxGraceSeconds     = 3600
)

func NormalizeGraceSeconds(value int) int {
	switch {
	case value < MinGraceSeconds:
		return MinGraceSeconds
	case value > MaxGraceSeconds:
		return MaxGraceSeconds
	default:
		return value
	}
}
