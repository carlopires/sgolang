// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Sealed runtime services (W02, TCB-P11 of SGO-TCB-001, OWN-6). Every goroutine created through the
// go-statement path (newproc) is classified by the runtime function that created it. Only the
// creation sites enumerated in sgo/TARGET_TCB.json "services" are runtime services; each gets an
// unforgeable service kind and a lifecycle count. With enforcement on (every S-Go program, from W03),
// any other go-statement creation is a fail-stop TCB defect: S-Go source cannot contain `go`, so the
// only legitimate callers of newproc are these runtime sites.

package runtime

import "internal/runtime/atomic"

type sgoServiceKind uint8

const (
	sgoSvcNone          sgoServiceKind = iota
	sgoSvcGCEnable                     // bgsweep, bgscavenge (gcenable)
	sgoSvcGCMarkWorker                 // gcBgMarkWorker (gcBgMarkStartWorkers)
	sgoSvcForceGC                      // forcegchelper (runtime init)
	sgoSvcMaxProcs                     // updateMaxProcsGoroutine (defaultGOMAXPROCSUpdateEnable)
	sgoSvcFinalizer                    // runFinalizers (createfing); dormant in subset 0
	sgoSvcCleanup                      // runCleanups ((*cleanupQueue).createGs); dormant in subset 0
	sgoSvcSignal                       // ensureSigM; dormant in subset 0 (os/signal denied)
	sgoSvcTraceAdvancer                // (*traceAdvancerState).start; dormant (runtime/trace denied)
	sgoSvcTraceCPU                     // traceStartReadCPU; dormant (runtime/trace denied)
	sgoSvcJSEvent                      // beforeIdle; js/wasm only, not an admitted target
	sgoSvcLabDriver                    // sgoLabDriver ((*sgoLabState).ensureDriver); lab binaries only (D170, D179)
	sgoSvcStdinPump                    // sgoStdinPump (sgoStdinRead); one blocking read of fd 0 per demand (D178)
	sgoSvcInterrupt                    // sgoInterruptWatch (sgoInterruptArm); cancels the root on SIGINT or SIGTERM (D179)
	sgoSvcCount
)

var sgoServiceNames = [sgoSvcCount]string{"none", "gc-enable", "gc-mark-worker", "force-gc", "maxprocs",
	"finalizer", "cleanup", "signal", "trace-advancer", "trace-cpu", "js-event", "lab-driver", "stdin-pump", "interrupt"}

var (
	// sgoEnforce turns unclassified go-statement creation into a fail-stop defect. Set at start-up of
	// every S-Go program (W03); off in the runtime test lane, whose tests are Go code.
	sgoEnforce atomic.Bool

	sgoServiceStarted [sgoSvcCount]atomic.Int64
	sgoServiceLive    [sgoSvcCount]atomic.Int64
)

// sgoClassifyService maps the creating runtime function (innermost inlined frame at pc) of a
// go statement to its service kind.
func sgoClassifyService(pc uintptr, fn *funcval) sgoServiceKind {
	return sgoClassify(sgoCreatorName(pc), fn)
}

// sgoCreatorName is the innermost inlined function containing the call that returns to pc. pc is a
// return address; pc-1 lies inside the call instruction (as in tracebacks), otherwise an inlined call
// that follows the go statement would be reported instead (e.g. releasem after gcBgMarkStartWorkers).
func sgoCreatorName(pc uintptr) string {
	f := findfunc(pc - 1)
	if !f.valid() {
		return "?"
	}
	u, uf := newInlineUnwinder(f, pc-1)
	return u.srcFunc(uf).name()
}

func sgoClassify(creator string, fn *funcval) sgoServiceKind {
	switch creator {
	case "runtime.gcenable":
		return sgoSvcGCEnable
	case "runtime.gcBgMarkStartWorkers":
		return sgoSvcGCMarkWorker
	case "runtime.defaultGOMAXPROCSUpdateEnable":
		return sgoSvcMaxProcs
	case "runtime.createfing":
		return sgoSvcFinalizer
	case "runtime.(*cleanupQueue).createGs":
		return sgoSvcCleanup
	case "runtime.ensureSigM":
		return sgoSvcSignal
	case "runtime.(*traceAdvancerState).start":
		return sgoSvcTraceAdvancer
	case "runtime.traceStartReadCPU":
		return sgoSvcTraceCPU
	case "runtime.beforeIdle":
		return sgoSvcJSEvent
	case "runtime.sgoStdinRead":
		return sgoSvcStdinPump
	case "runtime.sgoInterruptArm":
		return sgoSvcInterrupt
	case "runtime.(*sgoLabState).ensureDriver":
		if sgoLab != nil { // only a lab run has a deadline driver
			return sgoSvcLabDriver
		}
	}
	// forcegchelper is started from a runtime package init function (numbered name).
	if initPrefix := "runtime.init."; len(creator) > len(initPrefix) && creator[:len(initPrefix)] == initPrefix {
		if cf := findfunc(fn.fn); cf.valid() && funcname(cf) == "runtime.forcegchelper" {
			return sgoSvcForceGC
		}
	}
	return sgoSvcNone
}

// sgoServiceStart is called by newproc for every go-statement creation.
func sgoServiceStart(pc uintptr, fn *funcval) sgoServiceKind {
	k := sgoClassifyService(pc, fn)
	if k == sgoSvcNone {
		if sgoEnforce.Load() {
			print("sgo: creator ", sgoCreatorName(pc), "\n")
			throw("sgo: goroutine created outside the enumerated runtime services (OWN-6)") // RuntimeDefect
		}
		return k
	}
	sgoServiceStarted[k].Add(1)
	sgoServiceLive[k].Add(1)
	return k
}

// sgoServiceExit is called when a goroutine is destroyed.
func sgoServiceExit(gp *g) {
	if k := sgoServiceKind(gp.sgoService); k != sgoSvcNone {
		sgoServiceLive[k].Add(-1)
		gp.sgoService = 0
	}
}
