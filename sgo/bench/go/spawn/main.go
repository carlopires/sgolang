// Spawn and join 10000 goroutines that return at once.
package main

import "sync"

func main() {
	var wg sync.WaitGroup
	for i := 0; i < 10000; i++ {
		wg.Add(1)
		go func() { wg.Done() }()
	}
	wg.Wait()
}
