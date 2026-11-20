package binarycookies_test

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/moond4rk/binarycookies/v2"
)

func ExampleDecode() {
	data, err := os.ReadFile("testdata/vectors/apple.binarycookies")
	if err != nil {
		log.Fatal(err)
	}
	file, err := binarycookies.Decode(bytes.NewReader(data))
	if err != nil {
		log.Fatal(err)
	}
	for _, c := range file.Cookies()[:3] {
		fmt.Println(c.Domain, c.Name, c.Value, c.Expires.Format(time.RFC3339))
	}
	// Output:
	// .apple.com dssid2 b267acef-b91e-4a5e-8f15-54be2c037b1c 2014-04-02T21:56:02Z
	// .apple.com pxro 1 2015-04-02T21:56:02Z
	// .apple.com s_invisit_n2_us 3 2015-04-02T21:55:59Z
}

func ExampleEncode() {
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
	decoded, err := binarycookies.Decode(&buf)
	if err != nil {
		log.Fatal(err)
	}
	c := decoded.Cookies()[0]
	fmt.Println(c.Name, c.Value, c.Secure(), c.HTTPOnly(), c.Expires)
	// Output: session abc123 true true 2030-01-01 00:00:00 +0000 UTC
}
