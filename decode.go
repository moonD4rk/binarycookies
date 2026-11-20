package binarycookies

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"strings"
)

// Decode reads all of r into memory and decodes it as a Binary Cookies file.
//
// See the package documentation for the decoding rules and the errors returned.
func Decode(r io.Reader) (*File, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("binarycookies: read: %w", err)
	}
	return decode(data)
}

func decode(data []byte) (*File, error) {
	signature := data
	if len(signature) > len(fileSignature) {
		signature = signature[:len(fileSignature)]
	}
	if string(signature) != fileSignature {
		return nil, fmt.Errorf("%w: %q", ErrInvalidSignature, signature)
	}
	if len(data) < fileHeaderSize {
		return nil, truncated("page count")
	}

	numPages := binary.BigEndian.Uint32(data[len(fileSignature):])
	tableEnd := uint64(fileHeaderSize) + 4*uint64(numPages)
	if tableEnd > uint64(len(data)) {
		return nil, truncated("page size table")
	}

	var pages []Page
	if numPages > 0 {
		pages = make([]Page, 0, numPages)
	}
	pos := int(tableEnd)
	for i := 0; i < int(numPages); i++ {
		size := binary.BigEndian.Uint32(data[fileHeaderSize+4*i:])
		if uint64(size) > uint64(len(data)-pos) {
			return nil, truncated(fmt.Sprintf("page %d", i))
		}
		page, err := decodePage(data[pos : pos+int(size)])
		if err != nil {
			return nil, malformed(fmt.Sprintf("page %d", i), err)
		}
		pages = append(pages, page)
		pos += int(size)
	}

	if len(data)-pos < footerSize {
		return nil, truncated("footer")
	}
	return &File{Pages: pages}, nil
}

func decodePage(page []byte) (Page, error) {
	if len(page) < pageHeaderSize {
		return Page{}, fmt.Errorf("size %d is smaller than a page header", len(page))
	}
	if tag := page[:pageCountAt]; string(tag) != pageStartTag {
		return Page{}, fmt.Errorf("invalid start tag %q", tag)
	}

	numCookies := binary.LittleEndian.Uint32(page[pageCountAt:])
	// Every cookie needs an offset table entry and a record header of its own.
	if uint64(numCookies)*(4+recordHeaderSize) > uint64(len(page)-pageHeaderSize) {
		return Page{}, fmt.Errorf("%d cookies do not fit in %d bytes", numCookies, len(page))
	}
	tableEnd := pageOffsetsAt + 4*int(numCookies)
	if tag := page[tableEnd : tableEnd+len(pageEndTag)]; string(tag) != pageEndTag {
		return Page{}, fmt.Errorf("invalid end tag %q", tag)
	}

	var cookies []Cookie
	if numCookies > 0 {
		cookies = make([]Cookie, 0, numCookies)
	}
	next := tableEnd + len(pageEndTag)
	for i := 0; i < int(numCookies); i++ {
		offset := binary.LittleEndian.Uint32(page[pageOffsetsAt+4*i:])
		cookie, end, err := decodeRecord(page, offset, next)
		if err != nil {
			return Page{}, fmt.Errorf("cookie %d: %w", i, err)
		}
		cookies = append(cookies, cookie)
		next = end
	}
	return Page{Cookies: cookies}, nil
}

// decodeRecord decodes the record at offset, which must not start before next
// so that records never overlap; it returns the end of the record.
func decodeRecord(page []byte, offset uint32, next int) (Cookie, int, error) {
	if uint64(offset) < uint64(next) {
		return Cookie{}, 0, fmt.Errorf("record offset %d overlaps data ending at %d", offset, next)
	}
	if uint64(offset)+recordHeaderSize > uint64(len(page)) {
		return Cookie{}, 0, fmt.Errorf("record offset %d is out of range", offset)
	}
	size := binary.LittleEndian.Uint32(page[offset:])
	if size < recordHeaderSize || uint64(offset)+uint64(size) > uint64(len(page)) {
		return Cookie{}, 0, fmt.Errorf("record size %d at offset %d is out of range", size, offset)
	}
	end := int(offset) + int(size)
	cookie, err := decodeCookie(page[offset:end])
	return cookie, end, err
}

func decodeCookie(record []byte) (Cookie, error) {
	le := binary.LittleEndian
	strs := recordStrings{record: string(record)}
	c := Cookie{
		Domain:  strs.at("domain", le.Uint32(record[recDomainOffset:])),
		Name:    strs.at("name", le.Uint32(record[recNameOffset:])),
		Path:    strs.at("path", le.Uint32(record[recPathOffset:])),
		Value:   strs.at("value", le.Uint32(record[recValueOffset:])),
		Flags:   Flags(le.Uint32(record[recFlags:])),
		Expires: timeFromMac(math.Float64frombits(le.Uint64(record[recExpires:]))),
		Created: timeFromMac(math.Float64frombits(le.Uint64(record[recCreated:]))),
	}
	if offset := le.Uint32(record[recCommentOffset:]); offset != 0 {
		c.Comment = strs.at("comment", offset)
	}
	if strs.err != nil {
		return Cookie{}, strs.err
	}
	return c, nil
}

// recordStrings extracts NUL-terminated strings from one record, keeping the
// first error. All strings share a single copy of the record.
type recordStrings struct {
	record string
	err    error
}

func (r *recordStrings) at(field string, offset uint32) string {
	if r.err != nil {
		return ""
	}
	if offset < recordHeaderSize || uint64(offset) > uint64(len(r.record)) {
		r.err = fmt.Errorf("%s offset %d is outside [%d, %d]", field, offset, recordHeaderSize, len(r.record))
		return ""
	}
	s, _, _ := strings.Cut(r.record[offset:], "\x00")
	return s
}
