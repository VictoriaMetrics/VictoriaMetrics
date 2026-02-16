package graphemes

// ansiEscapeLength returns the byte length of a valid ANSI escape sequence at the
// start of data, or 0 if none. Input is UTF-8; only 7-bit ESC sequences are
// recognized (C1 0x80–0x9F can be UTF-8 continuation bytes).
//
// Recognized forms (ECMA-48 / ISO 6429):
//   - CSI: ESC [ then parameter bytes (0x30–0x3F), intermediate (0x20–0x2F), final (0x40–0x7E)
//   - OSC: ESC ] then payload until ST (ESC \) or BEL (0x07)
//   - DCS, SOS, PM, APC: ESC P / X / ^ / _ then payload until ST (ESC \)
//   - Two-byte: ESC + Fe (0x40–0x5F excluding above), or Fp (0x30–0x3F), or nF (0x20–0x2F then final)
func ansiEscapeLength[T ~string | ~[]byte](data T) int {
	n := len(data)
	if n < 2 {
		return 0
	}
	if data[0] != esc {
		return 0
	}

	b1 := data[1]
	switch b1 {
	case '[': // CSI
		body := csiLength(data[2:])
		if body == 0 {
			return 0
		}
		return 2 + body
	case ']': // OSC – allows BEL or ST as terminator
		body := oscLength(data[2:])
		if body == 0 {
			return 0
		}
		return 2 + body
	case 'P', 'X', '^', '_': // DCS, SOS, PM, APC – require ST (ESC \) only
		body := stSequenceLength(data[2:])
		if body == 0 {
			return 0
		}
		return 2 + body
	}
	if b1 >= 0x40 && b1 <= 0x5F {
		// Fe (C1) two-byte; [ ] P X ^ _ handled above
		return 2
	}
	if b1 >= 0x30 && b1 <= 0x3F {
		// Fp (private) two-byte
		return 2
	}
	if b1 >= 0x20 && b1 <= 0x2F {
		// nF: intermediates then one final (0x30–0x7E)
		i := 2
		for i < n && data[i] >= 0x20 && data[i] <= 0x2F {
			i++
		}
		if i < n && data[i] >= 0x30 && data[i] <= 0x7E {
			return i + 1
		}
		return 0
	}
	return 0
}

// csiLength returns the length of the CSI body (param/intermediate/final bytes).
// data is the slice after "ESC [".
// Per ECMA-48, the CSI body has the form:
//
//	parameters (0x30–0x3F)*, intermediates (0x20–0x2F)*, final (0x40–0x7E)
//
// Once an intermediate byte is seen, subsequent parameter bytes are invalid.
func csiLength[T ~string | ~[]byte](data T) int {
	seenIntermediate := false
	for i := 0; i < len(data); i++ {
		b := data[i]
		if b >= 0x30 && b <= 0x3F {
			if seenIntermediate {
				return 0
			}
			continue
		}
		if b >= 0x20 && b <= 0x2F {
			seenIntermediate = true
			continue
		}
		if b >= 0x40 && b <= 0x7E {
			return i + 1
		}
		return 0
	}
	return 0
}

// oscLength returns the length of the OSC body up to and including
// the terminator. OSC accepts either BEL (0x07) or ST (ESC \) per
// widespread terminal convention. data is the slice after "ESC ]".
func oscLength[T ~string | ~[]byte](data T) int {
	for i := 0; i < len(data); i++ {
		b := data[i]
		if b == bel {
			return i + 1
		}
		if b == esc && i+1 < len(data) && data[i+1] == '\\' {
			return i + 2
		}
	}
	return 0
}

// stSequenceLength returns the length of a control-string body up to and
// including the ST (ESC \) terminator. Used for DCS, SOS, PM, and APC, which
// per ECMA-48 require ST and do not accept BEL. data is the slice after "ESC x".
func stSequenceLength[T ~string | ~[]byte](data T) int {
	for i := 0; i < len(data); i++ {
		if data[i] == esc && i+1 < len(data) && data[i+1] == '\\' {
			return i + 2
		}
	}
	return 0
}
