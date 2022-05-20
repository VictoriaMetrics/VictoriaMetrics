package matchr

type runestring []rune

// A safe way to index a runestring. It will return a null rune if you try
// to index outside of the bounds of the runestring.
func (r *runestring) SafeAt(pos int) rune {
	if pos < 0 || pos >= len(*r) {
		return 0
	} else {
		return (*r)[pos]
	}
}

// A safe way to obtain a substring of a runestring. It will return a null
// string ("") if you index somewhere outside its bounds.
func (r *runestring) SafeSubstr(pos int, length int) string {
	if pos < 0 || pos > len(*r) || (pos+length) > len(*r) {
		return ""
	} else {
		return string((*r)[pos : pos+length])
	}
}

// Delete characters at positions pos. It will do nothing if you provide
// an index outside the bounds of the runestring.
func (r *runestring) Del(pos ...int) {
	for _, i := range pos {
		if i >= 0 && i <= len(*r) {
			*r = append((*r)[:i], (*r)[i+1:]...)
		}
	}
}

// A helper to determine if any substrings exist within the given runestring.
func (r *runestring) Contains(start int, length int, criteria ...string) bool {
	substring := r.SafeSubstr(start, length)
	for _, c := range criteria {
		if substring == c {
			return true
		}
	}
	return false
}
