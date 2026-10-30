// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syntax

import "testing"

// sgoKeywords are the reserved words S-Go adds to the token range _Break.._Var (SGO-LEX-003).
var sgoKeywords = map[string]token{"nursery": _Nursery}

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
	for kw, tok := range sgoKeywords {
		if tok < _Break || tok > _Var {
			t.Errorf("S-Go keyword %q is outside the _Break.._Var range", kw) // keywordMap init only covers that range
		}
	}
}

// TestKeywordsStillScan checks that every reserved word, including the S-Go
// keywords, scans as its own token.
func TestKeywordsStillScan(t *testing.T) {
	for tok := _Break; tok <= _Var; tok++ {
		if got := keywordMap[hash([]byte(tok.String()))]; got != tok {
			t.Errorf("keyword %q maps to %v, want %v", tok.String(), got, tok)
		}
	}
	for kw, tok := range sgoKeywords {
		if got := keywordMap[hash([]byte(kw))]; got != tok {
			t.Errorf("%q maps to %v, want %v", kw, got, tok)
		}
	}
}
