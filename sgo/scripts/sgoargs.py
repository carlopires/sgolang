# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
"""sgoargs.py - strict command lines for the S-Go scripts (W12 R13, R14).

A script that writes evidence or artifacts must refuse what it does not specify: an unknown or
misspelt argument would otherwise run a different workload, or record a configuration nobody
chose. check() validates sys.argv against the script's options before anything runs: -h/--help
prints the script's documentation and exits 0; anything else unknown exits 2 with nothing done.
"""
import sys


def check(flags=None, positional=None, max_positional=None):
    """flags: {"--name": "bool" | "value"}; positional: None (none allowed), a set of allowed values,
    or "any". Returns nothing; exits 0 for help and 2 for an invalid command line."""
    import __main__
    flags = flags or {}
    args = sys.argv[1:]
    if any(a in ("-h", "--help") for a in args):
        print((__main__.__doc__ or "").strip())
        sys.exit(0)
    npos = 0
    i = 0
    while i < len(args):
        a = args[i]
        name, eq, _ = a.partition("=")
        if a.startswith("-") and a != "-":
            kind = flags.get(name)
            if kind is None:
                usage(f"unknown option {a}")
            if kind == "value" and eq:
                usage(f"write {name} VALUE, not {a}")
            if kind == "value":
                if i + 1 >= len(args):
                    usage(f"option {a} needs a value")
                i += 1
            elif kind == "bool" and eq:
                usage(f"option {name} takes no value")
        else:
            npos += 1
            if positional is None or positional != "any" and a not in positional:
                usage(f"unexpected argument {a!r}")
            if max_positional is not None and npos > max_positional:
                usage(f"too many arguments")
        i += 1


def usage(msg):
    import os
    print(f"{os.path.basename(sys.argv[0])}: {msg} (--help for usage)", file=sys.stderr)
    sys.exit(2)
