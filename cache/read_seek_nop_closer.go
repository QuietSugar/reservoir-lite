package cache

import (
	"bytes"
)

// readSeekNopCloser wraps a *bytes.Reader to implement io.ReadSeekCloser with a no-op Close.
// It also implements io.ReaderAt through embedding *bytes.Reader.
type readSeekNopCloser struct {
	*bytes.Reader
}

func (r *readSeekNopCloser) Close() error {
	return nil
}

func NewReadSeekNopCloser(data []byte) EntryData {
	return &readSeekNopCloser{
		Reader: bytes.NewReader(data),
	}
}
