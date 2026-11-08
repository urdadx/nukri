package preview

import (
	"errors"
	"fmt"
	"strconv"
)

/*
A minimal bencode reader for .torrent previews. Values are kept as raw byte
spans and decoded on demand, so the info dictionary's exact bytes are
available for hashing without re-encoding.
*/

const bencodeMaxDepth = 64

var errBencode = errors.New("invalid bencode")

// bencodeEnd returns the index just past the value starting at pos.
func bencodeEnd(data []byte, pos, depth int) (int, error) {
	if pos >= len(data) {
		return 0, fmt.Errorf("%w: unexpected end of data", errBencode)
	}
	if depth > bencodeMaxDepth {
		return 0, fmt.Errorf("%w: nesting too deep", errBencode)
	}
	switch c := data[pos]; {
	case c == 'i':
		end := indexByteFrom(data, pos+1, 'e')
		if end < 0 {
			return 0, fmt.Errorf("%w: unterminated integer", errBencode)
		}
		if _, err := parseBencodeInt(data[pos+1 : end]); err != nil {
			return 0, err
		}
		return end + 1, nil
	case c >= '0' && c <= '9':
		colon := indexByteFrom(data, pos, ':')
		if colon < 0 {
			return 0, fmt.Errorf("%w: unterminated string length", errBencode)
		}
		length, err := strconv.Atoi(string(data[pos:colon]))
		if err != nil || length < 0 || length > len(data)-colon-1 {
			return 0, fmt.Errorf("%w: bad string length", errBencode)
		}
		return colon + 1 + length, nil
	case c == 'l' || c == 'd':
		pos++
		for index := 0; ; index++ {
			if pos >= len(data) {
				return 0, fmt.Errorf("%w: unterminated container", errBencode)
			}
			isKey := c == 'd' && index%2 == 0
			if data[pos] == 'e' {
				if c == 'd' && !isKey {
					return 0, fmt.Errorf("%w: dictionary key without value", errBencode)
				}
				return pos + 1, nil
			}
			if isKey && (data[pos] < '0' || data[pos] > '9') {
				return 0, fmt.Errorf("%w: dictionary key is not a string", errBencode)
			}
			end, err := bencodeEnd(data, pos, depth+1)
			if err != nil {
				return 0, err
			}
			pos = end
		}
	default:
		return 0, fmt.Errorf("%w: unexpected byte %q", errBencode, c)
	}
}

func indexByteFrom(data []byte, from int, b byte) int {
	for i := from; i < len(data); i++ {
		if data[i] == b {
			return i
		}
	}
	return -1
}

func parseBencodeInt(digits []byte) (int64, error) {
	value, err := strconv.ParseInt(string(digits), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: bad integer %q", errBencode, digits)
	}
	return value, nil
}

// bencodeValidate checks that data holds exactly one well-formed value.
func bencodeValidate(data []byte) error {
	end, err := bencodeEnd(data, 0, 0)
	if err != nil {
		return err
	}
	if end != len(data) {
		return fmt.Errorf("%w: trailing data", errBencode)
	}
	return nil
}

// The decoders below expect raw to be one complete, validated value.

func bencodeInt(raw []byte) (int64, error) {
	if len(raw) < 2 || raw[0] != 'i' {
		return 0, fmt.Errorf("%w: expected integer", errBencode)
	}
	return parseBencodeInt(raw[1 : len(raw)-1])
}

func bencodeString(raw []byte) (string, error) {
	if len(raw) == 0 || raw[0] < '0' || raw[0] > '9' {
		return "", fmt.Errorf("%w: expected string", errBencode)
	}
	colon := indexByteFrom(raw, 0, ':')
	return string(raw[colon+1:]), nil
}

func bencodeList(raw []byte) ([][]byte, error) {
	if len(raw) == 0 || raw[0] != 'l' {
		return nil, fmt.Errorf("%w: expected list", errBencode)
	}
	items := [][]byte{}
	for pos := 1; raw[pos] != 'e'; {
		end, err := bencodeEnd(raw, pos, 0)
		if err != nil {
			return nil, err
		}
		items = append(items, raw[pos:end])
		pos = end
	}
	return items, nil
}

// bencodeDict returns the dictionary's values by key, plus its keys in order.
func bencodeDict(raw []byte) (map[string][]byte, []string, error) {
	if len(raw) == 0 || raw[0] != 'd' {
		return nil, nil, fmt.Errorf("%w: expected dictionary", errBencode)
	}
	values := map[string][]byte{}
	var keys []string
	for pos := 1; raw[pos] != 'e'; {
		keyEnd, err := bencodeEnd(raw, pos, 0)
		if err != nil {
			return nil, nil, err
		}
		key, err := bencodeString(raw[pos:keyEnd])
		if err != nil {
			return nil, nil, err
		}
		valueEnd, err := bencodeEnd(raw, keyEnd, 0)
		if err != nil {
			return nil, nil, err
		}
		if _, seen := values[key]; !seen {
			keys = append(keys, key)
		}
		values[key] = raw[keyEnd:valueEnd]
		pos = valueEnd
	}
	return values, keys, nil
}

func bencodeStrings(raw []byte) ([]string, error) {
	items, err := bencodeList(raw)
	if err != nil {
		return nil, err
	}
	values := make([]string, 0, len(items))
	for _, item := range items {
		value, err := bencodeString(item)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}
