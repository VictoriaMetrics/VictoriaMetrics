// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// License regexp syntax and parsing
// See LRE doc comment in regexp.go for syntax.

package match

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// A SyntaxError reports a syntax error during parsing.
type SyntaxError struct {
	File    string
	Offset  int
	Context string
	Err     string
}

func (e *SyntaxError) Error() string {
	var text string
	if e.File != "" {
		text = fmt.Sprintf("%s:#%d: syntax error", e.File, e.Offset)
	} else {
		text = fmt.Sprintf("syntax error at offset %d", e.Offset)
	}
	if e.Context != "" {
		text += " near `" + e.Context + "`"
	}
	text += ": " + e.Err
	return text
}

// reSyntaxError returns a *SyntaxError with context.
func reSyntaxError(s string, i int, err error) error {
	context := s[:i]
	j := i - 10
	for j >= 0 && s[j] != ' ' {
		j--
	}
	if j >= 0 && i-j < 20 {
		context = s[j+1 : i]
	} else {
		if j < 0 {
			j = 0
		}
		context = s[j:i]
	}
	return &SyntaxError{
		Offset:  i,
		Context: context,
		Err:     err.Error(),
	}
}

// A reSyntax is a regexp syntax tree.
type reSyntax struct {
	op  reOp        // opcode
	sub []*reSyntax // subexpressions (opConcat, opAlternate, opWild, opQuest)
	w   []WordID    // words (opWords)
	n   int32       // wildcard count (opWild)
}

// A reOp is the opcode for a regexp syntax tree node.
type reOp int

const (
	opNone reOp = 1 + iota
	opEmpty
	opWords
	opConcat
	opAlternate
	opWild
	opQuest

	// pseudo-ops during parsing
	opPseudo
	opLeftParen
	opVerticalBar
)

// string returns a text form for the regexp syntax.
// The dictionary d supplies the word literals.
func (re *reSyntax) string(d *Dict) string {
	var b bytes.Buffer
	rePrint(&b, re, d)
	return strings.Trim(b.String(), "\n")
}

// nl guarantees b ends with a complete, non-empty line with no trailing spaces
// or has no lines at all.
func nl(b *bytes.Buffer) {
	buf := b.Bytes()
	if len(buf) == 0 || buf[len(buf)-1] == '\n' {
		return
	}
	i := len(buf)
	for i > 0 && buf[i-1] == ' ' {
		i--
	}
	if i < len(buf) {
		b.Truncate(i)
	}
	b.WriteByte('\n')
}

// rePrint prints re to b, using d for words.
func rePrint(b *bytes.Buffer, re *reSyntax, d *Dict) {
	switch re.op {
	case opEmpty:
		b.WriteString("(( ))")
	case opConcat:
		if len(re.sub) == 0 {
			b.WriteString("«empty concat»")
		}
		for i, sub := range re.sub {
			if i > 0 && b.Len() > 0 && b.Bytes()[b.Len()-1] != '\n' {
				b.WriteString(" ")
			}
			rePrint(b, sub, d)
		}

	case opAlternate:
		nl(b)
		b.WriteString("((")
		for i, sub := range re.sub {
			if i > 0 {
				b.WriteString(" || ")
			}
			rePrint(b, sub, d)
		}
		b.WriteString("))\n")

	case opWild:
		fmt.Fprintf(b, "__%d__", re.n)

	case opQuest:
		sub := re.sub[0]
		nl(b)
		if sub.op == opAlternate {
			rePrint(b, sub, d)
			b.Truncate(b.Len() - 1) // strip \n
		} else {
			b.WriteString("((")
			rePrint(b, sub, d)
			b.WriteString("))")
		}
		b.WriteString("??\n")

	case opWords:
		if len(re.w) == 0 {
			b.WriteString("«empty opWords»")
		}
		for i, w := range re.w {
			if i > 0 && b.Len() > 0 && b.Bytes()[b.Len()-1] != '\n' {
				b.WriteString(" ")
			}
			s := d.Words()[w]
			if s == "" {
				b.WriteString("''")
			} else {
				b.WriteString(d.Words()[w])
			}
		}
	}
}

// A reParser is the regexp parser state.
type reParser struct {
	dict  *Dict
	stack []*reSyntax
}

// reParse parses a license regexp s
// and returns a reSyntax parse tree.
// reParse adds words to the dictionary d,
// so it is not safe to call reParse from concurrent goroutines
// using the same dictionary.
// If strict is false, the rules about operators at the start or end of line are ignored,
// to make trivial test expressions easier to write.
func reParse(d *Dict, s string, strict bool) (*reSyntax, error) {
	var p reParser
	p.dict = d

	start := 0
	parens := 0
	i := 0
	for i < len(s) {
		switch {
		case strings.HasPrefix(s[i:], "(("):
			if strict && !atBOL(s, i) {
				return nil, reSyntaxError(s, i, fmt.Errorf("(( not at beginning of line"))
			}
			p.words(s[start:i], "((")
			p.push(&reSyntax{op: opLeftParen})
			i += 2
			start = i
			parens++

		case strings.HasPrefix(s[i:], "||"):
			if strict && parens == 0 {
				return nil, reSyntaxError(s, i, fmt.Errorf("|| outside (( ))"))
			}
			p.words(s[start:i], "||")
			if err := p.verticalBar(); err != nil {
				return nil, reSyntaxError(s, i, err)
			}
			i += 2
			start = i

		case strings.HasPrefix(s[i:], "))"):
			// )) must be followed by ?? or end line
			if strict {
				j := i + 2
				for j < len(s) && (s[j] == ' ' || s[j] == '\t') {
					j++
				}
				if j < len(s) && s[j] != '\n' && (j+1 >= len(s) || s[j] != '?' || s[j+1] != '?') {
					return nil, reSyntaxError(s, i, fmt.Errorf(")) not at end of line"))
				}
			}

			p.words(s[start:i], "))")
			if err := p.rightParen(); err != nil {
				return nil, reSyntaxError(s, i, err)
			}
			i += 2
			start = i
			parens--

		case strings.HasPrefix(s[i:], "??"):
			// ?? must be preceded by )) on same line and must end the line.
			if strict {
				j := i
				for j > 0 && (s[j-1] == ' ' || s[j-1] == '\t') {
					j--
				}
				if j < 2 || s[j-1] != ')' || s[j-2] != ')' {
					return nil, reSyntaxError(s, i, fmt.Errorf("?? not preceded by ))"))
				}
			}
			if strict && !atEOL(s, i+2) {
				return nil, reSyntaxError(s, i, fmt.Errorf("?? not at end of line"))
			}

			p.words(s[start:i], "??")
			if err := p.quest(); err != nil {
				return nil, reSyntaxError(s, i, err)
			}
			i += 2
			start = i

		case strings.HasPrefix(s[i:], "__"):
			j := i + 2
			for j < len(s) && '0' <= s[j] && s[j] <= '9' {
				j++
			}
			if j == i+2 {
				i++
				continue
			}
			if !strings.HasPrefix(s[j:], "__") {
				i++
				continue
			}
			n, err := strconv.Atoi(s[i+2 : j])
			if err != nil {
				return nil, reSyntaxError(s, i, errors.New("invalid wildcard count "+s[i:j+2]))
			}
			p.words(s[start:i], "__")
			p.push(&reSyntax{op: opWild, n: int32(n)})
			i = j + 2
			start = i

		case strings.HasPrefix(s[i:], "//**"):
			j := strings.Index(s[i+4:], "**//")
			if j < 0 {
				return nil, reSyntaxError(s, i, errors.New("opening //** without closing **//"))
			}
			p.words(s[start:i], "//** **//")
			i += 4 + j + 4
			start = i

		default:
			i++
		}
	}

	p.words(s[start:], "")
	p.concat()
	if p.swapVerticalBar() {
		// pop vertical bar
		p.stack = p.stack[:len(p.stack)-1]
	}
	p.alternate()

	n := len(p.stack)
	if n != 1 {
		return nil, reSyntaxError(s, len(s), fmt.Errorf("missing )) at end"))
	}
	return p.stack[0], nil
}

// atBOL reports whether i is at the beginning of a line (ignoring spaces) in s.
func atBOL(s string, i int) bool {
	for i > 0 && (s[i-1] == ' ' || s[i-1] == '\t') {
		i--
	}
	return i == 0 || s[i-1] == '\n'
}

// atEOL reports whether i is at the end of a line (ignoring spaces) in s.
func atEOL(s string, i int) bool {
	for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\r') {
		i++
	}
	return i >= len(s) || s[i] == '\n'
}

// push pushes the regexp re onto the parse stack and returns the regexp.
func (p *reParser) push(re *reSyntax) *reSyntax {
	p.stack = append(p.stack, re)
	return re
}

// words handles a block of words in the input.
func (p *reParser) words(text, next string) {
	words := p.dict.InsertSplit(text)
	if len(words) == 0 {
		return
	}

	// If the next operator is ??, we need to keep the last word
	// separate, so that the ?? will only apply to that word.
	// There are no other operators that grab the last word.
	var last Word
	if next == "??" {
		words, last = words[:len(words)-1], words[len(words)-1]
	}

	// Add the words (with last possibly removed) into an opWords.
	// If there's one atop the stack, use it. Otherwise, add one.
	if len(words) > 0 {
		var re *reSyntax
		if len(p.stack) > 0 && p.stack[len(p.stack)-1].op == opWords {
			re = p.stack[len(p.stack)-1]
		} else {
			re = p.push(&reSyntax{op: opWords})
		}
		for _, w := range words {
			re.w = append(re.w, w.ID)
		}
	}

	// Add the last word if needed.
	if next == "??" {
		p.stack = append(p.stack, &reSyntax{op: opWords, w: []WordID{last.ID}})
	}
}

// verticalBar handles a || in the input.
func (p *reParser) verticalBar() error {
	p.concat()

	// The concatenation we just parsed is on top of the stack.
	// If it sits above an opVerticalBar, swap it below
	// (things below an opVerticalBar become an alternation).
	// Otherwise, push a new vertical bar.
	if !p.swapVerticalBar() {
		p.push(&reSyntax{op: opVerticalBar})
	}

	return nil
}

// If the top of the stack is an element followed by an opVerticalBar
// swapVerticalBar swaps the two and returns true.
// Otherwise it returns false.
func (p *reParser) swapVerticalBar() bool {
	n := len(p.stack)
	if n >= 2 {
		re1 := p.stack[n-1]
		re2 := p.stack[n-2]
		if re2.op == opVerticalBar {
			p.stack[n-2] = re1
			p.stack[n-1] = re2
			return true
		}
	}
	return false
}

// rightParen handles a )) in the input.
func (p *reParser) rightParen() error {
	p.concat()
	if p.swapVerticalBar() {
		// pop vertical bar
		p.stack = p.stack[:len(p.stack)-1]
	}
	p.alternate()

	n := len(p.stack)
	if n < 2 {
		return fmt.Errorf("unexpected ))")
	}
	re1 := p.stack[n-1]
	re2 := p.stack[n-2]
	p.stack = p.stack[:n-2]
	if re2.op != opLeftParen {
		return fmt.Errorf("unexpected ))")
	}

	p.push(re1)
	return nil
}

// quest replaces the top stack element with itself made optional.
func (p *reParser) quest() error {
	n := len(p.stack)
	if n == 0 {
		return fmt.Errorf("missing argument to ??")
	}
	sub := p.stack[n-1]
	if sub.op >= opPseudo {
		return fmt.Errorf("missing argument to ??")
	}
	if sub.op == opQuest {
		// Repeated ?? don't accomplish anything new.
		return nil
	}

	p.stack[n-1] = &reSyntax{op: opQuest, sub: []*reSyntax{sub}}
	return nil
}

// concat replaces the top of the stack (above the topmost '||' or '((') with its concatenation.
func (p *reParser) concat() *reSyntax {
	// Scan down to find pseudo-operator || or ((.
	i := len(p.stack)
	for i > 0 && p.stack[i-1].op < opPseudo {
		i--
	}
	subs := p.stack[i:]
	p.stack = p.stack[:i]

	// Empty concatenation is special case.
	if len(subs) == 0 {
		return p.push(&reSyntax{op: opEmpty})
	}

	return p.push(p.collapse(opConcat, subs))
}

// alternate replaces the top of the stack (above the topmost '((') with its alternation.
func (p *reParser) alternate() *reSyntax {
	// Scan down to find pseudo-operator ((.
	// There are no || above ((.
	i := len(p.stack)
	for i > 0 && p.stack[i-1].op < opPseudo {
		i--
	}
	subs := p.stack[i:]
	p.stack = p.stack[:i]

	return p.push(p.collapse(opAlternate, subs))
}

// collapse returns the result of applying op to sub.
// If sub contains op nodes, they all get hoisted up
// so that there is never a concat of a concat or an
// alternate of an alternate.
func (p *reParser) collapse(op reOp, subs []*reSyntax) *reSyntax {
	if len(subs) == 1 {
		return subs[0]
	}
	re := &reSyntax{op: op}
	for _, sub := range subs {
		if sub.op == op {
			re.sub = append(re.sub, sub.sub...)
		} else {
			re.sub = append(re.sub, sub)
		}
	}
	return re
}

// leadingPhrases returns the set of possible initial phrases
// in any match of the given re syntax.
func (re *reSyntax) leadingPhrases() []phrase {
	switch re.op {
	default:
		panic("bad op in phrases")

	case opWild:
		return []phrase{{BadWord, BadWord}, {AnyWord, BadWord}, {AnyWord, AnyWord}}

	case opEmpty:
		return []phrase{{BadWord, BadWord}}

	case opWords:
		w := re.w
		var p phrase
		if len(w) == 0 {
			p = phrase{BadWord, BadWord}
		} else if len(w) == 1 {
			p = phrase{w[0], BadWord}
		} else {
			p = phrase{w[0], w[1]}
		}
		return []phrase{p}

	case opQuest:
		list := re.sub[0].leadingPhrases()
		for _, l := range list {
			if l[0] == BadWord {
				return list
			}
		}
		list = append(list, phrase{BadWord, BadWord})
		return list

	case opAlternate:
		var list []phrase
		have := make(map[phrase]bool)
		for _, sub := range re.sub {
			for _, p := range sub.leadingPhrases() {
				if !have[p] {
					have[p] = true
					list = append(list, p)
				}
			}
		}
		return list

	case opConcat:
		xs := []phrase{{BadWord, BadWord}}
		for _, sub := range re.sub {
			ok := true
			for _, x := range xs {
				if x[1] == BadWord {
					ok = false
				}
			}
			if ok {
				break
			}
			ys := sub.leadingPhrases()
			have := make(map[phrase]bool)
			var xys []phrase
			for _, x := range xs {
				if x[1] != BadWord {
					if !have[x] {
						have[x] = true
						xys = append(xys, x)
					}
					continue
				}
				for _, y := range ys {
					var xy phrase
					if x[0] == BadWord {
						xy = y
					} else {
						xy = phrase{x[0], y[0]}
					}
					if !have[xy] {
						have[xy] = true
						xys = append(xys, xy)
					}
				}
			}
			xs = xys
		}
		return xs
	}
}
