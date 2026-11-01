#!/usr/bin/env python3
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
"""S12 (W07 part): compiled S-Go code reaches channels only through the S-Go entry points.

Builds a module whose main package and library package use every channel form admitted by spec
0.3.0 (make, send, receive with and without ok, close, range, select with and without default,
single-arm select, select {}, directional parameters, inlined library helpers), disassembles every
function of the module's packages, and requires that they call no Go channel entry point
(runtime.chansend1, chanrecv1, chanrecv2, closechan, makechan, makechan64, chanlen, chancap,
selectgo, selectnbsend, selectnbrecv, block, reflect channel entry points) and that they do call
the S-Go ones (SGO-CHAN-002, SGO-CHAN-004). The program is also run.
"""

import os
import platform
import re
import subprocess
import sys
import tempfile

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
GOOS = {"Darwin": "darwin", "Linux": "linux"}[platform.system()]
GOARCH = {"x86_64": "amd64", "amd64": "amd64", "arm64": "arm64", "aarch64": "arm64"}[platform.machine()]
SGO = os.path.join(ROOT, "pkg", "tool", f"{GOOS}_{GOARCH}", "sgo")
VERSION = open(os.path.join(ROOT, "sgo", "spec", "VERSION")).read().strip()

LIB = '''package lib

// Forward copies every value of in to out, then closes out.
func Forward(in <-chan int, out chan<- int) {
	for v := range in {
		out <- v
	}
	close(out)
}

// Poll receives a value if one is ready, without waiting (ok is false if the channel is closed).
func Poll(c chan int) (int, bool) {
	select {
	case v, ok := <-c:
		return v, ok
	default:
		return 0, false
	}
}

// Offer sends v if a slot is free.
func Offer(c chan int, v int) bool {
	select {
	case c <- v:
		return true
	default:
		return false
	}
}
'''

MAIN = '''package main

import (
	"sgo"

	"example.com/lowering/lib"
)

func main(root *sgo.Context) error {
	out := root.Stdout()
	a := make(chan int, 4)
	b := make(chan int)
	done := make(chan bool, 1)
	nursery (n) {
		n.Spawn(func(task *sgo.Context) error {
			lib.Forward(a, b)
			return nil
		})
		n.Spawn(func(task *sgo.Context) error {
			select {}
		})
		if !lib.Offer(a, 1) {
			return sgo.NewError("offer failed")
		}
		a <- 2
		close(a)
		sum := 0
		for {
			v, ok := <-b
			if !ok {
				break
			}
			sum = sum + v
		}
		if _, ready := lib.Poll(b); ready {
			return sgo.NewError("closed channel polled a value")
		}
		select {
		case done <- true:
		}
		<-done
		select {
		case v := <-b:
			sum = sum + v
		case done <- true:
		}
		if sum != 3 {
			return sgo.NewError("bad sum")
		}
		if err := out.WriteString("lowered\\n"); err != nil {
			return err
		}
		n.Cancel()
	}
	return nil
}
'''

STOCK = """package main

func main() {
	a, b := make(chan int, 1), make(chan int, 1)
	a <- 1
	select {
	case v := <-a:
		b <- v
	case b <- 2:
	}
	_, ok := <-b
	close(a)
	println(ok)
}
"""

GO_CHAN = re.compile(r"\b(?:CALL|BL|JMP|B)\s+runtime\.(chansend1|chanrecv1|chanrecv2|closechan|makechan|makechan64|"
                     r"chanlen|chancap|selectgo|selectnbsend|selectnbrecv|block|reflect_\w+)\b")
SGO_CHAN = re.compile(r"\b(?:CALL|BL|JMP|B)\s+runtime\.(sgo(?:makechan|chansend1|chanrecv1|chanrecv2|closechan|"
                      r"selectgo|selectnbsend|selectnbrecv|block))\b")


def run(args, cwd, env):
    r = subprocess.run(args, cwd=cwd, env=env, capture_output=True, text=True)
    if r.returncode != 0:
        sys.exit(f"test-native-lowering: {' '.join(args[:3])} failed:\n{r.stdout}{r.stderr}")
    return r.stdout


def main():
    boot = subprocess.run([os.path.join(ROOT, "sgo", "scripts", "fetch-bootstrap.sh")], capture_output=True,
                          text=True, check=True).stdout.strip()
    with tempfile.TemporaryDirectory() as tmp:
        tmp = os.path.realpath(tmp)
        env = {"HOME": tmp, "PATH": "/usr/bin:/bin", "SGO_CACHE": os.path.join(tmp, "cache"), "GOROOT": ROOT,
               "GOOS": GOOS, "GOARCH": GOARCH, "CGO_ENABLED": "0", "GOEXPERIMENT": "", "GODEBUG": ""}
        mod = os.path.join(tmp, "lowering")
        os.makedirs(os.path.join(mod, "lib"))
        open(os.path.join(mod, "sgo.mod"), "w").write(f"module example.com/lowering\n\nsgo {VERSION}\n")
        open(os.path.join(mod, "main.sgo"), "w").write(MAIN)
        open(os.path.join(mod, "lib", "lib.sgo"), "w").write(LIB)
        prog = os.path.join(tmp, "prog")
        run([SGO, "build", "-o", prog, "."], mod, env)
        got = run([prog], mod, env)
        if got != "lowered\n":
            sys.exit(f"test-native-lowering: program printed {got!r}")
        benv = {"HOME": tmp, "PATH": "/usr/bin:/bin", "GOROOT": boot, "GOCACHE": os.path.join(tmp, "gocache")}
        dis = run([os.path.join(boot, "bin", "go"), "tool", "objdump", "-s", r"^(main|example\.com/lowering/lib)\.", prog],
                  tmp, benv)
        bad = sorted(set(GO_CHAN.findall(dis)))
        good = sorted(set(SGO_CHAN.findall(dis)))
        if bad:
            sys.exit(f"test-native-lowering: S-Go code calls Go channel entry points: {bad}")
        want = {"sgomakechan", "sgochansend1", "sgochanrecv1", "sgochanrecv2", "sgoclosechan", "sgoselectgo",
                "sgoselectnbsend", "sgoselectnbrecv", "sgoblock"}
        if missing := sorted(want - set(good)):
            sys.exit(f"test-native-lowering: expected calls missing (the sample no longer covers them?): {missing}")
        # Self-test: the same disassembly check on a stock Go program does see Go's entry points.
        gomod = os.path.join(tmp, "stock")
        os.makedirs(gomod)
        open(os.path.join(gomod, "main.go"), "w").write(STOCK)
        stock = os.path.join(tmp, "stock-prog")
        run([os.path.join(boot, "bin", "go"), "build", "-o", stock, "main.go"], gomod, dict(benv, GO111MODULE="off"))
        sdis = run([os.path.join(boot, "bin", "go"), "tool", "objdump", "-s", r"^main\.", stock], tmp, benv)
        if not {"chansend1", "chanrecv2", "closechan", "selectgo"} <= set(GO_CHAN.findall(sdis)):
            sys.exit("test-native-lowering: self-test failed: the check does not see Go channel calls in stock Go code")
        print(f"test-native-lowering: ok: {len(good)} S-Go channel entry points called, no Go channel entry point (self-test on stock Go code sees them)")


if __name__ == "__main__":
    main()
