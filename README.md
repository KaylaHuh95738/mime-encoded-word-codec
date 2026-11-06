# mime-encoded-word-codec

Encodes and decodes RFC 2047 encoded words for non-ASCII text in MIME headers.

```go
package main

import (
	"fmt"

	mimeword "mime-encoded-word-codec"
)

func main() {
	word := mimeword.Encode("héllo wörld", mimeword.EncodingQ)
	fmt.Println(word)
	// =?UTF-8?Q?h=C3=A9llo_w=C3=B6rld?=

	text, err := mimeword.DecodeHeader("Re: " + word)
	if err != nil {
		panic(err)
	}
	fmt.Println(text)
	// Re: héllo wörld
}
```

## Why this exists

MIME headers are ASCII by specification, so any header text outside ASCII has to be
wrapped in an "encoded word" of the form `=?charset?encoding?payload?=`. Parsing
those back out is fiddly: the payload may contain `?`, whitespace between two
adjacent encoded words is not real whitespace, and a header can mix encoded words
with ordinary text. This package handles exactly that, with no dependencies.

The trade-off is charset support. Only UTF-8 is accepted. Go strings are UTF-8, so
encoding is lossless, and decoding an encoded word in any other charset fails with
`ErrUnsupportedEncoding` instead of being silently garbled. If you need to read
legacy `ISO-8859-1` headers you will have to convert them yourself before calling
`Decode`.

## API

- `Encode(s string, enc Encoding) string` — returns one encoded word. `EncodingB`
  uses base64, `EncodingQ` uses quoted-printable with `_` for space.
- `Decode(word string) (string, error)` — decodes exactly one encoded word.
- `DecodeHeader(s string) (string, error)` — decodes every encoded word in a header
  value, dropping whitespace between adjacent words and leaving other text alone.

Errors: `ErrInvalid`, `ErrUnsupportedEncoding`, `ErrInvalidUTF8`.

## The awkward edge

`DecodeHeader` removes whitespace between two adjacent encoded words, because
RFC 2047 says that whitespace is a separator and not part of the text. That means
`=?UTF-8?Q?foo?= =?UTF-8?Q?bar?=` decodes to `foobar`, not `foo bar`. If you want a
space, encode it inside one word.
