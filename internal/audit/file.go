package audit

import (
	"encoding/json"
	"os"
	"sync"
)

type FileSink struct {
	mu   sync.Mutex
	path string
}

func NewFileSink(path string) *FileSink {
	return &FileSink{path: path}
}

func (f *FileSink) Write(ev Event) error {
	data, err := json.Marshal(ev)
	if err != nil {
		return err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	fh, err := os.OpenFile(f.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}

	if _, err := fh.Write(append(data, '\n')); err != nil {
		fh.Close()
		return err
	}
	return fh.Close()
}
