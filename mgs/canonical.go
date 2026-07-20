package mgs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	jsoncanonicalizer "github.com/cyberphone/json-canonicalization/go/src/webpki.org/jsoncanonicalizer"
)

var ErrInvalidJSON = errors.New("invalid canonical JSON input")

// Canonicalize validates an I-JSON object and returns its RFC 8785 JCS form.
// Validation happens before canonicalization so duplicate keys and negative zero
// cannot collapse into a different semantic value.
func Canonicalize(raw []byte) ([]byte, error) {
	if !utf8.Valid(raw) {
		return nil, fmt.Errorf("%w: input is not valid UTF-8", ErrInvalidJSON)
	}
	if err := validateUnicodeEscapes(raw); err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	top, err := scanValue(dec, "$")
	if err != nil {
		return nil, err
	}
	if top != '{' {
		return nil, fmt.Errorf("%w: top-level value must be an object", ErrInvalidJSON)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, fmt.Errorf("%w: trailing JSON value", ErrInvalidJSON)
		}
		return nil, fmt.Errorf("%w: trailing data: %v", ErrInvalidJSON, err)
	}
	canonical, err := jsoncanonicalizer.Transform(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: JCS: %v", ErrInvalidJSON, err)
	}
	return canonical, nil
}

func validateUnicodeEscapes(raw []byte) error {
	inString := false
	for index := 0; index < len(raw); index++ {
		switch raw[index] {
		case '"':
			inString = !inString
		case '\\':
			if !inString {
				continue
			}
			index++
			if index >= len(raw) {
				return fmt.Errorf("%w: truncated string escape", ErrInvalidJSON)
			}
			if raw[index] != 'u' {
				continue
			}
			codepoint, err := parseHexQuad(raw, index+1)
			if err != nil {
				return err
			}
			index += 4
			switch {
			case codepoint >= 0xd800 && codepoint <= 0xdbff:
				if index+6 >= len(raw) || raw[index+1] != '\\' || raw[index+2] != 'u' {
					return fmt.Errorf("%w: unpaired high Unicode surrogate", ErrInvalidJSON)
				}
				low, err := parseHexQuad(raw, index+3)
				if err != nil {
					return err
				}
				if low < 0xdc00 || low > 0xdfff {
					return fmt.Errorf("%w: high Unicode surrogate is not followed by a low surrogate", ErrInvalidJSON)
				}
				index += 6
			case codepoint >= 0xdc00 && codepoint <= 0xdfff:
				return fmt.Errorf("%w: unpaired low Unicode surrogate", ErrInvalidJSON)
			}
		}
	}
	return nil
}

func parseHexQuad(raw []byte, start int) (uint16, error) {
	if start+4 > len(raw) {
		return 0, fmt.Errorf("%w: truncated Unicode escape", ErrInvalidJSON)
	}
	var value uint16
	for _, char := range raw[start : start+4] {
		value <<= 4
		switch {
		case char >= '0' && char <= '9':
			value |= uint16(char - '0')
		case char >= 'a' && char <= 'f':
			value |= uint16(char-'a') + 10
		case char >= 'A' && char <= 'F':
			value |= uint16(char-'A') + 10
		default:
			return 0, fmt.Errorf("%w: invalid Unicode escape", ErrInvalidJSON)
		}
	}
	return value, nil
}

func scanValue(dec *json.Decoder, path string) (json.Delim, error) {
	tok, err := dec.Token()
	if err != nil {
		return 0, fmt.Errorf("%w at %s: %v", ErrInvalidJSON, path, err)
	}
	switch value := tok.(type) {
	case json.Delim:
		switch value {
		case '{':
			seen := make(map[string]struct{})
			for dec.More() {
				keyToken, err := dec.Token()
				if err != nil {
					return 0, fmt.Errorf("%w at %s: %v", ErrInvalidJSON, path, err)
				}
				key, ok := keyToken.(string)
				if !ok {
					return 0, fmt.Errorf("%w at %s: object key is not a string", ErrInvalidJSON, path)
				}
				if _, exists := seen[key]; exists {
					return 0, fmt.Errorf("%w at %s: duplicate member %q", ErrInvalidJSON, path, key)
				}
				seen[key] = struct{}{}
				if _, err := scanValue(dec, path+"."+key); err != nil {
					return 0, err
				}
			}
			end, err := dec.Token()
			if err != nil || end != json.Delim('}') {
				return 0, fmt.Errorf("%w at %s: unterminated object", ErrInvalidJSON, path)
			}
		case '[':
			for index := 0; dec.More(); index++ {
				if _, err := scanValue(dec, fmt.Sprintf("%s[%d]", path, index)); err != nil {
					return 0, err
				}
			}
			end, err := dec.Token()
			if err != nil || end != json.Delim(']') {
				return 0, fmt.Errorf("%w at %s: unterminated array", ErrInvalidJSON, path)
			}
		default:
			return 0, fmt.Errorf("%w at %s: unexpected delimiter %q", ErrInvalidJSON, path, value)
		}
		return value, nil
	case json.Number:
		number := value.String()
		f, err := strconv.ParseFloat(number, 64)
		if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
			return 0, fmt.Errorf("%w at %s: number %q is outside binary64", ErrInvalidJSON, path, number)
		}
		if strings.HasPrefix(number, "-") && f == 0 {
			return 0, fmt.Errorf("%w at %s: negative zero is forbidden", ErrInvalidJSON, path)
		}
	}
	return 0, nil
}

func decodeObject(raw []byte) (map[string]any, error) {
	if _, err := Canonicalize(raw); err != nil {
		return nil, err
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}
	return object, nil
}

func encodeObject(object map[string]any) ([]byte, error) {
	raw, err := json.Marshal(object)
	if err != nil {
		return nil, fmt.Errorf("encode JSON object: %w", err)
	}
	return Canonicalize(raw)
}
