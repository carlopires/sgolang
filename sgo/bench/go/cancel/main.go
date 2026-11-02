// Cancel 1000 goroutines sleeping on a context and wait for them.
package main

import (
	"context"
	"sync"
	"time"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case <-time.After(600 * time.Second):
			case <-ctx.Done():
			}
		}()
	}
	time.Sleep(10 * time.Millisecond)
	cancel()
	wg.Wait()
}
