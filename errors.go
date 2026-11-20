package binarycookies

import (
	"errors"
	"fmt"
	"io"
)

var (
	// ErrInvalidSignature reports that the input is not a Binary Cookies file.
	ErrInvalidSignature = errors.New("binarycookies: invalid signature")

	// ErrMalformed reports a damaged Binary Cookies file.
	ErrMalformed = errors.New("binarycookies: malformed file")
)

func malformed(where string, err error) error {
	return fmt.Errorf("%w: %s: %w", ErrMalformed, where, err)
}

func truncated(where string) error {
	return malformed(where, io.ErrUnexpectedEOF)
}
