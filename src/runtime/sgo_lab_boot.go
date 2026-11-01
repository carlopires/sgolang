// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Lab binaries (D41): the linker's -sgolab sets sgoLabBuild; only then does the root task read the
// run's manifest from fd 3 (written by sgo lab) and enable the lab, and the run's canonical event
// lines are written to fd 4 when it ends. A production binary has sgoLabBuild empty: no input of
// any kind enables the lab in it.

package runtime

import "unsafe"

// sgoLabBuild is "lab" in a lab binary (set by the linker, never by -X).
var sgoLabBuild string

const (
	sgoLabManifestFD = 3
	sgoLabTraceFD    = 4
)

// sgoLabBoot enables the lab in the root task of a lab binary, from its manifest:
//
//	seed <n>          master seed (D44)
//	policy <random|fifo>
//	k <n>             steps per scheduling slice (D43)
//	quota <n>         total steps (D43)
//	replay <p,...>    replay: the pick of every choose and select event whose set has two or
//	                  more elements, in order (D46, replayPick)
//
// one key per line. A lab binary started without a manifest is a usage defect (status 4).
func sgoLabBoot() {
	if sgoLabBuild != "lab" {
		return
	}
	data, ok := sgoLabReadAll(sgoLabManifestFD)
	if !ok {
		sgoDefect("a lab binary runs only under sgo lab (no lab manifest on fd 3)")
	}
	var seed uint64
	fifo, k, quota := false, int64(1024), int64(1)<<40
	var replay []int32
	var haveReplay bool
	for len(data) > 0 {
		line := data
		if i := sgoIndexByte(data, '\n'); i >= 0 {
			line, data = data[:i], data[i+1:]
		} else {
			data = ""
		}
		key, val := line, ""
		if i := sgoIndexByte(line, ' '); i >= 0 {
			key, val = line[:i], line[i+1:]
		}
		switch key {
		case "seed":
			seed = uint64(sgoLabAtoi(val))
		case "policy":
			fifo = val == "fifo"
			if val != "fifo" && val != "random" {
				sgoDefect("lab manifest: unknown policy " + val)
			}
		case "k":
			k = sgoLabAtoi(val)
		case "quota":
			quota = sgoLabAtoi(val)
		case "replay":
			haveReplay = true
			for val != "" {
				f := val
				if i := sgoIndexByte(val, ','); i >= 0 {
					f, val = val[:i], val[i+1:]
				} else {
					val = ""
				}
				replay = append(replay, int32(sgoLabAtoi(f)))
			}
		case "":
		default:
			sgoDefect("lab manifest: unknown key " + key)
		}
	}
	sgoLabEnable(seed, fifo, k, quota, sgoLabExit)
	if haveReplay {
		sgoLab.replay, sgoLab.replaying = replay, true
	}
}

// sgoLabExit ends a lab binary's process: the record of a lab outcome (D47), then the canonical
// lines on fd 4, then the exit. A production outcome's record was already written.
func sgoLabExit(status int32) {
	l := sgoLab
	switch l.kind {
	case "LabDeadlock":
		sgoWriteRecord(sgoFailureInfo{l.kind, "every live task is waiting and no timer is pending", ""}, nil, sgoOrigin{})
	case "LabQuotaExhausted":
		sgoWriteRecord(sgoFailureInfo{l.kind, "the step quota of the run is used up", ""}, nil, sgoOrigin{})
	case "LabReplayDivergence":
		sgoWriteRecord(sgoFailureInfo{l.kind, "the program made a choice the trace does not allow", ""}, nil, sgoOrigin{})
	}
	for _, s := range l.lines() {
		s += "\n"
		p := unsafe.Pointer(unsafe.StringData(s))
		for done := 0; done < len(s); {
			n := write1(sgoLabTraceFD, add(p, uintptr(done)), int32(len(s)-done))
			if n == -_EINTR {
				continue
			}
			if n <= 0 {
				break
			}
			done += int(n)
		}
	}
	exit(status)
}

// sgoProgramExit ends an S-Go program with status: in a lab binary the outcome is recorded and
// the trace written first (sgoLabProgramEnd calls the exit hook).
func sgoProgramExit(status int32, kind string) {
	if sgoLab != nil {
		sgoLabProgramEnd(status, kind)
	}
	exit(status)
}

// sgoLabReadAll reads fd to its end; false if fd cannot be read.
func sgoLabReadAll(fd int32) (string, bool) {
	var b []byte
	buf := make([]byte, 4096) // on the heap: read's buffer escapes on some systems
	for {
		n := read(fd, unsafe.Pointer(&buf[0]), int32(len(buf)))
		if n == -_EINTR {
			continue
		}
		if n < 0 {
			return "", false
		}
		if n == 0 {
			return string(b), true
		}
		b = append(b, buf[:n]...)
	}
}

func sgoIndexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// sgoLabAtoi parses a non-negative decimal manifest value.
func sgoLabAtoi(s string) int64 {
	if s == "" {
		sgoDefect("lab manifest: empty number")
	}
	var v int64
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' || v > (1<<62)/10 {
			sgoDefect("lab manifest: bad number " + s)
		}
		v = v*10 + int64(c-'0')
	}
	return v
}
