package binarycookies

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestEncodeRoundTrip(t *testing.T) {
	a, b, c := sampleCookie(), sampleCookie(), sampleCookie()
	b.Name, b.Comment, b.Flags = "b", "a comment", 0xffffffff
	c.Domain, c.Value = ".other.example", ""

	tests := []struct {
		name string
		file *File
	}{
		{name: "no pages", file: &File{}},
		{name: "empty page", file: &File{Pages: []Page{{}}}},
		{name: "single cookie", file: singleCookieFile(a)},
		{name: "zero values", file: singleCookieFile(Cookie{})},
		{name: "time limits", file: singleCookieFile(Cookie{
			Expires: time.Unix(maxUnixSeconds, 0).UTC(),
			Created: time.Unix(minUnixSeconds, 0).UTC(),
		})},
		{name: "arbitrary bytes", file: singleCookieFile(Cookie{
			Domain: "\xff\xfe", Name: "名前", Path: "/a b", Value: "\x01\t\n=;",
		})},
		{name: "several pages", file: &File{Pages: []Page{
			{Cookies: []Cookie{a, b}},
			{},
			{Cookies: []Cookie{c}},
		}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mustDecode(t, mustEncode(t, tt.file)); !reflect.DeepEqual(got, tt.file) {
				t.Errorf("got %+v, want %+v", got, tt.file)
			}
		})
	}
}

func TestEncodeDiscardsFractionalSeconds(t *testing.T) {
	c := sampleCookie()
	c.Expires = c.Expires.Add(999 * time.Millisecond)
	got := decodeSingle(t, mustEncode(t, singleCookieFile(c)))
	if want := c.Expires.Truncate(time.Second); !got.Expires.Equal(want) {
		t.Errorf("Expires = %v, want %v", got.Expires, want)
	}
}

func TestEncodeReproducesSafariFiles(t *testing.T) {
	for _, name := range safariVectors {
		data := vector(t, name)
		encoded := mustEncode(t, mustDecode(t, data))
		if len(encoded) != footerEnd(data) || !bytes.HasPrefix(data, encoded) {
			t.Errorf("%s: re-encoding does not reproduce the original bytes up to the footer", name)
		}
	}
}

func TestEncodeRejects(t *testing.T) {
	tests := []struct {
		field  string
		mutate func(*Cookie)
	}{
		{field: "domain", mutate: func(c *Cookie) { c.Domain = "a\x00b" }},
		{field: "name", mutate: func(c *Cookie) { c.Name = "\x00" }},
		{field: "path", mutate: func(c *Cookie) { c.Path = "/\x00" }},
		{field: "value", mutate: func(c *Cookie) { c.Value = "v\x00" }},
		{field: "comment", mutate: func(c *Cookie) { c.Comment = "\x00note" }},
		{field: "expires", mutate: func(c *Cookie) { c.Expires = time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC) }},
		{field: "created", mutate: func(c *Cookie) { c.Created = time.Date(0, 12, 31, 23, 59, 59, 0, time.UTC) }},
	}
	for _, tt := range tests {
		c := sampleCookie()
		tt.mutate(&c)
		var buf bytes.Buffer
		err := Encode(&buf, singleCookieFile(c))
		if err == nil || !strings.Contains(err.Error(), tt.field) {
			t.Errorf("%s: got %v, want an error naming the field", tt.field, err)
		}
		if buf.Len() != 0 {
			t.Errorf("%s: wrote %d bytes despite the error", tt.field, buf.Len())
		}
	}
}

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestEncodeWriteError(t *testing.T) {
	errWrite := errors.New("write failed")
	if err := Encode(failingWriter{errWrite}, singleCookieFile(sampleCookie())); !errors.Is(err, errWrite) {
		t.Errorf("got %v, want an error wrapping %v", err, errWrite)
	}
}
