package binarycookies

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"testing/iotest"
	"time"
)

func TestDecodeFlags(t *testing.T) {
	tests := []struct {
		flags            Flags
		secure, httpOnly bool
	}{
		{flags: 0x0},
		{flags: 0x1, secure: true},
		{flags: 0x4, httpOnly: true},
		{flags: 0x5, secure: true, httpOnly: true},
		{flags: 0x2},
		{flags: 0x9, secure: true},
		{flags: 0xd, secure: true, httpOnly: true},
		{flags: 0xffffffff, secure: true, httpOnly: true},
	}
	for _, tt := range tests {
		c := sampleCookie()
		c.Flags = tt.flags
		got := decodeSingle(t, mustEncode(t, singleCookieFile(c)))
		if got.Flags != tt.flags || got.Secure() != tt.secure || got.HTTPOnly() != tt.httpOnly {
			t.Errorf("flags %#x: got Flags=%#x Secure=%v HTTPOnly=%v, want Secure=%v HTTPOnly=%v",
				tt.flags, got.Flags, got.Secure(), got.HTTPOnly(), tt.secure, tt.httpOnly)
		}
	}
}

func TestDecodeComment(t *testing.T) {
	for _, comment := range []string{"", "used for login"} {
		c := sampleCookie()
		c.Comment = comment
		if got := decodeSingle(t, mustEncode(t, singleCookieFile(c))); got != c {
			t.Errorf("comment %q: got %+v, want %+v", comment, got, c)
		}
	}
}

func TestDecodeLocatesStringsByOffset(t *testing.T) {
	data := mustEncode(t, singleCookieFile(sampleCookie()))
	at := onlyRecordAt + recDomainOffset
	binary.LittleEndian.PutUint32(data[at:], binary.LittleEndian.Uint32(data[at:])+1)

	if got := decodeSingle(t, data).Domain; got != "example.com" {
		t.Errorf("Domain = %q, want %q", got, "example.com")
	}
}

func TestDecodeStringEndsAtFirstNUL(t *testing.T) {
	c := sampleCookie()
	c.Value = "abc|trailing metadata"
	data := mustEncode(t, singleCookieFile(c))
	data[bytes.Index(data, []byte("|trailing"))] = 0

	if got := decodeSingle(t, data).Value; got != "abc" {
		t.Errorf("Value = %q, want %q", got, "abc")
	}
}

func TestDecodeStringWithoutTerminator(t *testing.T) {
	for _, value := range []string{"abc123", ""} {
		c := sampleCookie()
		c.Value = value
		record := mustRecord(t, c)
		record = record[:len(record)-1]
		binary.LittleEndian.PutUint32(record, uint32(len(record)))
		header := pageHeaderSize + 4
		data := buildFile(buildPage(header+len(record), placedRecord{header, record}))

		if got := decodeSingle(t, data).Value; got != value {
			t.Errorf("Value = %q, want %q", got, value)
		}
	}
}

func TestDecodeIgnoresUnknownHeaderFields(t *testing.T) {
	c := sampleCookie()
	data := mustEncode(t, singleCookieFile(c))
	for _, at := range []int{recUnknown1, recUnknown2, recUnknown3} {
		binary.LittleEndian.PutUint32(data[onlyRecordAt+at:], 0xdeadbeef)
	}
	if got := decodeSingle(t, data); got != c {
		t.Errorf("got %+v, want %+v", got, c)
	}
}

func TestDecodeAllowsGapsBetweenRecords(t *testing.T) {
	a, b := sampleCookie(), sampleCookie()
	b.Name = "other"
	recordA, recordB := mustRecord(t, a), mustRecord(t, b)
	offsetA := pageHeaderSize + 2*4 + 5
	offsetB := offsetA + len(recordA) + 7
	page := buildPage(offsetB+len(recordB)+3,
		placedRecord{offsetA, recordA},
		placedRecord{offsetB, recordB},
	)

	want := &File{Pages: []Page{{Cookies: []Cookie{a, b}}}}
	if got := mustDecode(t, buildFile(page)); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestDecodeTimestamps(t *testing.T) {
	epoch := time.Date(2001, time.January, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		seconds float64
		want    time.Time
	}{
		{name: "Mac epoch", seconds: 0, want: epoch},
		{name: "fraction discarded", seconds: 1.9, want: epoch.Add(time.Second)},
		{name: "negative fraction discarded", seconds: -0.5, want: epoch.Add(-time.Second)},
		{name: "last second of 9999", seconds: maxUnixSeconds - macEpochUnix, want: time.Unix(maxUnixSeconds, 0).UTC()},
		{name: "first second of year 1", seconds: minUnixSeconds - macEpochUnix},
		{name: "after 9999", seconds: maxUnixSeconds - macEpochUnix + 1},
		{name: "before year 1", seconds: minUnixSeconds - macEpochUnix - 1},
		{name: "NaN", seconds: math.NaN()},
		{name: "+Inf", seconds: math.Inf(1)},
		{name: "-Inf", seconds: math.Inf(-1)},
		{name: "huge", seconds: 1e300},
		{name: "tiny", seconds: -1e300},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := mustEncode(t, singleCookieFile(sampleCookie()))
			binary.LittleEndian.PutUint64(data[onlyRecordAt+recExpires:], math.Float64bits(tt.seconds))
			if got := decodeSingle(t, data).Expires; got != tt.want {
				t.Errorf("Expires = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDecodeLargeCookie(t *testing.T) {
	c := sampleCookie()
	c.Value = strings.Repeat("v", 64<<10)
	if got := decodeSingle(t, mustEncode(t, singleCookieFile(c))); got != c {
		t.Errorf("large cookie did not survive decoding")
	}
}

func TestDecodeIgnoresFooterAndTrailingData(t *testing.T) {
	data := mustEncode(t, singleCookieFile(sampleCookie()))
	want := mustDecode(t, data)

	badFooter := append([]byte(nil), data...)
	copy(badFooter[len(badFooter)-footerSize:], "garbage!")
	trailing := append(append([]byte(nil), data...), "bplist00 accept policy"...)

	for name, input := range map[string][]byte{"bad footer": badFooter, "trailing data": trailing} {
		if got := mustDecode(t, input); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %+v, want %+v", name, got, want)
		}
	}
}

func TestDecodeEmptyStructures(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want *File
	}{
		{name: "no pages", data: buildFile(), want: &File{}},
		{name: "empty page", data: buildFile(buildPage(pageHeaderSize)), want: &File{Pages: []Page{{}}}},
	}
	for _, tt := range tests {
		if got := mustDecode(t, tt.data); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: got %+v, want %+v", tt.name, got, tt.want)
		}
	}
}

func TestDecodeShortReads(t *testing.T) {
	data := vector(t, safariVectors[0])
	want := mustDecode(t, data)
	readers := map[string]io.Reader{
		"one byte":  iotest.OneByteReader(bytes.NewReader(data)),
		"half":      iotest.HalfReader(bytes.NewReader(data)),
		"data+EOF":  iotest.DataErrReader(bytes.NewReader(data)),
		"timeout":   iotest.TimeoutReader(bytes.NewReader(data)),
		"unchanged": bytes.NewReader(data),
	}
	for name, r := range readers {
		got, err := Decode(r)
		if name == "timeout" {
			if !errors.Is(err, iotest.ErrTimeout) {
				t.Errorf("%s: error %v, want %v", name, err, iotest.ErrTimeout)
			}
			continue
		}
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %+v, %v", name, got, err)
		}
	}
}

func TestDecodeInvalidSignature(t *testing.T) {
	inputs := map[string][]byte{
		"empty":     nil,
		"short":     []byte("coo"),
		"wrong":     []byte("cooK\x00\x00\x00\x00\x07\x17\x20\x05\x00\x00\x00\x00"),
		"plist":     []byte("bplist00\xd1\x01\x02"),
		"text file": vector(t, "invalid.bin"),
	}
	for name, input := range inputs {
		_, err := Decode(bytes.NewReader(input))
		if !errors.Is(err, ErrInvalidSignature) || errors.Is(err, ErrMalformed) {
			t.Errorf("%s: got %v, want %v", name, err, ErrInvalidSignature)
		}
	}
}

func TestDecodeTruncated(t *testing.T) {
	for _, name := range safariVectors {
		data := vector(t, name)
		for n := len(fileSignature); n < footerEnd(data); n++ {
			_, err := Decode(bytes.NewReader(data[:n]))
			if !errors.Is(err, ErrMalformed) || !errors.Is(err, io.ErrUnexpectedEOF) {
				t.Fatalf("%s truncated to %d bytes: got %v, want a truncation error", name, n, err)
			}
		}
	}
}

func TestDecodeMalformed(t *testing.T) {
	single := func() []byte {
		return mustEncode(t, singleCookieFile(sampleCookie()))
	}
	double := func() []byte {
		a, b := sampleCookie(), sampleCookie()
		b.Name = "other"
		return mustEncode(t, &File{Pages: []Page{{Cookies: []Cookie{a, b}}}})
	}
	setLE := func(data []byte, at int, v uint32) []byte {
		binary.LittleEndian.PutUint32(data[at:], v)
		return data
	}
	swapOffsets := func(data []byte) []byte {
		first, second := onlyPageAt+pageOffsetsAt, onlyPageAt+pageOffsetsAt+4
		a, b := binary.LittleEndian.Uint32(data[first:]), binary.LittleEndian.Uint32(data[second:])
		return setLE(setLE(data, first, b), second, a)
	}
	firstOffset := func(data []byte) uint32 {
		return binary.LittleEndian.Uint32(data[onlyPageAt+pageOffsetsAt:])
	}

	shortPage := single()
	binary.BigEndian.PutUint32(shortPage[fileHeaderSize:], uint32(pageHeaderSize-1))
	badStartTag := single()
	badStartTag[onlyPageAt+2] = 0x02
	badEndTag := single()
	badEndTag[onlyPageAt+pageOffsetsAt+4] = 0x01
	overlapping := double()
	overlapping = setLE(overlapping, onlyPageAt+pageOffsetsAt+4, firstOffset(overlapping))

	tests := map[string][]byte{
		"page smaller than its header":   shortPage,
		"invalid page start tag":         badStartTag,
		"invalid page end tag":           badEndTag,
		"more cookies than fit":          setLE(single(), onlyPageAt+pageCountAt, 1000),
		"record inside page header":      setLE(single(), onlyPageAt+pageOffsetsAt, uint32(pageHeaderSize)),
		"record past page end":           setLE(single(), onlyPageAt+pageOffsetsAt, 1<<20),
		"record smaller than its header": setLE(single(), onlyRecordAt, recordHeaderSize-1),
		"record past page end by size":   setLE(single(), onlyRecordAt, 1<<20),
		"overlapping records":            overlapping,
		"records out of order":           swapOffsets(double()),
		"string inside record header":    setLE(single(), onlyRecordAt+recNameOffset, recordHeaderSize-1),
		"string past record end":         setLE(single(), onlyRecordAt+recValueOffset, 1<<20),
		"comment past record end":        setLE(single(), onlyRecordAt+recCommentOffset, 1<<20),
	}
	for name, data := range tests {
		_, err := Decode(bytes.NewReader(data))
		if !errors.Is(err, ErrMalformed) || errors.Is(err, io.ErrUnexpectedEOF) {
			t.Errorf("%s: got %v, want a malformed-file error that is not a truncation", name, err)
		}
	}
}

func TestDecodeReadError(t *testing.T) {
	errRead := errors.New("read failed")
	_, err := Decode(iotest.ErrReader(errRead))
	if !errors.Is(err, errRead) || errors.Is(err, ErrMalformed) {
		t.Errorf("got %v, want an error wrapping %v", err, errRead)
	}
}

func TestDecodeAllocationIsBounded(t *testing.T) {
	const claimed = 1 << 20
	hostile := make([]byte, pageHeaderSize+4*claimed)
	copy(hostile, pageStartTag)
	binary.LittleEndian.PutUint32(hostile[pageCountAt:], claimed)
	copy(hostile[pageOffsetsAt+4*claimed:], pageEndTag)

	const count = 20000
	record := mustRecord(t, Cookie{})
	header := pageHeaderSize + 4*count
	records := make([]placedRecord, count)
	for i := range records {
		records[i] = placedRecord{offset: header + i*len(record), data: record}
	}
	dense := buildFile(buildPage(header+count*len(record), records...))

	tests := []struct {
		name  string
		data  []byte
		limit uint64
	}{
		{name: "count larger than the page can hold", data: buildFile(hostile), limit: 64 << 10},
		{name: "many small cookies", data: dense, limit: 8 * uint64(len(dense))},
	}
	for _, tt := range tests {
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		_, _ = decode(tt.data)
		runtime.ReadMemStats(&after)
		if allocated := after.TotalAlloc - before.TotalAlloc; allocated > tt.limit {
			t.Errorf("%s: allocated %d bytes for %d bytes of input, limit %d",
				tt.name, allocated, len(tt.data), tt.limit)
		}
	}
}

func TestFileCookies(t *testing.T) {
	a, b, c := sampleCookie(), sampleCookie(), sampleCookie()
	b.Name, c.Name = "b", "c"
	f := &File{Pages: []Page{{Cookies: []Cookie{a, b}}, {}, {Cookies: []Cookie{c}}}}

	got := f.Cookies()
	if want := []Cookie{a, b, c}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Cookies() = %+v, want %+v", got, want)
	}
	got[0].Name = "changed"
	if f.Pages[0].Cookies[0].Name != a.Name {
		t.Error("Cookies() shares memory with the pages")
	}
}
