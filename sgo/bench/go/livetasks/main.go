// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

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
