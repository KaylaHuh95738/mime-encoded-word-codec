package mimeword

import (
	"errors"
	"testing"
)

func TestEncodeB(t *testing.T) {
	got := Encode("héllo", EncodingB)
	want := "=?UTF-8?B?aMOpbGxv?="
	if got != want {
		t.Fatalf("Encode B = %q, want %q", got, want)
	}
}

func TestEncodeQ(t *testing.T) {
	cases := map[string]string{
		"hello":   "=?UTF-8?Q?hello?=",
		"a b":     "=?UTF-8?Q?a_b?=",
		"a?b":     "=?UTF-8?Q?a=3Fb?=",
		"a_b":     "=?UTF-8?Q?a=5Fb?=",
		"a=b":     "=?UTF-8?Q?a=3Db?=",
		"héllo":   "=?UTF-8?Q?h=C3=A9llo?=",
		"tabs\t":  "=?UTF-8?Q?tabs=09?=",
		"":        "=?UTF-8?Q??=",
	}
	for in, want := range cases {
		if got := Encode(in, EncodingQ); got != want {
			t.Errorf("Encode(%q, Q) = %q, want %q", in, got, want)
		}
	}
}

func TestDecodeB(t *testing.T) {
	got, err := Decode("=?UTF-8?B?aMOpbGxv?=")
	if err != nil {
		t.Fatal(err)
	}
	if got != "héllo" {
		t.Fatalf("got %q, want %q", got, "héllo")
	}
}

func TestDecodeQ(t *testing.T) {
	got, err := Decode("=?UTF-8?Q?h=C3=A9llo?=")
	if err != nil {
		t.Fatal(err)
	}
	if got != "héllo" {
		t.Fatalf("got %q, want %q", got, "héllo")
	}
}

func TestDecodeQSpace(t *testing.T) {
	got, err := Decode("=?UTF-8?Q?a_b?=")
	if err != nil {
		t.Fatal(err)
	}
	if got != "a b" {
		t.Fatalf("got %q, want %q", got, "a b")
	}
}

func TestDecodeRejectsBadSyntax(t *testing.T) {
	bad := []string{
		"hello",
		"=?UTF-8?B?",
		"=?UTF-8?B?!!?=",
		"=?UTF-8?Q?=ZZ?=",
		"=?UTF-8?Q?=A?=",
	}
	for _, w := range bad {
		if _, err := Decode(w); err == nil {
			t.Errorf("Decode(%q) succeeded, want error", w)
		}
	}
}

func TestDecodeRejectsUnsupportedCharset(t *testing.T) {
	_, err := Decode("=?ISO-8859-1?Q?caf=E9?=")
	if !errors.Is(err, ErrUnsupportedEncoding) {
		t.Fatalf("err = %v, want ErrUnsupportedEncoding", err)
	}
}

func TestDecodeRejectsNonUTF8(t *testing.T) {
	// 0xFF is not valid UTF-8.
	_, err := Decode("=?UTF-8?B?/w==?=")
	if !errors.Is(err, ErrInvalidUTF8) {
		t.Fatalf("err = %v, want ErrInvalidUTF8", err)
	}
}

func TestDecodeHeaderMixed(t *testing.T) {
	got, err := DecodeHeader("Re: =?UTF-8?Q?h=C3=A9llo?= world")
	if err != nil {
		t.Fatal(err)
	}
	want := "Re: héllo world"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDecodeHeaderAdjacentWordsDropSpace(t *testing.T) {
	got, err := DecodeHeader("=?UTF-8?Q?foo?= =?UTF-8?Q?bar?=")
	if err != nil {
		t.Fatal(err)
	}
	if got != "foobar" {
		t.Fatalf("got %q, want %q", got, "foobar")
	}
}

func TestDecodeHeaderLeavesLiteralText(t *testing.T) {
	got, err := DecodeHeader("plain =?not a word")
	if err != nil {
		t.Fatal(err)
	}
	if got != "plain =?not a word" {
		t.Fatalf("got %q", got)
	}
}

func TestRoundTrip(t *testing.T) {
	inputs := []string{
		"hello",
		"héllo wörld",
		"日本語",
		"emoji 🚀",
		"mixed =? literal ?= text",
		"",
	}
	for _, s := range inputs {
		for _, enc := range []Encoding{EncodingB, EncodingQ} {
			word := Encode(s, enc)
			got, err := Decode(word)
			if err != nil {
				t.Errorf("Decode(Encode(%q, %v)) error: %v", s, enc, err)
				continue
			}
			if got != s {
				t.Errorf("round trip %q via %v = %q", s, enc, got)
			}
		}
	}
}
