package binarycookies

import "time"

// File is the content of a Binary Cookies file, as returned by Decode and
// written by Encode.
type File struct {
	// Pages holds the pages of the file in stored order.
	Pages []Page
}

// Cookies returns the cookies of all pages in stored order.
// The returned slice is newly allocated.
func (f *File) Cookies() []Cookie {
	n := 0
	for i := range f.Pages {
		n += len(f.Pages[i].Cookies)
	}
	cookies := make([]Cookie, 0, n)
	for i := range f.Pages {
		cookies = append(cookies, f.Pages[i].Cookies...)
	}
	return cookies
}

// Page is a storage page of a file. Safari groups cookies into pages, usually
// one per domain, and may leave pages empty.
type Page struct {
	// Cookies holds the cookies of the page in stored order.
	Cookies []Cookie
}

// Cookie is a single stored cookie.
type Cookie struct {
	Domain  string
	Name    string
	Path    string
	Value   string
	Comment string // empty when the cookie has no comment

	// Flags holds the raw flag bits, including bits this package does not interpret.
	Flags Flags

	// Expires and Created are in UTC with one-second precision. They are zero
	// when the stored timestamp is not a valid time.
	Expires time.Time
	Created time.Time
}

// Secure reports whether the cookie is only sent over secure connections.
func (c Cookie) Secure() bool {
	return c.Flags&FlagSecure != 0
}

// HTTPOnly reports whether the cookie is hidden from scripts.
func (c Cookie) HTTPOnly() bool {
	return c.Flags&FlagHTTPOnly != 0
}

// Flags is the raw flag word of a cookie.
type Flags uint32

// Flag bits with a known meaning.
const (
	FlagSecure   Flags = 1 << 0
	FlagHTTPOnly Flags = 1 << 2
)
