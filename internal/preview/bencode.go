package preview

import (
	"errors"
	"fmt"
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
		length, ok := parseDigits(data[pos:colon])
		if !ok || length < 0 || length > int64(len(data)-colon-1) {
			return 0, fmt.Errorf("%w: bad string length", errBencode)
		}
		return colon + 1 + int(length), nil
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
	value, ok := parseDigits(digits)
	if !ok {
		return 0, fmt.Errorf("%w: bad integer %q", errBencode, digits)
	}
	return value, nil
}

// parseDigits parses an optionally negative decimal integer without the string
// conversion strconv needs, which allocated for every number in a torrent.
func parseDigits(digits []byte) (int64, bool) {
	negative := len(digits) > 0 && digits[0] == '-'
	if negative {
		digits = digits[1:]
	}
	if len(digits) == 0 || len(digits) > 19 {
		return 0, false
	}
	var value uint64
	for _, digit := range digits {
		if digit < '0' || digit > '9' {
			return 0, false
		}
		value = value*10 + uint64(digit-'0')
	}
	if negative {
		if value > 1<<63 {
			return 0, false
		}
		return -int64(value), true
	}
	if value > 1<<63-1 {
		return 0, false
	}
	return int64(value), true
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
	for pos := 1; pos < len(raw) && raw[pos] != 'e'; {
		end, err := bencodeEnd(raw, pos, 0)
		if err != nil {
			return nil, err
		}
		items = append(items, raw[pos:end])
		pos = end
	}
	return items, nil
}

// bencodeEachEntry calls visit for each key and value of a dictionary, in
// order, without building a map.
func bencodeEachEntry(raw []byte, visit func(key, value []byte) error) error {
	if len(raw) == 0 || raw[0] != 'd' {
		return fmt.Errorf("%w: expected dictionary", errBencode)
	}
	for pos := 1; pos < len(raw) && raw[pos] != 'e'; {
		keyEnd, err := bencodeEnd(raw, pos, 0)
		if err != nil {
			return err
		}
		if raw[pos] < '0' || raw[pos] > '9' {
			return fmt.Errorf("%w: expected string", errBencode)
		}
		valueEnd, err := bencodeEnd(raw, keyEnd, 0)
		if err != nil {
			return err
		}
		colon := indexByteFrom(raw, pos, ':')
		if err := visit(raw[colon+1:keyEnd], raw[keyEnd:valueEnd]); err != nil {
			return err
		}
		pos = valueEnd
	}
	return nil
}

// bencodeDict returns the dictionary's values by key, plus its keys in order.
func bencodeDict(raw []byte) (map[string][]byte, []string, error) {
	values, keys, _, err := bencodeDictEnd(raw)
	return values, keys, err
}

// bencodeDictEnd is bencodeDict that also returns the index just past the
// dictionary. It checks bounds itself, so raw need not be validated first.
func bencodeDictEnd(raw []byte) (map[string][]byte, []string, int, error) {
	if len(raw) == 0 || raw[0] != 'd' {
		return nil, nil, 0, fmt.Errorf("%w: expected dictionary", errBencode)
	}
	values := map[string][]byte{}
	var keys []string
	pos := 1
	for pos < len(raw) && raw[pos] != 'e' {
		keyEnd, err := bencodeEnd(raw, pos, 0)
		if err != nil {
			return nil, nil, 0, err
		}
		key, err := bencodeString(raw[pos:keyEnd])
		if err != nil {
			return nil, nil, 0, err
		}
		valueEnd, err := bencodeEnd(raw, keyEnd, 0)
		if err != nil {
			return nil, nil, 0, err
		}
		if _, seen := values[key]; !seen {
			keys = append(keys, key)
		}
		values[key] = raw[keyEnd:valueEnd]
		pos = valueEnd
	}
	if pos >= len(raw) {
		return nil, nil, 0, fmt.Errorf("%w: unterminated dictionary", errBencode)
	}
	return values, keys, pos + 1, nil
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
