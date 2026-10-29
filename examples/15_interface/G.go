package interfacesample

import "fmt"

type Counter interface{ Add(int) int }
type counter struct{ n int }

func (v *counter) Add(delta int) int {
	if v == nil {
		return -1
	}
	v.n += delta
	return v.n
}
func New(start int) Counter      { return &counter{n: start} }
func Echo(value Counter) Counter { return value }
func Nil() Counter               { return nil }
func TypedNil() Counter          { return (*counter)(nil) }

// Result returns error as an ordinary interface value.
func Result(fail bool) (int, error) {
	if fail {
		return 0, fmt.Errorf("ordinary error")
	}
	return 7, nil
}
