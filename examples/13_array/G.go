package arraysample

// Echo copies the array; Increment changes its own copy.
func Echo(value [3]int) [3]int      { return value }
func Increment(value [3]int) [3]int { value[0]++; return value }
func Mutate(value *[3]int)          { value[0]++ }
