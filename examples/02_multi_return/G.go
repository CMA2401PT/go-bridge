package multireturn

func ReturnTwo(i int) (i1 int, i2 int) {
	return i + 1, i + 2
}

func ReturnFour(i int) (i1 int, i2 int, i3 int, i4 int) {
	return i + 1, i + 2, i + 3, i + 4
}
