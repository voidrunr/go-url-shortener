package audit

import (
	"encoding/json"
	"os"
	"sync"
)

// FileSink appends audit events to a file as JSON lines.
//
// The file is opened lazily on the first Write and kept open between writes.
// Each event is encoded and written in one call; no bufio layer is used.
type FileSink struct {
	mu     sync.Mutex
	path   string
	file   *os.File
	enc    *json.Encoder
	closed bool
}

// NewFileSink returns a FileSink that appends events to path. The file is
// created if it does not exist and is opened on the first Write.
func NewFileSink(path string) *FileSink {
	return &FileSink{path: path}
}

// Write appends ev to the file as a single JSON line. Write is safe for
// concurrent use. If a write fails, the underlying file is closed and reopened
// by the next Write.
func (f *FileSink) Write(ev Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return os.ErrClosed
	}

	if f.file == nil {
		file, err := os.OpenFile(f.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		f.file = file
		f.enc = json.NewEncoder(file)
	}

	if err := f.enc.Encode(ev); err != nil {
		_ = f.file.Close()
		f.file = nil
		f.enc = nil
		return err
	}
	return nil
}

// Close closes the underlying file. It is safe to call Close more than once.
// Write returns os.ErrClosed after Close.
func (f *FileSink) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return nil
	}
	f.closed = true
	if f.file == nil {
		return nil
	}

	err := f.file.Close()
	f.file = nil
	f.enc = nil
	return err
}
