// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syntax

import "testing"

// sgoKeywords are the reserved words S-Go adds to the token range _Break.._Var
// (SGO_SPEC_SUBSET0.md SGO-LEX-003). The tokens themselves are added in W03.
var sgoKeywords = []string{"nursery"}

// TestKeywordHashRoom checks that the perfect hash used by keywordMap gives every
// existing keyword and every planned S-Go keyword its own slot. The scanner's init
// panics ("imperfect hash") on a collision, so a failure here means adding the S-Go
// keyword token would crash the compiler at startup.
func TestKeywordHashRoom(t *testing.T) {
	slots := make(map[uint]string)
	add := func(kw string) {
		h := hash([]byte(kw))
		if h >= uint(len(keywordMap)) {
			t.Fatalf("hash(%q) = %d out of range for table of %d", kw, h, len(keywordMap))
		}
		if other, ok := slots[h]; ok {
			t.Errorf("keyword %q collides with %q at slot %d (table size %d)", kw, other, h, len(keywordMap))
		}
		slots[h] = kw
	}
	for tok := _Break; tok <= _Var; tok++ {
		add(tok.String())
	}
	for _, kw := range sgoKeywords {
		add(kw)
	}
}

// TestKeywordsStillScan checks that every Go keyword still scans as its own token
// after the table change, and that the planned S-Go keywords remain plain names
// until their tokens exist.
func TestKeywordsStillScan(t *testing.T) {
	for tok := _Break; tok <= _Var; tok++ {
		if got := keywordMap[hash([]byte(tok.String()))]; got != tok {
			t.Errorf("keyword %q maps to %v, want %v", tok.String(), got, tok)
		}
	}
	for _, kw := range sgoKeywords {
		if got := keywordMap[hash([]byte(kw))]; got != 0 {
			t.Errorf("%q maps to token %v before its token is defined", kw, got)
		}
	}
}
