// Package binarycookies reads and writes the Binary Cookies format
// (Cookies.binarycookies) used by Safari and other WebKit-based applications
// on macOS and iOS.
//
// Decode parses a complete file and Encode writes one:
//
//	data, err := os.ReadFile("Cookies.binarycookies")
//	if err != nil {
//		return err
//	}
//	file, err := binarycookies.Decode(bytes.NewReader(data))
//	if err != nil {
//		return err
//	}
//	for _, c := range file.Cookies() {
//		fmt.Println(c.Domain, c.Name, c.Value, c.Expires)
//	}
//
// # Decoding rules
//
// Decode fails only when the structure of the file is broken: a wrong
// signature, an invalid page tag, a count, offset or size that points outside
// its enclosing page or record, cookie records that overlap or are out of
// order, or a file that ends early. Everything else is tolerated so that as
// many cookies as possible are recovered:
//
//   - header fields with an unknown meaning are ignored;
//   - all flag bits are kept in [Cookie.Flags], including unknown ones;
//   - the footer checksum and any data after the footer are ignored;
//   - gaps between cookie records and between strings are allowed;
//   - an invalid timestamp decodes as the zero [time.Time].
//
// Strings are NUL-terminated and located by their offsets: each one ends at
// its first NUL byte or at the end of its record, so decoded strings never
// contain NUL. Bytes are returned as stored and are not validated as UTF-8.
//
// Times are in UTC with one-second precision; fractional seconds are
// discarded. A timestamp that is NaN, infinite or outside the years 1 to 9999
// decodes as the zero [time.Time].
//
// Memory use is proportional to the size of the input: counts and offsets read
// from the file are checked against the available bytes before anything is
// allocated for them.
//
// # Errors
//
// Decode returns an error matching [ErrInvalidSignature] when the input is not
// a Binary Cookies file and an error matching [ErrMalformed] when it is a
// damaged one; a truncated file also matches [io.ErrUnexpectedEOF]. Errors from
// the reader are wrapped. Use [errors.Is] to classify errors; their messages
// may change.
//
// # Encoding
//
// Encode is deterministic, and decoding its output reproduces the encoded file
// with times at one-second precision. It rejects values the format cannot
// represent: strings containing NUL, times outside the years 1 to 9999 and
// sizes that do not fit in 32 bits.
//
// Decode and Encode are safe for concurrent use.
//
// # Compatibility
//
// Within major version 2, the exported API and the behavior described here
// change only in backward-compatible ways. Fields may be added to the structs,
// so write composite literals with field names. Cookie stays comparable with ==.
package binarycookies
