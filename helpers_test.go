package binarycookies

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Positions inside a file that holds a single page with a single cookie.
const (
	onlyPageAt   = fileHeaderSize + 4
	onlyRecordAt = onlyPageAt + pageHeaderSize + 4
)

// safariVectors are files written by Safari, stored in testdata/vectors.
var safariVectors = []string{"apple.binarycookies", "multipage.binarycookies"}

// Record header fields this package does not interpret.
const (
	recUnknown1 = 4
	recUnknown2 = 12
	recUnknown3 = 36
)

func sampleCookie() Cookie {
	return Cookie{
		Domain:  ".example.com",
		Name:    "session",
		Path:    "/",
		Value:   "abc123",
		Flags:   FlagSecure | FlagHTTPOnly,
		Expires: time.Date(2030, time.January, 2, 3, 4, 5, 0, time.UTC),
		Created: time.Date(2024, time.June, 7, 8, 9, 10, 0, time.UTC),
	}
}

func singleCookieFile(c Cookie) *File {
	return &File{Pages: []Page{{Cookies: []Cookie{c}}}}
}

func mustEncode(t testing.TB, f *File) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := Encode(&buf, f); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	return buf.Bytes()
}

func mustDecode(t testing.TB, data []byte) *File {
	t.Helper()
	f, err := Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	return f
}

func decodeSingle(t testing.TB, data []byte) Cookie {
	t.Helper()
	cookies := mustDecode(t, data).Cookies()
	if len(cookies) != 1 {
		t.Fatalf("decoded %d cookies, want 1", len(cookies))
	}
	return cookies[0]
}

func mustRecord(t testing.TB, c Cookie) []byte {
	t.Helper()
	record, err := encodeCookie(&c)
	if err != nil {
		t.Fatalf("encodeCookie: %v", err)
	}
	return record
}

func readFile(t testing.TB, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func vector(t testing.TB, name string) []byte {
	t.Helper()
	return readFile(t, filepath.Join("testdata", "vectors", name))
}

// footerEnd returns where the footer of data ends, read straight from its header.
func footerEnd(data []byte) int {
	numPages := int(binary.BigEndian.Uint32(data[len(fileSignature):]))
	end := fileHeaderSize + 4*numPages
	for i := 0; i < numPages; i++ {
		end += int(binary.BigEndian.Uint32(data[fileHeaderSize+4*i:]))
	}
	return end + footerSize
}

// buildFile assembles raw pages into a file with a valid footer.
func buildFile(pages ...[]byte) []byte {
	out := binary.BigEndian.AppendUint32([]byte(fileSignature), uint32(len(pages)))
	var checksum uint32
	for _, page := range pages {
		out = binary.BigEndian.AppendUint32(out, uint32(len(page)))
		checksum += pageChecksum(page)
	}
	for _, page := range pages {
		out = append(out, page...)
	}
	out = binary.BigEndian.AppendUint32(out, checksum)
	return append(out, footerTag...)
}

type placedRecord struct {
	offset int
	data   []byte
}

// buildPage places records at their offsets in a page of size bytes and fills
// every byte outside the page header and the records with padding.
func buildPage(size int, records ...placedRecord) []byte {
	page := bytes.Repeat([]byte{0xee}, size)
	copy(page, pageStartTag)
	binary.LittleEndian.PutUint32(page[pageCountAt:], uint32(len(records)))
	for i, r := range records {
		binary.LittleEndian.PutUint32(page[pageOffsetsAt+4*i:], uint32(r.offset))
		copy(page[r.offset:], r.data)
	}
	copy(page[pageOffsetsAt+4*len(records):], pageEndTag)
	return page
}
