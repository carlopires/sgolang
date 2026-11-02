// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Probe for the W11 fixed-stack layout (sgo/docs/w11-architecture.md §2). Host tooling, built with
// the isolated reference toolchain; it changes nothing in the runtime. Output is one JSON object
// per line on stdout.
//
//	probe density -size MiB -guard KiB -max N      reserve until N or failure; report at checkpoints
//	probe guard   -size MiB -guard KiB             write below lo: the process must die by a fault
//	probe reuse   -size MiB                         pattern, release, read back: zeroed or not
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const madvFreeReusable = 7 // darwin MADV_FREE_REUSABLE

func emit(v map[string]any) {
	b, _ := json.Marshal(v)
	fmt.Println(string(b))
}

// reserve maps size+guard bytes and makes the low guard bytes inaccessible; it returns lo.
func reserve(size, guard int) (base []byte, lo uintptr, err error) {
	flags := syscall.MAP_PRIVATE | syscall.MAP_ANON
	if runtime.GOOS == "linux" {
		flags |= 0x4000 // MAP_NORESERVE
	}
	b, err := syscall.Mmap(-1, 0, size+guard, syscall.PROT_READ|syscall.PROT_WRITE, flags)
	if err != nil {
		return nil, 0, fmt.Errorf("mmap: %w", err)
	}
	if err := syscall.Mprotect(b[:guard], syscall.PROT_NONE); err != nil {
		syscall.Munmap(b)
		return nil, 0, fmt.Errorf("mprotect: %w", err)
	}
	return b, uintptr(unsafe.Pointer(&b[guard])), nil
}

// usage reports virtual size and RSS in bytes, and the mapping count (-1 if unknown).
func usage() (vsz, rss int64, maps int) {
	maps = -1
	if runtime.GOOS == "linux" {
		f, _ := os.Open("/proc/self/status")
		s := bufio.NewScanner(f)
		for s.Scan() {
			fs := strings.Fields(s.Text())
			if len(fs) >= 2 && (fs[0] == "VmSize:" || fs[0] == "VmRSS:") {
				n, _ := strconv.ParseInt(fs[1], 10, 64)
				if fs[0] == "VmSize:" {
					vsz = n << 10
				} else {
					rss = n << 10
				}
			}
		}
		f.Close()
		data, _ := os.ReadFile("/proc/self/maps")
		maps = strings.Count(string(data), "\n")
		return
	}
	out, err := exec.Command("ps", "-o", "vsz=,rss=", "-p", strconv.Itoa(os.Getpid())).Output()
	if err == nil {
		fs := strings.Fields(string(out))
		if len(fs) == 2 {
			v, _ := strconv.ParseInt(fs[0], 10, 64)
			r, _ := strconv.ParseInt(fs[1], 10, 64)
			vsz, rss = v<<10, r<<10
		}
	}
	return
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: probe density|guard|reuse [flags]")
		os.Exit(2)
	}
	mode := os.Args[1]
	fs := flag.NewFlagSet(mode, flag.ExitOnError)
	sizeMiB := fs.Int("size", 8, "usable stack MiB")
	guardKiB := fs.Int("guard", 64, "guard KiB below lo")
	max := fs.Int("max", 100000, "density: stop after this many reservations")
	child := fs.Bool("child", false, "guard: the faulting child")
	fs.Parse(os.Args[2:])
	size, guard := *sizeMiB<<20, *guardKiB<<10
	page := os.Getpagesize()
	switch mode {
	case "density":
		var keep [][]byte
		start := time.Now()
		next := 1000
		for i := 1; i <= *max; i++ {
			b, lo, err := reserve(size, guard)
			if err != nil {
				vsz, rss, maps := usage()
				emit(map[string]any{"mode": mode, "event": "exhausted", "n": i - 1, "error": err.Error(),
					"vsz": vsz, "rss": rss, "maps": maps, "secs": time.Since(start).Seconds()})
				return
			}
			// An idle task touches its top page (the first frames).
			*(*byte)(unsafe.Pointer(lo + uintptr(size) - 1)) = 1
			keep = append(keep, b)
			if i == next || i == *max {
				vsz, rss, maps := usage()
				emit(map[string]any{"mode": mode, "event": "checkpoint", "n": i, "vsz": vsz, "rss": rss,
					"maps": maps, "secs": time.Since(start).Seconds(), "page": page, "size": size, "guard": guard})
				next *= 2
				if next > *max && i != *max {
					next = *max
				}
			}
		}
		emit(map[string]any{"mode": mode, "event": "done", "n": len(keep)})
	case "guard":
		if *child {
			_, lo, err := reserve(size, guard)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(3)
			}
			*(*byte)(unsafe.Pointer(lo - uintptr(page))) = 1 // one page below lo: inside the guard
			os.Exit(0)                                       // not reached if the guard holds
		}
		cmd := exec.Command(os.Args[0], "guard", "-child", "-size", strconv.Itoa(*sizeMiB), "-guard", strconv.Itoa(*guardKiB))
		err := cmd.Run()
		res := map[string]any{"mode": mode, "page": page, "guard": guard}
		if ee, ok := err.(*exec.ExitError); ok {
			ws := ee.Sys().(syscall.WaitStatus)
			res["signaled"], res["signal"], res["exit"] = ws.Signaled(), ws.Signal().String(), ws.ExitStatus()
		} else {
			res["signaled"], res["error"] = false, fmt.Sprint(err)
		}
		// Go turns the fault into a panic exit (2) with "unexpected fault address"; both count as caught.
		emit(res)
	case "reuse":
		b, lo, err := reserve(size, guard)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(3)
		}
		usable := b[guard:]
		for i := 0; i < len(usable); i += page {
			usable[i] = 0xA5
		}
		_, rssBefore, _ := usage()
		advice := syscall.MADV_DONTNEED
		if runtime.GOOS == "darwin" {
			advice = madvFreeReusable
		}
		if _, _, e := syscall.Syscall(syscall.SYS_MADVISE, uintptr(unsafe.Pointer(&usable[0])), uintptr(len(usable)), uintptr(advice)); e != 0 {
			err = e
		}
		_, rssAfter, _ := usage()
		survivors := 0
		for i := 0; i < len(usable); i += page {
			if usable[i] == 0xA5 {
				survivors++
			}
		}
		emit(map[string]any{"mode": mode, "advice": advice, "madvise_error": fmt.Sprint(err), "pages": len(usable) / page,
			"pages_with_old_data": survivors, "rss_before": rssBefore, "rss_after": rssAfter, "lo": fmt.Sprintf("%#x", lo)})
	default:
		fmt.Fprintln(os.Stderr, "unknown mode", mode)
		os.Exit(2)
	}
}
