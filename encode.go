package binarycookies

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"strings"
)

// Encode writes f to w in the Binary Cookies format, computing all offsets,
// sizes and the checksum. Nothing is written when f cannot be encoded.
//
// See the package documentation for the values Encode rejects.
func Encode(w io.Writer, f *File) error {
	data, err := encode(f)
	if err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("binarycookies: write: %w", err)
	}
	return nil
}

func encode(f *File) ([]byte, error) {
	if !fitsUint32(len(f.Pages)) {
		return nil, fmt.Errorf("binarycookies: page count %d does not fit in 32 bits", len(f.Pages))
	}
	pages := make([][]byte, len(f.Pages))
	size := fileHeaderSize + 4*len(f.Pages) + footerSize
	for i := range f.Pages {
		page, err := encodePage(&f.Pages[i])
		if err != nil {
			return nil, fmt.Errorf("binarycookies: page %d: %w", i, err)
		}
		pages[i] = page
		size += len(page)
	}

	out := make([]byte, 0, size)
	out = append(out, fileSignature...)
	out = binary.BigEndian.AppendUint32(out, uint32(len(pages)))
	for _, page := range pages {
		out = binary.BigEndian.AppendUint32(out, uint32(len(page)))
	}
	var checksum uint32
	for _, page := range pages {
		out = append(out, page...)
		checksum += pageChecksum(page)
	}
	out = binary.BigEndian.AppendUint32(out, checksum)
	return append(out, footerTag...), nil
}

func encodePage(p *Page) ([]byte, error) {
	records := make([][]byte, len(p.Cookies))
	headerSize := pageHeaderSize + 4*len(p.Cookies)
	size := headerSize
	for i := range p.Cookies {
		record, err := encodeCookie(&p.Cookies[i])
		if err != nil {
			return nil, fmt.Errorf("cookie %d: %w", i, err)
		}
		records[i] = record
		size += len(record)
	}
	if !fitsUint32(size) {
		return nil, fmt.Errorf("page size %d does not fit in 32 bits", size)
	}

	out := make([]byte, 0, size)
	out = append(out, pageStartTag...)
	out = binary.LittleEndian.AppendUint32(out, uint32(len(records)))
	offset := headerSize
	for _, record := range records {
		out = binary.LittleEndian.AppendUint32(out, uint32(offset))
		offset += len(record)
	}
	out = append(out, pageEndTag...)
	for _, record := range records {
		out = append(out, record...)
	}
	return out, nil
}

func encodeCookie(c *Cookie) ([]byte, error) {
	strs := []struct{ field, value string }{
		{"comment", c.Comment},
		{"domain", c.Domain},
		{"name", c.Name},
		{"path", c.Path},
		{"value", c.Value},
	}
	for _, s := range strs {
		if strings.IndexByte(s.value, 0) >= 0 {
			return nil, fmt.Errorf("%s contains a NUL byte", s.field)
		}
	}
	expires, ok := timeToMac(c.Expires)
	if !ok {
		return nil, fmt.Errorf("expires %v is outside the years 1 to 9999", c.Expires)
	}
	created, ok := timeToMac(c.Created)
	if !ok {
		return nil, fmt.Errorf("created %v is outside the years 1 to 9999", c.Created)
	}

	size := recordHeaderSize + len(c.Domain) + len(c.Name) + len(c.Path) + len(c.Value) + 4
	if c.Comment != "" {
		size += len(c.Comment) + 1
	}
	if !fitsUint32(size) {
		return nil, fmt.Errorf("record size %d does not fit in 32 bits", size)
	}

	le := binary.LittleEndian
	out := make([]byte, recordHeaderSize, size)
	le.PutUint32(out, uint32(size))
	le.PutUint32(out[recFlags:], uint32(c.Flags))
	le.PutUint64(out[recExpires:], math.Float64bits(expires))
	le.PutUint64(out[recCreated:], math.Float64bits(created))
	appendString := func(offsetAt int, s string) {
		le.PutUint32(out[offsetAt:], uint32(len(out)))
		out = append(append(out, s...), 0)
	}
	if c.Comment != "" {
		appendString(recCommentOffset, c.Comment)
	}
	appendString(recDomainOffset, c.Domain)
	appendString(recNameOffset, c.Name)
	appendString(recPathOffset, c.Path)
	appendString(recValueOffset, c.Value)
	return out, nil
}

func fitsUint32(n int) bool {
	return n >= 0 && uint64(n) <= math.MaxUint32
}
