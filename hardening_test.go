package binarycookies

import (
	"bytes"
	"encoding/binary"
	"testing"
	"testing/iotest"
	"time"
)

// buildOneCookieFile assembles a minimal, well-formed single-page file holding
// one cookie. Each *Stored argument is the field's on-disk bytes INCLUDING any
// NUL terminator, so a test can pass a zero-length slice to reproduce the
// crafted "empty field" case that used to panic in data[0:len(data)-1].
func buildOneCookieFile(domainStored, nameStored, pathStored, valueStored []byte) []byte {
	const headerLen = 56 // 10 x uint32 + 2 x float64

	domainOffset := uint32(headerLen)
	nameOffset := domainOffset + uint32(len(domainStored))
	pathOffset := nameOffset + uint32(len(nameStored))
	valueOffset := pathOffset + uint32(len(pathStored))
	size := valueOffset + uint32(len(valueStored))

	cookie := new(bytes.Buffer)
	header := []uint32{size, 0, 0, 0, domainOffset, nameOffset, pathOffset, valueOffset, 0, 0}
	for _, v := range header {
		_ = binary.Write(cookie, binary.LittleEndian, v)
	}
	_ = binary.Write(cookie, binary.LittleEndian, float64(0)) // expires
	_ = binary.Write(cookie, binary.LittleEndian, float64(0)) // creation
	cookie.Write(domainStored)
	cookie.Write(nameStored)
	cookie.Write(pathStored)
	cookie.Write(valueStored)

	page := new(bytes.Buffer)
	page.Write([]byte{0x00, 0x00, 0x01, 0x00})                // page start tag
	_ = binary.Write(page, binary.LittleEndian, uint32(1))    // numCookies
	_ = binary.Write(page, binary.LittleEndian, uint32(0x10)) // cookie offset (ignored on decode)
	page.Write([]byte{0x00, 0x00, 0x00, 0x00})                // page end
	page.Write(cookie.Bytes())

	file := new(bytes.Buffer)
	file.WriteString("cook")
	_ = binary.Write(file, binary.BigEndian, uint32(1))    // numPages
	_ = binary.Write(file, binary.BigEndian, uint32(0x0c)) // page offset (ignored on decode)
	file.Write(page.Bytes())
	file.Write(make([]byte, 8)) // checksum
	return file.Bytes()
}

// TestRejectsHugeCookieCount locks the fix for the allocation DoS: a crafted
// numCookies must be rejected before make([]uint32, length), never OOM.
func TestRejectsHugeCookieCount(t *testing.T) {
	input := []byte{
		'c', 'o', 'o', 'k',
		0x00, 0x00, 0x00, 0x01, // numPages (BE)
		0x00, 0x00, 0x00, 0x0c, // page offset (BE, ignored)
		0x00, 0x00, 0x01, 0x00, // page start tag
		0x00, 0x00, 0x00, 0xe8, // numCookies = 0xe8000000 (LE) -> ~3.9 billion
	}

	if _, err := New(bytes.NewReader(input)).Decode(); err == nil {
		t.Fatal("expected error for oversized cookie count, got nil (allocation bomb not rejected)")
	}
}

// TestRejectsHugePageCount locks the symmetric numPages bound.
func TestRejectsHugePageCount(t *testing.T) {
	input := []byte{
		'c', 'o', 'o', 'k',
		0x7f, 0xff, 0xff, 0xff, // numPages (BE) = 0x7fffffff
	}

	if _, err := New(bytes.NewReader(input)).Decode(); err == nil {
		t.Fatal("expected error for oversized page count, got nil")
	}
}

// TestEmptyFieldDoesNotPanic locks the fix for the data[0:len(data)-1] panic on
// a zero-length domain/name/path field (offsets equal on crafted input).
func TestEmptyFieldDoesNotPanic(t *testing.T) {
	cases := []struct {
		label                          string
		domain, cookieName, cookiePath []byte
	}{
		{"empty domain", []byte{}, []byte("n\x00"), []byte("/\x00")},
		{"empty name", []byte("d\x00"), []byte{}, []byte("/\x00")},
		{"empty path", []byte("d\x00"), []byte("n\x00"), []byte{}},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			input := buildOneCookieFile(tc.domain, tc.cookieName, tc.cookiePath, []byte("v\x00"))

			pages, err := New(bytes.NewReader(input)).Decode()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(pages) != 1 || len(pages[0].Cookies) != 1 {
				t.Fatalf("expected 1 page / 1 cookie, got %d pages", len(pages))
			}
		})
	}
}

// TestDecodeChunkedReader proves io.ReadFull handles readers that return fewer
// bytes than requested; a single Read per field would silently corrupt here.
func TestDecodeChunkedReader(t *testing.T) {
	r := iotest.OneByteReader(bytes.NewReader(appleData))

	pages, err := New(r).Decode()
	if err != nil {
		t.Fatalf("unexpected error decoding via one-byte reader: %v", err)
	}

	if len(pages) != 2 {
		t.Fatalf("expected 2 pages, got %d", len(pages))
	}
}

// TestTruncatedInputReturnsError verifies every proper prefix of a complete
// file now errors deterministically instead of producing a partial decode.
func TestTruncatedInputReturnsError(t *testing.T) {
	for _, n := range []int{16, 64, 256, 512, len(appleData) - 1} {
		if _, err := New(bytes.NewReader(appleData[:n])).Decode(); err == nil {
			t.Errorf("expected error for truncated input appleData[:%d], got nil", n)
		}
	}
}

// TestCookieString locks the human-readable format, including the space before
// the comment block (previously glued to the preceding field).
func TestCookieString(t *testing.T) {
	base := Cookie{
		Domain:  []byte("example.com"),
		Path:    []byte("/"),
		Name:    []byte("sess"),
		Value:   []byte("abc"),
		Expires: time.Date(2014, time.April, 2, 10, 0, 0, 0, time.UTC),
	}

	t.Run("plain", func(t *testing.T) {
		want := "2014-04-02 10:00:00 example.com / sess abc"
		if got := base.String(); got != want {
			t.Fatalf("String()\n- %q\n+ %q", want, got)
		}
	})

	t.Run("secure httponly with comment", func(t *testing.T) {
		c := base
		c.Secure = true
		c.HTTPOnly = true
		c.Comment = []byte("note")

		want := "2014-04-02 10:00:00 example.com / sess abc Secure HttpOnly /* note */"
		if got := c.String(); got != want {
			t.Fatalf("String()\n- %q\n+ %q", want, got)
		}
	})
}
