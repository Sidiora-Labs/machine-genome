package mgs

import (
	"encoding/json"
	"fmt"
)

func unmarshal(raw []byte, target any) error {
	if _, err := Canonicalize(raw); err != nil {
		return err
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("decode record: %w", err)
	}
	return nil
}

func pretty(object any) ([]byte, error) {
	raw, err := json.MarshalIndent(object, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode record: %w", err)
	}
	return append(raw, '\n'), nil
}
