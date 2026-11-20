package binarycookies

import (
	"math"
	"time"
)

const (
	fileSignature = "cook"
	pageStartTag  = "\x00\x00\x01\x00"
	pageEndTag    = "\x00\x00\x00\x00"
	footerTag     = "\x07\x17\x20\x05"

	fileHeaderSize = len(fileSignature) + 4
	footerSize     = 4 + len(footerTag)

	pageCountAt    = len(pageStartTag)
	pageOffsetsAt  = pageCountAt + 4
	pageHeaderSize = pageOffsetsAt + len(pageEndTag) // excluding the cookie offset table

	recFlags         = 8
	recDomainOffset  = 16
	recNameOffset    = 20
	recPathOffset    = 24
	recValueOffset   = 28
	recCommentOffset = 32
	recExpires       = 40
	recCreated       = 48
	recordHeaderSize = recCreated + 8

	macEpochUnix   = 978307200    // 2001-01-01T00:00:00Z
	minUnixSeconds = -62135596800 // 0001-01-01T00:00:00Z
	maxUnixSeconds = 253402300799 // 9999-12-31T23:59:59Z
)

func timeFromMac(seconds float64) time.Time {
	unix := math.Trunc(seconds + macEpochUnix)
	if math.IsNaN(unix) || unix < minUnixSeconds || unix > maxUnixSeconds {
		return time.Time{}
	}
	return time.Unix(int64(unix), 0).UTC()
}

func timeToMac(t time.Time) (seconds float64, ok bool) {
	unix := t.Unix()
	if unix < minUnixSeconds || unix > maxUnixSeconds {
		return 0, false
	}
	return float64(unix - macEpochUnix), true
}

func pageChecksum(page []byte) uint32 {
	var sum uint32
	for i := 0; i < len(page); i += 4 {
		sum += uint32(page[i])
	}
	return sum
}
