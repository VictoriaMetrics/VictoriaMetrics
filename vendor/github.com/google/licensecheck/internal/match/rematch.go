// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// License regexp compilation and execution.
// See https://swtch.com/~rsc/regexp/regexp2.html
// for detailed explanation of a similar machine.
//
// This machine differs from that one in a few important ways:
// word-at-a-time operation, word canonicalization, spell-checking,
// and early-cut wildcard matching.
//
// Word-at-a-Time Operation
//
// Most regexp matching engines, including the one linked above, process
// one input byte – or perhaps one Unicode code point – at a time.
// In contrast, this engine processes one word at a time.
// The license regular expressions (LREs) and the input texts themselves
// are first split into words, represented by integer word IDs in an
// automatically maintained word list (the Dict type).
// Then the NFA programs and DFA state sets are expressed in terms of
// word IDs, not characters.
//
// Operating on words lets us define wildcards like __10__, meaning
// "up to 10 words", which may be more useful than "up to 100 bytes".
// More importantly, it makes the match completely independent of to
// the exact punctuation and spacing of the input. It also greatly
// shortens the NFA and DFA representations.
//
// Word Canonicalization
//
// Splitting the input into words provides an opportunity to regularize
// the input further. Splitting folds letter case: "SHALL" and "shall"
// are given the same word ID. It also strips accents from vowels:
// "QUÉBEC", "Québec", and "quebec" are all given the same word ID.
//
// Although in general punctuation is ignored, canonicalization recognizes
// the pattern "word(s)" where word is any word and (s) is the literal
// three-byte sequence "(s)"; it canonicalizes that to "words".
// For example, "notice(s)" canonicalizes to "notices". Combined with
// spell checking (see below), "notice(s)" will therefore match
// "notices" or "notice".
//
// Another special case involving punctuation is "(c)" and "©", both of which
// canonicalize to "copyright". A pair of "copyright" words also canonicalizes
// to a single one, so that "Copyright © 2020" and "copyright 2020" are the
// same text. An unfortunate side effect of the "(c)" conversion is that
// the list bullets "c." and "c)" are different words from "(c)".
// That mismatch is handled by spell checking (see below).
//
// There are other common words that are often substituted for each other
// and worth canonicalizing, to simplify license patterns. These include
//
//  - is are
//  - it them they
//  - the these this those
//  - copy copies
//
// See the canonicalRewrites list in dict.go and its uses for the details.
// Regular singular/plural forms are handled by spell checking.
//
// Spell Checking
//
// At each state, the DFA knows which words would help move the match along.
// If it doesn't see one of those words, it would normally end the search.
// That moment provides an opportunity to do spelling correction:
// if the input word being processed is close enough to an expected word,
// we can treat it as having been corrected to the expected word.
//
// More specifically, if both the input and the target word are at least
// four bytes, and if the input can be edited to produce the target by
// a single byte insertion, deletion, or modification, then the words are
// considered close enough, and the misspelled word is treated as the target.
// This handles typos like "notwork" for "network". It also handles
// most singular/plural distinctions, such as "notice" for "notices".
// See the canMisspell function for the implementation.
//
// [One cost of word canonicalization (at least in the current implementation)
// is that spell checking does not consider non-canonical spellings of words.
// For example, "copies" canonicalizes to "copy", meaning that the word
// the DFA expects is "copy" (never "copies"), so an input word "copiesx"
// does NOT get spell-checked to "copies" (and from there to "copy").
// This could be added if necessary.]
//
// Another possible spell-check fix is to join words incorrectly split apart.
// This happens sometimes in text that has been word-wrapped with hyphenation.
// If the next two input words can be joined together to produce an expected
// word, that pair is consumed as a misspelling of the expected one.
// For example, the hyphenation "nonin-" (new line) "fringement" might
// be reassembled as the expected word "noninfringement".
// This also handles non-standard over-hyphenation, such as "non-infringement"
// for "noninfringement" or "sub-license" for "sublicense".
// See canMisspellJoin for the implementation.
//
// Another possible spell-check fix is to split apart words incorrectly joined.
// This happens sometimes in text that has been un-word-wrapped by deleting
// newlines instead of turning them into spaces, or where spaces or other
// separating punctuation have been inadvertently deleted.
// If an expected word is a prefix of the next input word, and the remaining
// suffix is expected after that word, then the single word is consumed as
// a misspelling of the word pair. For example, "andor" can be consumed as
// a misspelling of "and/or".
// The "misspell split" comment marks the implementation.
//
// The inclusion of both the joining and splitting fixes means that
// license patterns can, for example, use either "non-infringement" or
// "noninfringement" and still match the other form without additional work.
// However, the former may be preferable in that it also accepts a
// wrapped and hyphenated "non-infringement" that turns into
// "non-in <newline> fringement", which "noninfringemnt" would not match.
//
// Spell checking also accepts "c" and "copyright" for each other.
// A plain "c" is not canonicalized to "copyright" to avoid making plain "c",
// especially in a file name like "file.c", appear to be the start of a copyright notice.
// (Spell checking only applies inside a potential match that is already started,
// but word canonicalization applies to every word in the file.)
//
// Early-Cut Wildcard Matching
//
// This implementation adds "cut" operations to reduce the number
// of states tracked when using counted wildcard patterns.
//
// To explain what that means exactly, first some background.
//
// DFAs are not good at counting: the standard regexp pattern /.{0,10}/
// is much more expensive than /.*/. On the other hand, /.*/ can easily
// match far too much; the limit in /.{0,10}/ is semantically useful.
//
// LREs provide a counted wildcard __10__ equivalent to regexp /.{0,10}/.
// The counting can produce a multiplicative number of DFA states.
// For example, consider the pattern "The name __10__ may not be used"
// matched against "The name may not be may not be may not be may not be used".
// If __10__ means "up to 10 words, any words at all", then that text should
// be matched, assigning "may not be may not be may not be" to the wildcard.
// This never happens in practice, but the standard DFA construction must be
// prepared for it, along with variants like:
//
//	The name may may may may may may may may may may may not be used.
//	The name may not may not may not may not may not may not be used.
//	The name may may not be  may not be  may not be  may not be used.
//	The name may not may not be used may not be used may not be used.
//
// In general the DFA must track
// (1) how many wildcard words have been used up so far, and
// (2) how far into the "may not be used" has been matched
// at the current input location. That's roughly 10 * 4 = 40 states.
// In a real license, the possible literal text that must be tracked
// is as many words as the wildcard itself allows, leading in this case
// to roughly 10 * 10 = 100 states. A __20__ produces roughly 400 states,
// and so on.
//
// This extra work is clearly useless: real inputs don't look like that,
// and people wouldn't recognize them if they did. After seeing a few of
// the words following the wildcard, we can safely assume that the wildcard
// part of the match is over. In the example above, after seeing
//
//	The name Google may not be
//
// we might as well assume the wildcard is over and the next word must
// be "used", to complete the pattern. This is true even though technically
// the pattern might be interpreted to allow
//
//	The name "Google may not be may not be" may not be used.
//
// Empirically, this kind of abuse does not come up in practice.
//
// This implementation cuts off wildcard matches by inserting an implicit
// "cut" operator (similar to and named after Prolog's cut operator)
// three literal words after each wildcard. The cut discards (cuts off)
// any NFA threads still attempting wildcard matches.
// That is, our example is interpreted implicitly as
//
//	The name __10__ may not be (CUT) used.
//
// The effect of this cut after three words is that the 10 * 10 states
// drops to 10 * 3, 20 * 20 drops to 20 * 3, and so on: wildcards now
// have a footprint only linear in their size, not quadratic.
//
// As a larger example, the implicit cut in the longer pattern
//
//	The name __20__ may not be (CUT) used to endorse or promote products
//	derived from this software without specific prior written permission.
//
// reduces the total number of DFA states from 248 to 80.
//
// It is still possible to delay the cut by following the wildcard with
// optional words or phrases, so some state blowup is still possible,
// but not nearly as much.
//
// Overall, at time of writing, implicit cuts reduce the size of the
// DFA for the full license set from 5.8M states (240 MB and 39s to build)
// to 615k states (10 MB, under one second to build).
//
// The implicit cut is represented as an instCut that terminates any
// other NFA threads still matching a particular wildcard. The application
// of instCut happens in (*nfaState).trim.

package match

import (
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
)

// A reProg is a regexp program: an instruction list.
type reProg []reInst

// A reInst is a regexp instruction: an opcode and a numeric argument
type reInst struct {
	op  instOp
	arg int32
}

// An instOp is the opcode for a regexp instruction.
type instOp int32

const (
	instInvalid instOp = iota

	instWord  // match specific word
	instAny   // match any word
	instAlt   // jump to both pc+1 and pc+1+arg
	instJump  // jump to pc+1+arg
	instMatch // completed match identified by arg
	instCut   // cut off the instAlt range starting at pc+1+arg
)

// string returns a textual listing of the given program.
// The dictionary d supplies the actual words for the listing.
func (p reProg) string(d *Dict) string {
	var b strings.Builder
	words := d.Words()
	for i, inst := range p {
		fmt.Fprintf(&b, "%d\t", i)
		switch inst.op {
		case instWord:
			fmt.Fprintf(&b, "word %s\n", words[inst.arg])
		case instAny:
			fmt.Fprintf(&b, "any\n")
		case instAlt:
			fmt.Fprintf(&b, "alt %d\n", i+1+int(inst.arg))
		case instJump:
			fmt.Fprintf(&b, "jump %d\n", i+1+int(inst.arg))
		case instMatch:
			fmt.Fprintf(&b, "match %d\n", int(inst.arg))
		case instCut:
			// target is always an instAlt.
			// Decode the target of the alt as well.
			targ := i + 1 + int(inst.arg)
			fmt.Fprintf(&b, "cut [%d, %d]\n", targ, targ+1+int(p[targ].arg))
		}
	}
	return b.String()
}

// reCompile holds compilation state for a single regexp.
type reCompile struct {
	prog       reProg // program being constructed
	endPattern bool   // compiling the end of the pattern
	cut        []reCut
	err        error // first problem found; report delayed until end of compile
}

// reCut holds the information about a pending cut.
type reCut struct {
	start   int // cut off the alt at pc = start
	trigger int // ... after trigger more literal word matches
}

// compile appends a program for the regular expression re to init and returns the result.
// A successful match of the program for re will report the match value m.
func (re *reSyntax) compile(init reProg, m int32) (reProg, error) {
	c := &reCompile{prog: init, endPattern: true}
	c.compile(re)
	c.compileCuts()
	return append(c.prog, reInst{op: instMatch, arg: m}), c.err
}

// compile appends the compiled program for re to c.prog.
func (c *reCompile) compile(re *reSyntax) {
	switch re.op {
	default:
		panic(fmt.Sprintf("unexpected re.op %d", re.op))

	case opEmpty:
		// nothing

	case opWords:
		for _, w := range re.w {
			c.prog = append(c.prog, reInst{op: instWord, arg: int32(w)})
			c.reduceCut()
		}
		if c.endPattern {
			c.compileCuts()
		}

	case opConcat:
		endIndex := len(re.sub)
		if c.endPattern {
			for endIndex > 0 && canMatchEmpty(re.sub[endIndex-1]) {
				endIndex--
			}
		}
		for i, sub := range re.sub {
			c.endPattern = i >= endIndex
			c.compile(sub)
		}

	case opQuest:
		alt := len(c.prog)
		c.prog = append(c.prog, reInst{op: instAlt})
		cut := c.cut
		endPattern := c.endPattern
		c.compile(re.sub[0])
		if endPattern {
			c.compileCuts()
		}
		c.cut = c.mergeCut(cut, c.cut)
		c.prog[alt].arg = int32(len(c.prog) - (alt + 1))

	case opAlternate:
		cut := c.cut
		endPattern := c.endPattern
		var newCut []reCut
		var alts, jumps []int
		for i, sub := range re.sub {
			if i+1 < len(re.sub) {
				alts = append(alts, len(c.prog))
				c.prog = append(c.prog, reInst{op: instAlt})
			}
			c.cut = cut
			c.endPattern = endPattern
			c.compile(sub)
			newCut = c.mergeCut(newCut, c.cut)
			if i+1 < len(re.sub) {
				jumps = append(jumps, len(c.prog))
				c.prog = append(c.prog, reInst{op: instJump})
			}
		}
		c.cut = newCut

		// All alts jump to after jump.
		for i, a := range alts {
			c.prog[a].arg = int32((jumps[i] + 1) - (a + 1))
		}

		// Patch all jumps to the end.
		end := len(c.prog)
		for _, j := range jumps {
			c.prog[j].arg = int32(end - (j + 1))
		}

	case opWild:
		// All alts jump to the end of the expression, as if it were
		//	(.(.(.(.)?)?)?)?
		// This results in smaller NFA state lists (max 2 states)
		// than compiling like .?.?.?.? (max re.n states).
		c.compileCuts()
		if c.endPattern && c.err == nil {
			c.err = fmt.Errorf("__%d__ wildcard with no required text following", re.n)
		}
		start := len(c.prog)
		end := len(c.prog) + int(re.n)*2
		for i := int32(0); i < re.n; i++ {
			c.prog = append(c.prog, reInst{op: instAlt, arg: int32(end - (len(c.prog) + 1))})
			c.prog = append(c.prog, reInst{op: instAny})
		}
		if re.n > 3 {
			c.cut = []reCut{{start: start, trigger: 3}}
		}
	}
}

// compileCuts emits instCut instructions for all pending cuts.
// See comment at top of file for information about cuts.
func (c *reCompile) compileCuts() {
	for _, cut := range c.cut {
		c.compileCut(cut)
	}
	c.cut = nil
}

// compileCut emits an instCut instruction for cut.
func (c *reCompile) compileCut(cut reCut) {
	c.prog = append(c.prog, reInst{op: instCut, arg: int32(cut.start - (len(c.prog) + 1))})
}

// reduceCut records that a new literal word has been matched,
// reducing the triggers in c.cut by 1 and emitting any triggered cuts.
func (c *reCompile) reduceCut() {
	var next []reCut
	for _, cut := range c.cut {
		if cut.trigger--; cut.trigger == 0 {
			c.compileCut(cut)
			continue
		}
		next = append(next, cut)
	}
	c.cut = next
}

// mergeCut merges the two cut lists cut1 and cut2 into a single cut list.
// Cuts with the same start but different triggers are merged into a
// single entry with the larger of the two triggers.
func (c *reCompile) mergeCut(cut1, cut2 []reCut) []reCut {
	if len(cut1) == 0 {
		return cut2
	}
	if len(cut2) == 0 {
		return cut1
	}

	var list []reCut
	list = append(list, cut1...)
	list = append(list, cut2...)
	sort.Slice(list, func(i, j int) bool {
		if list[i].start != list[j].start {
			return list[i].start < list[j].start
		}
		return list[i].trigger > list[j].trigger
	})

	w := 0
	for _, cut := range list {
		if w == 0 || list[w-1].start != cut.start {
			list[w] = cut
			w++
		}
	}
	return list[:w]
}

// canMatchEmpty reports whether re can match an empty text.
func canMatchEmpty(re *reSyntax) bool {
	switch re.op {
	case opAlternate:
		for _, sub := range re.sub {
			if canMatchEmpty(sub) {
				return true
			}
		}
		return false

	case opConcat:
		for _, sub := range re.sub {
			if !canMatchEmpty(sub) {
				return false
			}
		}

	case opWords:
		if len(re.w) > 0 {
			return false
		}
	}

	return true
}

// reCompileMulti returns a program that matches any of the listed regexps.
// The regexp list[i] returns match value i when it matches.
func reCompileMulti(list []reProg) reProg {
	var prog reProg
	for i, prog1 := range list {
		alt := -1
		if i+1 < len(list) {
			// Insert Alt that can choose to jump over this program (to the next one).
			alt = len(prog)
			prog = append(prog, reInst{op: instAlt})
		}

		for _, inst := range prog1 {
			if inst.op == instMatch {
				prog = append(prog, reInst{op: instMatch, arg: int32(i)})
			} else {
				prog = append(prog, inst)
			}
		}

		if alt >= 0 {
			prog[alt].arg = int32(len(prog) - (alt + 1))
		}
	}
	return prog
}

// NFA state operations, in service of building a DFA.
// (Again, see https://swtch.com/~rsc/regexp/regexp2.html for background.)

// An nfaState represents the state of the NFA - all possible instruction locations -
// after reading a particular input.
type nfaState []int32

// nfaStart returns the start state for the NFA executing prog.
func nfaStart(prog reProg) nfaState {
	var next nfaState
	next.add(prog, 0)
	next.trim(prog)
	return next
}

// add adds pc and other states reachable from it
// to the set of possible instruction locations in *s.
func (s *nfaState) add(prog reProg, pc int32) {
	// Avoid adding same state twice.
	// This scan is linear in the size of *s, which makes the overall
	// nfaStart / s.next operation technically quadratic in the size of *s,
	// but licenses are long texts of literal words, so the NFA states
	// end up being very small - there's not much ambiguity about
	// where we are in the list. If this ever showed up as expensive
	// on a profile, we could switch to a sparse set instead;
	// see https://research.swtch.com/sparse.
	for _, old := range *s {
		if old == pc {
			return
		}
	}

	*s = append(*s, pc)
	switch prog[pc].op {
	case instAlt:
		s.add(prog, pc+1)
		s.add(prog, pc+1+prog[pc].arg)
	case instJump:
		s.add(prog, pc+1+prog[pc].arg)
	case instCut:
		s.add(prog, pc+1)
	}
}

// trim canonicalizes *s by sorting it and removing unnecessary states.
// All that must be preserved between input tokens are the instruction
// locations that advance the input (instWord and instAny) or that
// report a match (instMatch).
func (s *nfaState) trim(prog reProg) {
	// Collect cut ranges and sort.
	var cuts []int32
	for _, pc := range *s {
		if prog[pc].op == instCut {
			cuts = append(cuts, pc+1+prog[pc].arg)
		}
	}
	sortInt32s(cuts)

	// Sort and save just the word, any, match instructions, applying cuts.
	sortInt32s(*s)
	save := (*s)[:0]
	for _, pc := range *s {
		switch prog[pc].op {
		case instWord, instAny, instMatch:
			for len(cuts) > 0 && pc > cuts[0]+1+prog[cuts[0]].arg {
				cuts = cuts[1:]
			}
			if len(cuts) > 0 && cuts[0] <= pc && pc <= cuts[0]+1+prog[cuts[0]].arg {
				break
			}
			save = append(save, pc)
		}
	}
	*s = save
}

// next returns the new state that results from reading word w in state s,
// and whether a match has been belatedly detected just before w.
func (s nfaState) next(prog reProg, w WordID) nfaState {
	var next nfaState
	for _, pc := range s {
		inst := &prog[pc]
		switch inst.op {
		case instAny:
			next.add(prog, pc+1)
		case instWord:
			if w == WordID(inst.arg) {
				next.add(prog, pc+1)
			}
		}
	}
	next.trim(prog)
	return next
}

// match returns the smallest match value of matches reached in state s,
// or -1 if there is no match.
func (s nfaState) match(prog reProg) int32 {
	match := int32(-1)
	for _, pc := range s {
		inst := &prog[pc]
		switch inst.op {
		case instMatch:
			if match == -1 || match > inst.arg {
				match = inst.arg
			}
		}
	}
	return match
}

// words returns the list of distinct words that can
// lead the NFA out of state s and into a new state.
// The returned list is sorted in increasing order.
// If the state can match any word (using instAny),
// the word ID AnyWord is first in the list.
func (s nfaState) words(prog reProg) []WordID {
	var words []WordID
	haveAny := false
State:
	for _, pc := range s {
		inst := &prog[pc]
		switch inst.op {
		case instAny:
			if !haveAny {
				haveAny = true
				words = append(words, AnyWord)
			}
		case instWord:
			// Dedup; linear scan but list should be small.
			// If this is too slow, the caller should pass in
			// a reusable map[WordID]bool.
			for _, w := range words {
				if w == WordID(inst.arg) {
					continue State
				}
			}
			words = append(words, WordID(inst.arg))
		}
	}

	sortWordIDs(words)
	return words
}

// appendEncoding appends a byte encoding of the state s to enc and returns the result.
func (s nfaState) appendEncoding(enc []byte) []byte {
	n := len(enc)
	for cap(enc) < n+len(s)*4 {
		enc = append(enc[:cap(enc)], 0)
	}
	enc = enc[:n+len(s)*4]
	for i, pc := range s {
		binary.BigEndian.PutUint32(enc[n+4*i:], uint32(pc))
	}
	return enc
}

// DFA building

// A reDFA is an encoded DFA over word IDs.
//
// The encoded DFA is a sequence of encoded DFA states, packed together.
// Each DFA state is identified by the index where it starts in the slice.
// The initial DFA state is at the start of the slice, index 0.
//
// Each DFA state records whether reaching that state counts as matching
// the input, which of multiple regexps matched, and then the transition
// table for the possible words that lead to new states. (If a word is found
// that is not in the current state's transition table, the DFA stops immediately
// with no match.)
//
// The encoding of this state information is:
//
//	-  a one-word header M | N<<1, where M is 0 for a non-match, 1 for a match,
//	   and N is the number of words in the table.
//	   This header is conveniently also the number of words that follow in the encoding.
//
//	- if M == 1, a one-word value V that is the match value to report,
//	  identifying which of a set of regexps has been matched.
//
//	- N two-word pairs W:NEXT indicating that if word W is seen, the DFA should
//	  move to the state at offset NEXT. The pairs are sorted by W. An entry for W == AnyWord
//	  is treated as matching any input word; an exact match later in the list takes priority.
//	  The list is sorted by W, so AnyWord is always first if present.
//
type reDFA []int32

// A dfaBuilder holds state for building a DFA from a reProg.
type dfaBuilder struct {
	prog reProg         // program being processed
	dfa  reDFA          // DFA so far
	have map[string]int // map from encoded NFA state to dfa array offset
	enc  []byte         // encoding buffer
}

// reCompileDFA compiles prog into a DFA.
func reCompileDFA(prog reProg) reDFA {
	b := &dfaBuilder{
		prog: prog,
		have: map[string]int{"": -1}, // dead (empty) NFA state encoding maps to DFA offset -1
	}
	b.add(nfaStart(prog))
	return b.dfa
}

// add returns the offset of the NFA state s in the DFA b.dfa,
// adding it to the end of the DFA if needed.
func (b *dfaBuilder) add(s nfaState) int32 {
	// If we've processed this state already, return its known position.
	b.enc = s.appendEncoding(b.enc[:0])
	pos, ok := b.have[string(b.enc)]
	if ok {
		return int32(pos)
	}

	// New state; append to current end of b.dfa.
	// Record position now, before filling in completely,
	// in case a transition cycle leads back to s.
	pos = len(b.dfa)
	b.have[string(b.enc)] = pos

	// Reserve room for this DFA state, so that new DFA states
	// can be appended to it as we fill this one in.
	// The total size of the state is 1+haveMatch+2*#words.
	words := s.words(b.prog)
	match := s.match(b.prog)
	size := 1 + 2*len(words)
	if match >= 0 {
		size++
	}
	for cap(b.dfa) < pos+size {
		b.dfa = append(b.dfa[:cap(b.dfa)], 0)
	}
	b.dfa = b.dfa[:pos+size]

	// Fill in state.
	off := pos
	b.dfa[off] = int32(size - 1) // header: M | N<<1 == (match>=0) + 2*len(words)
	off++
	if match >= 0 {
		b.dfa[off] = match // match value
		off++
	}
	for _, w := range words {
		next := s.next(b.prog, w)
		nextPos := b.add(next)
		b.dfa[off] = int32(w)
		b.dfa[off+1] = nextPos
		off += 2
	}

	return int32(pos)
}

// string returns a textual listing of the DFA.
// The dictionary d supplies the actual words for the listing.
func (dfa reDFA) string(d *Dict) string {
	var b strings.Builder
	for i := 0; i < len(dfa); {
		fmt.Fprintf(&b, "%d", i)
		hdr := dfa[i]
		i++
		if hdr&1 != 0 {
			fmt.Fprintf(&b, " m%d", dfa[i])
			i++
		}
		n := hdr >> 1
		for ; n > 0; n-- {
			w := WordID(dfa[i])
			next := dfa[i+1]
			i += 2
			var s string
			if w == AnyWord {
				s = "*"
			} else {
				s = d.Words()[w]
			}
			fmt.Fprintf(&b, " %s:%d", s, next)
		}
		fmt.Fprintf(&b, "\n")
	}
	return b.String()
}

// stateAt returns (partly) decoded information about the
// DFA state at the given offset.
// If the state is a matching state, stateAt returns match >= 0 specifies the match ID.
// If the state is not a matching state, stateAt returns match == -1.
// Either way, stateAt also returns the outgoing transition list
// interlaced in the delta slice. The caller can iterate over delta using:
//
//	for i := 0; i < len(delta); i += 2 {
//		dw, dnext := WordID(delta[i]), delta[i+1]
//		if currentWord == dw {
//			off = dnext
//		}
//	}
//
func (dfa reDFA) stateAt(off int32) (match int32, delta []int32) {
	hdr := dfa[off]
	off++
	match = -1
	if hdr&1 != 0 {
		match = dfa[off]
		off++
	}
	n := hdr >> 1
	return match, dfa[off : off+2*n]
}

// TraceDFA controls whether DFA execution prints debug tracing when stuck.
// If TraceDFA > 0 and the DFA has followed a path of at least TraceDFA symbols
// since the last matching state but hits a dead end, it prints out information
// about the dead end.
var TraceDFA int

// match looks for a match of DFA at the start of words,
// which are the result of dict.Split(text) or a subslice of it.
// match returns the match ID of the longest match, as well as
// the index in words immediately following the last matched word.
// If there is no match, match returns -1, 0.
func (dfa reDFA) match(dict *Dict, text string, words []Word) (match int32, end int) {
	match, end = -1, 0
	off := int32(0) // offset of current state in DFA
	dictWords := dict.Words()

	// No range loop here: misspellings can adjust i.
Words:
	for i := 0; i < len(words); i++ {
		word := words[i]
		w := word.ID

		// Find next state in DFA for w.
		m, delta := dfa.stateAt(off)
		if m >= 0 {
			match = m
			end = i
		}

		// Handle and remove AnyWord if present.
		// Simplifes the remaining loops.
		nextAny := int32(-1)
		if len(delta) > 0 && WordID(delta[0]) == AnyWord {
			nextAny = delta[1]
			delta = delta[2:]
		}

		for j := 0; j < len(delta); j += 2 {
			if WordID(delta[j]) == w {
				off = delta[j+1]
				continue Words
			}
		}

		// No exact word match.
		// Try context-sensitive spell check.
		// We know the words that could usefully come next.
		// Do any of those look enough like the word we have?
		// TODO: Should the misspellings reduce the match percent?

		// have is the current word; have2 is the word after that.
		have := toFold(text[words[i].Lo:words[i].Hi])
		have2 := ""
		if i+1 < len(words) {
			have2 = toFold(text[words[i+1].Lo:words[i+1].Hi])
		}

		for j := 0; j < len(delta); j += 2 {
			dw, dnext := WordID(delta[j]), delta[j+1]
			want := dictWords[dw]

			// Can we spell want by joining have and have2?
			// This can happen with hyphenated line breaks.
			if canMisspellJoin(want, have, have2) {
				off = dnext
				i++ // for have; loop will i++ again for have2
				continue Words
			}

			// misspell split
			// Or can have be split into two words such that
			// the pair is something we'd expect to see right now?
			if len(have) > len(want) && have[:len(want)] == want {
				// have[:len(want)] matches want.
				// Look to see if have[len(want):] can be the word after want.
				rest := have[len(want):]
				m2, delta2 := dfa.stateAt(dnext)
				next2 := int32(-1)
				for j2 := 0; j2 < len(delta2); j2 += 2 {
					dw2, dnext2 := WordID(delta2[j2]), delta2[j2+1]
					if dw2 == AnyWord || dictWords[dw2] == rest {
						next2 = dnext2
					}
				}
				if next2 >= 0 {
					// Successfully split have into two words
					// to drive the DFA forward two steps.
					if m2 >= 0 {
						match = m2
						end = i
					}
					off = next2
					continue Words
				}
			}

			// Can we misspell want as have?
			if canMisspell(want, have) {
				off = dnext
				continue Words
			}
		}

		if nextAny == -1 {
			// Stuck - match is about to abort.
			// For help debugging why a match doesn't work,
			// if we seemed to be in the middle of a promising match
			// (at least 5 words that moved the DFA forward since
			// the last time we saw a matching state),
			// print information about it.
			if TraceDFA > 0 && i-end >= TraceDFA {
				start := i - 10
				if start < 0 {
					start = 0
				}
				print("DFA mismatch at «",
					text[words[start].Lo:words[i].Lo], "|",
					text[words[i].Lo:words[i].Hi], "»\n")
				print("Possible next words:\n")
				for j := 0; j < len(delta); j += 2 {
					print("\t", dictWords[delta[j]], "\n")
				}
			}

			// Return best match we found.
			return match, end
		}
		off = nextAny
	}

	if m, _ := dfa.stateAt(off); m >= 0 {
		match = m
		end = len(words)
	}
	if i := len(words); TraceDFA > 0 && i-end >= TraceDFA {
		start := i - 10
		if start < 0 {
			start = 0
		}
		println("DFA ran out of input at «", text[words[i-10].Lo:], "|", "EOF", "»\n")
	}
	return match, end
}

func sortInt32s(x []int32) {
	sort.Slice(x, func(i, j int) bool {
		return x[i] < x[j]
	})
}

func sortWordIDs(x []WordID) {
	sort.Slice(x, func(i, j int) bool {
		return x[i] < x[j]
	})
}

// canMisspell reports whether want can be misspelled as have.
// Both words have been converted to lowercase already
// (want by the Dict, have by the caller).
func canMisspell(want, have string) bool {
	// Allow single-letter replacement, insertion, or deletion.
	if len(want)-1 <= len(have) && len(have) <= len(want)+1 && (len(have) >= 4 || len(want) >= 4) {
		// Count common bytes at start and end of strings.
		i := 0
		for i < len(have) && i < len(want) && want[i] == have[i] {
			i++
		}
		j := 0
		for j < len(have) && j < len(want) && want[len(want)-1-j] == have[len(have)-1-j] {
			j++
		}
		// Total common bytes must be at least all but one of both strings.
		if i+j >= len(want)-1 && i+j >= len(have)-1 {
			return true
		}
	}

	// We have to canonicalize "(C)" and "(c)" to "copyright",
	// but then that produces an unfortunate disconnect between
	// list bullets "c.", "c)", and "(c)".
	// The first two are both "c", but the third is "copyright".
	// We can't canonicalize all "c" to "copyright",
	// or else we'll see spurious "copyright" words in path names like "file.c",
	// which might change the boundaries of an overall copyright notice match.
	// Instead, we correct the ambiguity by treating "c" and "copyright"
	// the same during spell check. (Spell checks only apply when a match
	// has already started, so they don't affect the match boundaries.)
	//
	// The want string has been canonicalized, so it must be "c" or "copyright" (not "©"),
	// but the have string has only been folded, so it can be any of the three.
	if (want == "c" || want == "copyright") && (have == "c" || have == "copyright" || have == "©") {
		return true
	}

	return false
}

// canMisspellJoin reports whether want can be misspelled as the word pair have1, have2.
// All three words have been converted to lowercase already
// (want by the Dict, have1, have2 by the caller).
func canMisspellJoin(want, have1, have2 string) bool {
	// want == have1+have2 but without allocating the concatenation
	return len(want) == len(have1)+len(have2) &&
		want[:len(have1)] == have1 &&
		want[len(have1):] == have2
}
