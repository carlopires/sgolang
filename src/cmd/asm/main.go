// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"flag"
	"fmt"
	"internal/buildcfg"
	"log"
	"os"

	"cmd/asm/internal/arch"
	"cmd/asm/internal/asm"
	"cmd/asm/internal/flags"
	"cmd/asm/internal/lex"

	"cmd/internal/bio"
	"cmd/internal/obj"
	"cmd/internal/objabi"
	"cmd/internal/telemetry/counter"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("asm: ")
	counter.Open()

	buildcfg.Check()
	GOARCH := buildcfg.GOARCH

	flags.Parse()
	sgoCheckToolInput()
	counter.Inc("asm/invocations")
	counter.CountFlags("asm/flag:", *flag.CommandLine)

	architecture := arch.Set(GOARCH, *flags.Shared || *flags.Dynlink)
	if architecture == nil {
		log.Fatalf("unrecognized architecture %s", GOARCH)
	}
	ctxt := obj.Linknew(architecture.LinkArch)
	ctxt.CompressInstructions = flags.DebugFlags.CompressInstructions != 0
	ctxt.Debugasm = flags.PrintOut
	ctxt.Debugvlog = flags.DebugV
	ctxt.Flag_dynlink = *flags.Dynlink
	ctxt.Flag_linkshared = *flags.Linkshared
	ctxt.Flag_shared = *flags.Shared || *flags.Dynlink
	ctxt.Flag_maymorestack = flags.DebugFlags.MayMoreStack
	ctxt.Debugpcln = flags.DebugFlags.PCTab
	ctxt.IsAsm = true
	ctxt.Pkgpath = *flags.Importpath
	ctxt.Std = *flags.Std
	ctxt.DwTextCount = objabi.DummyDwarfFunctionCountForAssembler()
	switch *flags.Spectre {
	default:
		log.Printf("unknown setting -spectre=%s", *flags.Spectre)
		os.Exit(2)
	case "":
		// nothing
	case "index":
		// known to compiler; ignore here so people can use
		// the same list with -gcflags=-spectre=LIST and -asmflags=-spectre=LIST
	case "all", "ret":
		ctxt.Retpoline = true
	}

	ctxt.Bso = bufio.NewWriter(os.Stdout)
	defer ctxt.Bso.Flush()

	architecture.Init(ctxt)

	// Create object file, write header.
	buf, err := bio.Create(*flags.OutputFile)
	if err != nil {
		log.Fatal(err)
	}
	defer buf.Close()

	if !*flags.SymABIs {
		buf.WriteString(objabi.HeaderString())
		fmt.Fprintf(buf, "!\n")
	}

	// Set macros for GOEXPERIMENTs so we can easily switch
	// runtime assembly code based on them.
	if objabi.LookupPkgSpecial(ctxt.Pkgpath).AllowAsmABI {
		for _, exp := range buildcfg.Experiment.Enabled() {
			flags.D = append(flags.D, "GOEXPERIMENT_"+exp)
		}
	}

	var ok, diag bool
	var failedFile string
	for _, f := range flag.Args() {
		lexer := lex.NewLexer(f)
		parser := asm.NewParser(ctxt, architecture, lexer)
		ctxt.DiagFunc = func(format string, args ...any) {
			diag = true
			log.Printf(format, args...)
		}
		if *flags.SymABIs {
			ok = parser.ParseSymABIs(buf)
		} else {
			pList := new(obj.Plist)
			pList.Firstpc, ok = parser.Parse()
			// reports errors to parser.Errorf
			if ok {
				obj.Flushplist(ctxt, pList, nil)
			}
		}
		if !ok {
			failedFile = f
			break
		}
	}
	if ok && !*flags.SymABIs {
		ctxt.NumberSyms()
		obj.WriteObjFile(ctxt, buf)
	}
	if !ok || diag {
		if failedFile != "" {
			log.Printf("assembly of %s failed", failedFile)
		} else {
			log.Print("assembly failed")
		}
		buf.Close()
		os.Remove(*flags.OutputFile)
		os.Exit(1)
	}
}

// sgoAsmFlags are the assembler flags the sgo driver writes when it assembles trusted target code
// (cmd/sgo tcb.go); every other flag is refused (D77, W12 R3): refuse-by-default, so a flag added
// upstream stays refused until it is added here deliberately.
var sgoAsmFlags = []string{"o", "p", "std", "I", "D", "trimpath", "gensymabis"}

// sgoCheckToolInput refuses, with SGO_E_USAGE and exit status 2, an assembler flag or target setting
// the driver never uses.
func sgoCheckToolInput() {
	var bad []string
	flag.CommandLine.Visit(func(f *flag.Flag) {
		for _, a := range sgoAsmFlags {
			if f.Name == a {
				return
			}
		}
		bad = append(bad, fmt.Sprintf("asm flag -%s is not admitted (SGO-TOOL-001)", f.Name))
	})
	bad = append(bad, buildcfg.SGoTargetConfigErrs()...)
	for _, b := range bad {
		fmt.Fprintf(os.Stderr, "sgo:1:1: SGO_E_USAGE: %s\n", b)
	}
	if len(bad) > 0 {
		os.Exit(2)
	}
}
