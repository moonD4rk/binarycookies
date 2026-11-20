package binarycookies

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

// FuzzDecode checks that Decode never panics, that every error it returns is
// classified, and that re-encoding a decoded file loses nothing.
func FuzzDecode(f *testing.F) {
	for _, name := range append([]string{"invalid.bin"}, safariVectors...) {
		f.Add(vector(f, name))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		file, err := Decode(bytes.NewReader(data))
		if err != nil {
			if !errors.Is(err, ErrMalformed) && !errors.Is(err, ErrInvalidSignature) {
				t.Fatalf("unclassified error: %v", err)
			}
			return
		}
		checkDecoded(t, file)

		var buf bytes.Buffer
		if err = Encode(&buf, file); err != nil {
			t.Fatalf("Encode of a decoded file: %v", err)
		}
		again, err := Decode(&buf)
		if err != nil {
			t.Fatalf("Decode of a re-encoded file: %v", err)
		}
		if !reflect.DeepEqual(again, file) {
			t.Fatalf("re-encoding changed the file:\nbefore %+v\nafter  %+v", file, again)
		}
	})
}

// FuzzRoundTrip checks that Encode either rejects a cookie the format cannot
// represent or writes one that decodes back unchanged.
func FuzzRoundTrip(f *testing.F) {
	f.Add(".example.com", "session", "/", "abc123", "", uint32(FlagSecure|FlagHTTPOnly),
		int64(1893553445), int64(1717747750))
	f.Add("", "", "", "", "note", uint32(0xffffffff), int64(maxUnixSeconds), int64(minUnixSeconds))
	f.Add("a\x00b", "", "", "", "", uint32(0), int64(maxUnixSeconds+1), int64(minUnixSeconds-1))
	f.Fuzz(func(t *testing.T, domain, name, path, value, comment string, flags uint32, expires, created int64) {
		in := singleCookieFile(Cookie{
			Domain:  domain,
			Name:    name,
			Path:    path,
			Value:   value,
			Comment: comment,
			Flags:   Flags(flags),
			Expires: time.Unix(expires, 0).UTC(),
			Created: time.Unix(created, 0).UTC(),
		})
		var buf bytes.Buffer
		err := Encode(&buf, in)

		encodable := !strings.Contains(domain+name+path+value+comment, "\x00") &&
			inUnixRange(expires) && inUnixRange(created)
		if !encodable {
			if err == nil {
				t.Fatal("Encode accepted a value the format cannot represent")
			}
			return
		}
		if err != nil {
			t.Fatalf("Encode: %v", err)
		}
		out, err := Decode(&buf)
		if err != nil {
			t.Fatalf("Decode: %v", err)
		}
		if !reflect.DeepEqual(out, in) {
			t.Fatalf("round trip changed the file:\nin  %+v\nout %+v", in, out)
		}
	})
}

func inUnixRange(seconds int64) bool {
	return seconds >= minUnixSeconds && seconds <= maxUnixSeconds
}

func checkDecoded(t *testing.T, f *File) {
	t.Helper()
	for _, c := range f.Cookies() {
		if strings.Contains(c.Domain+c.Name+c.Path+c.Value+c.Comment, "\x00") {
			t.Fatalf("decoded strings contain NUL: %+v", c)
		}
		for _, ts := range []time.Time{c.Expires, c.Created} {
			if ts.Location() != time.UTC || ts.Nanosecond() != 0 || !inUnixRange(ts.Unix()) {
				t.Fatalf("decoded time %v is not a whole UTC second within years 1 to 9999", ts)
			}
		}
	}
}
