// Package mimeword implements encoding and decoding of RFC 2047 "encoded words",
// the mechanism that lets non-ASCII text appear in MIME headers such as Subject
// and From.
//
// The package deliberately supports the two encodings that appear in practice:
// \"B\" (base64) and \"Q\" (quoted-printable, with '_' standing for space).
// Unknown encodings are reported as errors rather than silently mangled, since a
// caller that cannot decode a header should know that rather than emit garbage.
package mimeword

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// Encoding selects the transfer encoding used by Encode.
type Encoding int

const (
	// EncodingB uses base64. It is the safe default: the payload is opaque and
	// never contains characters that could be mistaken for header syntax.
	EncodingB Encoding = iota
	// EncodingQ uses quoted-printable with '_' for space. It is more readable
	// for short ASCII-heavy text but needs escaping for '?', '_' and '='.
	EncodingQ
)

// ErrInvalid reports malformed encoded-word syntax.
var ErrInvalid = errors.New("mimeword: invalid encoded word")

// ErrUnsupportedEncoding reports an encoded word whose encoding letter is not
// B or Q. We do not guess at other encodings.
var ErrUnsupportedEncoding = errors.New("mimeword: unsupported encoding")

// ErrInvalidUTF8 reports a payload that decoded to bytes that are not valid
// UTF-8. RFC 2047 encoded words carry text, and Go strings are UTF-8 by
// convention, so we reject rather than pass through mojibake.
var ErrInvalidUTF8 = errors.New("mimeword: payload is not valid UTF-8")

// Encode returns s as a single RFC 2047 encoded word of the form
// =?UTF-8?B?...?= or =?UTF-8?Q?...?=.
//
// The charset is always UTF-8: the input is a Go string and therefore already
// UTF-8, and rewriting it to another charset would require a dependency we do
// not want. The result is guaranteed to contain no spaces, so it can be placed
// in a header without introducing an unintended fold.
func Encode(s string, enc Encoding) string {
	switch enc {
	case EncodingQ:
		return "=?UTF-8?Q?" + encodeQ(s) + "?="
	default:
		return "=?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte(s)) + "?="
	}
}

// encodeQ implements the Q encoding for the UTF-8 charset: printable ASCII
// except '?', '_', '=' and space passes through; space becomes '_'; everything
// else becomes =XX with uppercase hex.
func encodeQ(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == ' ':
			b.WriteByte('_')
		case c >= 33 && c <= 126 && c != '?' && c != '_' && c != '=':
			b.WriteByte(c)
		default:
			const hex = "0123456789ABCDEF"
			b.WriteByte('=')
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&0x0F])
		}
	}
	return b.String()
}

// Decode decodes a single encoded word. The input must be exactly one word,
// i.e. it must start with "=?" and end with "?=". Leading and trailing
// whitespace is not trimmed; surrounding text is not handled here, see
// DecodeHeader for that.
//
// Only the UTF-8 charset is accepted. Other charsets are rejected with
// ErrUnsupportedEncoding, because converting them would need tables we do not
// carry.
func Decode(word string) (string, error) {
	if !strings.HasPrefix(word, "=?") || !strings.HasSuffix(word, "?=") {
		return "", ErrInvalid
	}
	inner := word[2 : len(word)-2]
	// Split into charset, encoding, payload. The payload may itself contain
	// '?', so SplitN with 3 is required.
	parts := strings.SplitN(inner, "?", 3)
	if len(parts) != 3 {
		return "", ErrInvalid
	}
	charset, enc, payload := parts[0], parts[1], parts[2]
	if !strings.EqualFold(charset, "UTF-8") {
		return "", ErrUnsupportedEncoding
	}
	if len(enc) != 1 {
		return "", ErrUnsupportedEncoding
	}
	var raw []byte
	var err error
	switch enc[0] {
	case 'B', 'b':
		raw, err = base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return "", fmt.Errorf("%w: %v", ErrInvalid, err)
		}
	case 'Q', 'q':
		raw, err = decodeQ(payload)
		if err != nil {
			return "", err
		}
	default:
		return "", ErrUnsupportedEncoding
	}
	if !utf8.Valid(raw) {
		return "", ErrInvalidUTF8
	}
	return string(raw), nil
}

// decodeQ reverses encodeQ. It is strict: a '=' must be followed by two hex
// digits, and a byte that is not valid UTF-8 is an error rather than a
// replacement character.
func decodeQ(s string) ([]byte, error) {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '_':
			out = append(out, ' ')
		case c == '=':
			if i+2 >= len(s) {
				return nil, ErrInvalid
			}
			hi, ok1 := fromHex(s[i+1])
			lo, ok2 := fromHex(s[i+2])
			if !ok1 || !ok2 {
				return nil, ErrInvalid
			}
			out = append(out, hi<<4|lo)
			i += 2
		default:
			out = append(out, c)
		}
	}
	return out, nil
}

func fromHex(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

// DecodeHeader decodes every encoded word in a header value and leaves other
// text untouched. Whitespace between two adjacent encoded words is removed, as
// RFC 2047 requires; whitespace elsewhere is preserved.
//
// This is the function most callers want for a Subject or From line, because
// real headers mix encoded words with plain text.
func DecodeHeader(s string) (string, error) {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		start := strings.Index(s[i:], "=?")
		if start < 0 {
			b.WriteString(s[i:])
			break
		}
		start += i
		end := strings.Index(s[start:], "?=")
		if end < 0 {
			// No terminator: the rest is literal text.
			b.WriteString(s[i:])
			break
		}
		end += start + 2
		word := s[start:end]
		decoded, err := Decode(word)
		if err != nil {
			// Not a valid encoded word; emit it verbatim and keep going.
			b.WriteString(s[i:end])
			i = end
			continue
		}
		// Drop whitespace that sits between two encoded words.
		prefix := s[i:start]
		if strings.TrimSpace(prefix) != "" || b.Len() == 0 {
			b.WriteString(prefix)
		}
		b.WriteString(decoded)
		i = end
	}
	return b.String(), nil
}
