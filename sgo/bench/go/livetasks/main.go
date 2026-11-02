// 10000 goroutines blocked at the same time on a receive, released by a close.
package main

import (
	"sync"
	"time"
)

func main() {
	c := make(chan int)
	var wg sync.WaitGroup
	for i := 0; i < 10000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-c
		}()
	}
	time.Sleep(50 * time.Millisecond)
	close(c)
	wg.Wait()
}
