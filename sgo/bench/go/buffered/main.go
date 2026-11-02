// 100000 values over an channel of capacity 128 from a goroutine to main.
package main

func main() {
	c := make(chan int, 128)
	go func() {
		for i := 0; i < 100000; i++ {
			c <- i
		}
		close(c)
	}()
	sum := 0
	for v := range c {
		sum += v
	}
	if sum != 4999950000 {
		panic("bad sum")
	}
}
