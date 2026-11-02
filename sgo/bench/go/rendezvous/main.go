// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// 100000 values over an unbuffered channel from a goroutine to main.
package main

func main() {
	c := make(chan int)
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
