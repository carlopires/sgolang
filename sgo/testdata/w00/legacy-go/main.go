package main

import "fmt"

// Ordinary Go with a bare go statement. Must be rejected by every S-Go target path (OWN-5, S05).
func main() {
	done := make(chan bool)
	go func() { fmt.Println("detached"); done <- true }()
	<-done
}
