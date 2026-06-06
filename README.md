# Binary Cookies

[![Tests](https://github.com/moonD4rk/binarycookies/actions/workflows/test.yml/badge.svg)](https://github.com/moonD4rk/binarycookies/actions/workflows/test.yml)
[![Lint](https://github.com/moonD4rk/binarycookies/actions/workflows/lint.yml/badge.svg)](https://github.com/moonD4rk/binarycookies/actions/workflows/lint.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/moond4rk/binarycookies.svg)](https://pkg.go.dev/github.com/moond4rk/binarycookies)
[![Go Report Card](https://goreportcard.com/badge/github.com/moond4rk/binarycookies)](https://goreportcard.com/report/github.com/moond4rk/binarycookies)

A pure Go library for decoding the Binary Cookies file format used by Safari and other WebKit-based applications on macOS and iOS.

## Installation

```sh
go get github.com/moond4rk/binarycookies
```

Requires Go 1.20 or later.

## Usage

```go
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/moond4rk/binarycookies"
)

func main() {
	f, err := os.Open("Cookies.binarycookies")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	pages, err := binarycookies.New(f).Decode()
	if err != nil {
		log.Fatal(err)
	}

	for _, page := range pages {
		for _, cookie := range page.Cookies {
			fmt.Println(cookie)
		}
	}
}
```

Common cookie file locations on macOS:

```sh
# Safari cookies
~/Library/Cookies/Cookies.binarycookies

# Containerized app cookies
~/Library/Containers/<APP_ID>/Data/Library/Cookies/Cookies.binarycookies
```

## Specification

Binary Cookies are binary files containing several pieces of data that together form an array of objects representing persistent web cookies for different applications in the macOS and iOS application ecosystem.

**Note:** BE stands for Big-endian and LE stands for Little-endian.

| Variable | Size | Type | Description |
|----------|------|------|-------------|
| signature | 4 | byte | File signature must be equal to `[]byte{0x63, 0x6f, 0x6f, 0x6b}` or `String("cook")` |
| numPages | 4 | BE_uint32 | Number of pages in the file |
| pageOffset | 4 | BE_uint32 | Page offset. Repeat this N times where `N = numPages` |
| pageStart | 4 | byte | Marks the beginning of a page. Must be equal to `[]byte{0x00, 0x00, 0x01, 0x00}` |
| numCookies | 4 | LE_uint32 | Number of cookies in the page |
| cookieOffset | 4 | LE_uint32 | Cookie offset. Repeat this N times where `N = numCookies` |
| pageEnd | 4 | byte | Marks the end of a page. Must be equal to `[]byte{0x00, 0x00, 0x00, 0x00}` |

Immediately after `pageEnd` we can read the page cookies. Repeat the steps below N times where `N = numCookies`.

| Variable | Size | Type | Description |
|----------|------|------|-------------|
| cookieSize | 4 | LE_uint32 | Cookie size. Number of bytes associated to the cookie |
| unknownOne | 4 | byte | Unknown field possibly related to the cookie flags |
| cookieFlags | 4 | LE_uint32 | `0x0:None` - `0x1:Secure` - `0x4:HttpOnly` - `0x5:Secure+HttpOnly` |
| unknownTwo | 4 | byte | Unknown field possibly related to the cookie flags |
| domainOffset | 4 | LE_uint32 | Cookie domain offset |
| nameOffset | 4 | LE_uint32 | Cookie name offset |
| pathOffset | 4 | LE_uint32 | Cookie path offset |
| valueOffset | 4 | LE_uint32 | Cookie value offset |
| commentOffset | 4 | LE_uint32 | Cookie comment offset |
| endHeader | 4 | byte | Marks the end of a header. Must be equal to `[]byte{0x00, 0x00, 0x00, 0x00}` |
| expires | 8 | LE_float64 | Cookie expiration time in Mac epoch time. Add 978307200 to turn into Unix |
| creation | 8 | LE_float64 | Cookie creation time in Mac epoch time. Add 978307200 to turn into Unix |
| comment | N | string | Cookie comment string. `N = domainOffset - commentOffset` |
| domain | N | string | Cookie domain string. `N = nameOffset - domainOffset` |
| name | N | string | Cookie name string. `N = pathOffset - nameOffset` |
| path | N | string | Cookie path string. `N = valueOffset - pathOffset` |
| value | N | string | Cookie value string. `N = cookieSize - valueOffset` (truncated at the first NUL byte) |

Immediately after the last cookie in the page we can read another page with `pageStart`.

The last cookie of the last page in the file is followed by an 8-bytes checksum.

An optional number of bytes follow the checksum, these are part of a [Binary Property List](https://en.wikipedia.org/wiki/Property_list) that contains a dictionary with additional information like the cookie accept policy for all tasks within sessions based on the software configuration. A `bplist00` file is a completely different file format we need to decode separately. The first 4-bytes after the checksum are the BE_uint32 representing the size of the binary property list. The remaining bytes represent the data we need to decode using a bplist parser.

## License

MIT
