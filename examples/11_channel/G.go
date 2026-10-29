// Package channel demonstrates channel ownership and blocking across Python and Go.
package channel

func Numbers(count int) <-chan int {
	ch := make(chan int)
	go func() {
		defer close(ch)
		for i := 0; i < count; i++ {
			ch <- i
		}
	}()
	return ch
}

func Sum(ch <-chan int) int {
	total := 0
	for value := range ch {
		total += value
	}
	return total
}

func Send(ch chan<- int, value int)       { ch <- value }
func Echo(ch chan int) chan int           { return ch }
func EchoText(ch chan string) chan string { return ch }
