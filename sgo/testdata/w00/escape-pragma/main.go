//go:build sgo_compat

// sgo:legacy
// Attempts to enable old semantics via build tags and pragmas. No such mechanism exists (S01).
package main

import _ "unsafe"

//go:linkname nanotime runtime.nanotime
func nanotime() int64

func main() { println(nanotime()) }
