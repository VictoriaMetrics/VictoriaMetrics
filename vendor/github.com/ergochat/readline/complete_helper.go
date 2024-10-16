package readline

import (
	"bytes"
	"strings"

	"github.com/ergochat/readline/internal/runes"
)

// PrefixCompleter implements AutoCompleter via a recursive tree.
type PrefixCompleter struct {
	// Name is the name of a command, subcommand, or argument eligible for completion.
	Name string
	// Callback is optional; if defined, it takes the current line and returns
	// a list of possible completions associated with the current node (i.e.
	// in place of Name).
	Callback func(string) []string
	// Children is a list of possible completions that can follow the current node.
	Children []*PrefixCompleter

	nameRunes []rune // just a cache
}

var _ AutoCompleter = (*PrefixCompleter)(nil)

func (p *PrefixCompleter) Tree(prefix string) string {
	buf := bytes.NewBuffer(nil)
	p.print(prefix, 0, buf)
	return buf.String()
}

func prefixPrint(p *PrefixCompleter, prefix string, level int, buf *bytes.Buffer) {
	if strings.TrimSpace(p.Name) != "" {
		buf.WriteString(prefix)
		if level > 0 {
			buf.WriteString("├")
			buf.WriteString(strings.Repeat("─", (level*4)-2))
			buf.WriteString(" ")
		}
		buf.WriteString(p.Name)
		buf.WriteByte('\n')
		level++
	}
	for _, ch := range p.Children {
		ch.print(prefix, level, buf)
	}
}

func (p *PrefixCompleter) print(prefix string, level int, buf *bytes.Buffer) {
	prefixPrint(p, prefix, level, buf)
}

func (p *PrefixCompleter) getName() []rune {
	if p.nameRunes == nil {
		if p.Name != "" {
			p.nameRunes = []rune(p.Name)
		} else {
			p.nameRunes = make([]rune, 0)
		}
	}
	return p.nameRunes
}

func (p *PrefixCompleter) getDynamicNames(line []rune) [][]rune {
	var result [][]rune
	for _, name := range p.Callback(string(line)) {
		nameRunes := []rune(name)
		nameRunes = append(nameRunes, ' ')
		result = append(result, nameRunes)
	}
	return result
}

func (p *PrefixCompleter) SetChildren(children []*PrefixCompleter) {
	p.Children = children
}

func NewPrefixCompleter(pc ...*PrefixCompleter) *PrefixCompleter {
	return PcItem("", pc...)
}

func PcItem(name string, pc ...*PrefixCompleter) *PrefixCompleter {
	name += " "
	result := &PrefixCompleter{
		Name:     name,
		Children: pc,
	}
	result.getName() // initialize nameRunes member
	return result
}

func PcItemDynamic(callback func(string) []string, pc ...*PrefixCompleter) *PrefixCompleter {
	return &PrefixCompleter{
		Callback: callback,
		Children: pc,
	}
}

func (p *PrefixCompleter) Do(line []rune, pos int) (newLine [][]rune, offset int) {
	return doInternal(p, line, pos, line)
}

func doInternal(p *PrefixCompleter, line []rune, pos int, origLine []rune) (newLine [][]rune, offset int) {
	line = runes.TrimSpaceLeft(line[:pos])
	goNext := false
	var lineCompleter *PrefixCompleter
	for _, child := range p.Children {
		var childNames [][]rune
		if child.Callback != nil {
			childNames = child.getDynamicNames(origLine)
		} else {
			childNames = make([][]rune, 1)
			childNames[0] = child.getName()
		}

		for _, childName := range childNames {
			if len(line) >= len(childName) {
				if runes.HasPrefix(line, childName) {
					if len(line) == len(childName) {
						newLine = append(newLine, []rune{' '})
					} else {
						newLine = append(newLine, childName)
					}
					offset = len(childName)
					lineCompleter = child
					goNext = true
				}
			} else {
				if runes.HasPrefix(childName, line) {
					newLine = append(newLine, childName[len(line):])
					offset = len(line)
					lineCompleter = child
				}
			}
		}
	}

	if len(newLine) != 1 {
		return
	}

	tmpLine := make([]rune, 0, len(line))
	for i := offset; i < len(line); i++ {
		if line[i] == ' ' {
			continue
		}

		tmpLine = append(tmpLine, line[i:]...)
		return doInternal(lineCompleter, tmpLine, len(tmpLine), origLine)
	}

	if goNext {
		return doInternal(lineCompleter, nil, 0, origLine)
	}
	return
}
