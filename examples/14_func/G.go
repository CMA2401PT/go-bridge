package funcsample

// Counter retains shared state in a Go closure.
func Counter(start int) func(int) int {
	return func(delta int) int { start += delta; return start }
}
func Apply(value func(int) int, delta int) int { return value(delta) }
func Echo(value func(int) int) func(int) int   { return value }
func Nil() func(int) int                       { return nil }
