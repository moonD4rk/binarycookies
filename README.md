# binarycookies

[![Tests](https://github.com/moonD4rk/binarycookies/actions/workflows/test.yml/badge.svg)](https://github.com/moonD4rk/binarycookies/actions/workflows/test.yml)
[![Lint](https://github.com/moonD4rk/binarycookies/actions/workflows/lint.yml/badge.svg)](https://github.com/moonD4rk/binarycookies/actions/workflows/lint.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/moond4rk/binarycookies/v2.svg)](https://pkg.go.dev/github.com/moond4rk/binarycookies/v2)
[![Go Report Card](https://goreportcard.com/badge/github.com/moond4rk/binarycookies)](https://goreportcard.com/report/github.com/moond4rk/binarycookies)
[![Go Version](https://img.shields.io/github/go-mod/go-version/moonD4rk/binarycookies)](go.mod)

Read and write Safari's `Cookies.binarycookies` files in pure Go.

`binarycookies` decodes and encodes the Binary Cookies format used by Safari and other WebKit-based applications on macOS and iOS. It powers the Safari cookie extraction in [HackBrowserData](https://github.com/moonD4rk/HackBrowserData).

## Features

- **Standard library only.** Builds with Go 1.20, the last release that supports Windows 7/8 and macOS 10.13/10.14, so programs that use it can still target those systems.
- **Tolerant decoding.** Every cookie that can be recovered is returned; decoding fails only when the structure of the file is broken.
- **Safe on crafted input.** Memory use stays proportional to the input size, and both the decoder and the encoder are fuzzed in CI.
- **Faithful encoding.** `Encode` writes the same layout as Safari: re-encoding the bundled Safari samples reproduces them byte for byte.

## Installation

```sh
go get github.com/moond4rk/binarycookies/v2
```

## Usage

### Read cookies

```go
package main

import (
	"bytes"
	"fmt"
	"log"
	"os"

	"github.com/moond4rk/binarycookies/v2"
)

func main() {
	data, err := os.ReadFile("Cookies.binarycookies")
	if err != nil {
		log.Fatal(err)
	}
	file, err := binarycookies.Decode(bytes.NewReader(data))
	if err != nil {
		log.Fatal(err)
	}
	for _, c := range file.Cookies() {
		fmt.Println(c.Domain, c.Name, c.Value, c.Expires, c.Secure(), c.HTTPOnly())
	}
}
```

### Handle errors

```go
file, err := binarycookies.Decode(r)
switch {
case errors.Is(err, binarycookies.ErrInvalidSignature):
	// not a Binary Cookies file
case errors.Is(err, io.ErrUnexpectedEOF):
	// truncated file (also matches ErrMalformed)
case errors.Is(err, binarycookies.ErrMalformed):
	// damaged file
case err != nil:
	// reading r failed
}
```

### Write a file

```go
file := &binarycookies.File{Pages: []binarycookies.Page{{
	Cookies: []binarycookies.Cookie{{
		Domain:  ".example.com",
		Name:    "session",
		Path:    "/",
		Value:   "abc123",
		Flags:   binarycookies.FlagSecure | binarycookies.FlagHTTPOnly,
		Expires: time.Date(2030, time.January, 1, 0, 0, 0, 0, time.UTC),
	}},
}}}
var buf bytes.Buffer
if err := binarycookies.Encode(&buf, file); err != nil {
	log.Fatal(err)
}
```

### Where Safari stores cookies

| Path                                                                                                              | Used by                             |
| ----------------------------------------------------------------------------------------------------------------- | ----------------------------------- |
| `~/Library/Containers/com.apple.Safari/Data/Library/Cookies/Cookies.binarycookies`                                 | Default profile, macOS 14 and later |
| `~/Library/Containers/com.apple.Safari/Data/Library/WebKit/WebsiteDataStore/<uuid>/Cookies/Cookies.binarycookies` | Named profiles, macOS 14 and later  |
| `~/Library/Cookies/Cookies.binarycookies`                                                                         | Earlier macOS versions              |
| `~/Library/Containers/<bundle-id>/Data/Library/Cookies/Cookies.binarycookies`                                     | Other sandboxed WebKit apps         |

## Behavior

| Input | Result |
| --- | --- |
| Not a Binary Cookies file | error matching `ErrInvalidSignature` |
| Truncated file, including a missing footer | error matching `ErrMalformed` and `io.ErrUnexpectedEOF` |
| Invalid page tag; a count, offset or size pointing outside its page or record; overlapping or out-of-order records | error matching `ErrMalformed` |
| Unknown header fields or flag bits | ignored; flag bits are kept in `Cookie.Flags` |
| Wrong footer checksum, data after the footer, gaps between records | ignored |
| Cookie larger than 4096 bytes | decoded normally |
| NaN, infinite, or out-of-range timestamp (outside the years 1 to 9999) | zero `time.Time` |

- Strings end at their first NUL byte or at the end of their record, so they never contain NUL. Bytes are not validated as UTF-8.
- Times are in UTC with one-second precision.
- `Encode` rejects strings containing NUL, times outside the years 1 to 9999 and sizes beyond 32 bits, and writes nothing in that case.
- `Decode` and `Encode` are safe for concurrent use. Error messages may change; classify errors with `errors.Is`.

The [package documentation](https://pkg.go.dev/github.com/moond4rk/binarycookies/v2) is the full reference.

## Compatibility

Version 2 follows semantic versioning: within `v2`, the exported API and the behavior above change only in backward-compatible ways. New fields may be added to the structs, so write struct literals with field names. `Cookie` stays comparable with `==`.

The module keeps building with Go 1.20 and takes no dependencies outside the standard library.

## Migrating from v1

```go
// v1
pages, err := binarycookies.New(f).Decode()
for _, page := range pages {
	for _, c := range page.Cookies {
		fmt.Println(string(c.Domain), string(c.Name), c.Secure, c.Creation)
	}
}

// v2
file, err := binarycookies.Decode(f)
for _, c := range file.Cookies() {
	fmt.Println(c.Domain, c.Name, c.Secure(), c.Created)
}
```

| v1 | v2 |
| --- | --- |
| `import "github.com/moond4rk/binarycookies"` | `import "github.com/moond4rk/binarycookies/v2"` |
| `New(r).Decode()` returns `[]Page` | `Decode(r)` returns `*File`; use `.Pages` or `.Cookies()` |
| `BinaryCookies`, `Page.Offsets`, `Cookie.Size`, `Cookie.String()` | removed |
| `Page.Length` | `len(page.Cookies)` |
| `Domain`, `Name`, `Path`, `Value`, `Comment` as `[]byte` | `string` |
| `Flags uint32` and `Secure`, `HTTPOnly` fields | `Flags Flags` and `Secure()`, `HTTPOnly()` methods |
| `Expires`, `Creation` in local time | `Expires`, `Created` in UTC; zero for invalid timestamps |
| unstructured errors | `ErrInvalidSignature`, `ErrMalformed`, `io.ErrUnexpectedEOF` via `errors.Is` |
| — | `Encode`, `FlagSecure`, `FlagHTTPOnly` |

Decoding is also more tolerant than in v1: a cookie larger than 4096 bytes no longer makes the whole file fail, records are located through their offsets, flags are read bit by bit, and the comment no longer keeps its NUL terminator.

## File format

Sizes and offsets are in bytes; BE and LE stand for big-endian and little-endian.

### File

| Offset | Size  | Type      | Description              |
| ------ | ----- | --------- | ------------------------ |
| 0      | 4     | bytes     | Signature `cook`         |
| 4      | 4     | BE uint32 | Number of pages `N`      |
| 8      | 4 × N | BE uint32 | Size of each page        |
| 8 + 4N |       |           | The pages, back to back  |
| …      | 4     | BE uint32 | Checksum                 |
| …      | 4     | bytes     | Footer tag `07 17 20 05` |
| …      |       |           | Optional trailing data   |

The checksum is the sum of every fourth byte of every page (page offsets 0, 4, 8, …), as a uint32. The optional trailing data is a BE uint32 size followed by a binary property list, for example holding `NSHTTPCookieAcceptPolicy`; this package ignores it.

### Page

| Offset  | Size  | Type      | Description                                             |
| ------- | ----- | --------- | ------------------------------------------------------- |
| 0       | 4     | bytes     | Start tag `00 00 01 00`                                 |
| 4       | 4     | LE uint32 | Number of cookies `M`                                   |
| 8       | 4 × M | LE uint32 | Offset of each cookie record from the start of the page |
| 8 + 4M  | 4     | bytes     | End tag `00 00 00 00`                                   |
| 12 + 4M |       |           | The cookie records                                      |

### Cookie record

| Offset | Size | Type       | Description                                             |
| ------ | ---- | ---------- | ------------------------------------------------------- |
| 0      | 4    | LE uint32  | Record size                                             |
| 4      | 4    |            | Unknown                                                 |
| 8      | 4    | LE uint32  | Flags: `0x1` Secure, `0x4` HttpOnly; other bits unknown |
| 12     | 4    |            | Unknown                                                 |
| 16     | 4    | LE uint32  | Domain offset                                           |
| 20     | 4    | LE uint32  | Name offset                                             |
| 24     | 4    | LE uint32  | Path offset                                             |
| 28     | 4    | LE uint32  | Value offset                                            |
| 32     | 4    | LE uint32  | Comment offset, 0 when there is no comment              |
| 36     | 4    |            | Unknown, usually 0                                      |
| 40     | 8    | LE float64 | Expiration time, in seconds since 2001-01-01T00:00:00Z  |
| 48     | 8    | LE float64 | Creation time, in seconds since 2001-01-01T00:00:00Z    |
| 56     |      |            | NUL-terminated strings at the offsets above             |

String offsets are relative to the start of the record. Newer files may store extra data after the value's NUL terminator inside the record, which is why strings are read up to their first NUL rather than up to the next offset.

## Development

```sh
go test -race ./...                                    # unit, golden and example tests
go test -run TestGolden -update                        # rewrite testdata/golden after an intended change
go test -run '^$' -fuzz '^FuzzDecode$' -fuzztime 1m .  # or FuzzRoundTrip
golangci-lint run                                      # v2.15 or later
```

- `testdata/vectors` holds files written by Safari, plus a file that is not a cookie file.
- `testdata/golden` holds the decoded form of each Safari file. Other implementations can check themselves against it, so a change there is a change in behavior.
- `testdata/fuzz/FuzzDecode` holds inputs found by fuzzing; every `go test` run replays them.

CI runs the tests on Linux, macOS and Windows with Go 1.20 and the latest Go, on linux/386, and fuzzes each target for 30 seconds.

## License

MIT. Originally written by CIXTOR and maintained by moonD4rk; see [LICENSE.md](LICENSE.md).
