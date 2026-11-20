package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// loadFile reads and parses a JSON configuration file.
func loadFile(path string) (map[string]json.RawMessage, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file %q: %w", path, err)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, fmt.Errorf("parse config file %q: %w", path, err)
	}

	return fields, nil
}
