package kittydnd

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	uriListMIME  = "text/uri-list"
	chunkSize    = 4096
	maxDropBytes = 16 << 20
)

type Operation uint8

const (
	Copy Operation = iota + 1
	Move
	Either
)

type EventKind uint8

const (
	DropOffer EventKind = iota
	DropLeave
	DropData
	DropDataError
	DropUnsupported
	DragOffer
	DragDataRequested
	DragEnded
	DragError
	DragOther
)

type Event struct {
	Kind               EventKind
	X, Y               int
	MIME               int
	Operation          Operation
	Final              bool
	Paths              []string
	UnsupportedSchemes []string
	Error              string
}

func EnableSequence(machineID string) string {
	return fmt.Sprintf("\x1b]72;t=a;%s\x1b\\\x1b]72;t=o:x=1;%s\x1b\\", uriListMIME, machineID)
}

func DisableSequence() string { return "\x1b]72;t=A\x1b\\\x1b]72;t=o:x=2\x1b\\" }
func CancelSequence() string  { return "\x1b]72;t=E:y=-1\x1b\\" }

func AcceptDropSequence(operation Operation) string {
	if operation == Either {
		operation = Move
	}
	return fmt.Sprintf("\x1b]72;t=m:o=%d;%s\x1b\\", operation, uriListMIME)
}

func RejectDropSequence() string { return "\x1b]72;t=m:o=0\x1b\\" }
func RequestDropDataSequence(mime int) string {
	return fmt.Sprintf("\x1b]72;t=r:x=%d\x1b\\", mime)
}
func FinishDropSequence(operation Operation) string {
	if operation != Copy && operation != Move {
		return "\x1b]72;t=r:o=0\x1b\\"
	}
	return fmt.Sprintf("\x1b]72;t=r:o=%d\x1b\\", operation)
}

func StartSequence(paths []string, label string) ([]byte, string) {
	payload := URIListPayload(paths)
	if len(payload) == 0 {
		return nil, CancelSequence()
	}
	sequence := "\x1b]72;t=o:o=3;" + uriListMIME + "\x1b\\"
	sequence += payloadSequence("t=p:x=0", payload, true)
	if label == "" {
		label = dragIconLabel(paths)
	}
	sequence += payloadSequence("t=p:x=-1:y=0:X=6:Y=4:o=1024", []byte(label), false)
	sequence += "\x1b]72;t=P:x=-1\x1b\\"
	return payload, sequence
}

func dragIconLabel(paths []string) string {
	if len(paths) == 1 {
		return filepath.Base(paths[0])
	}
	return fmt.Sprintf("%d selected files", len(paths))
}

func DataSequence(mimeIndex int, payload []byte) string {
	if mimeIndex != 0 || len(payload) == 0 {
		return fmt.Sprintf("\x1b]72;t=E:y=%d;ENOENT\x1b\\", mimeIndex)
	}
	return payloadSequence(fmt.Sprintf("t=e:y=%d", mimeIndex), payload, true)
}

func URIListPayload(paths []string) []byte {
	encoded := make([]string, 0, len(paths))
	for _, path := range paths {
		if !filepath.IsAbs(path) {
			continue
		}
		u := url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
		value := u.String()
		if info, err := os.Stat(path); err == nil && info.IsDir() && !strings.HasSuffix(value, "/") {
			value += "/"
		}
		encoded = append(encoded, value)
	}
	return []byte(strings.Join(encoded, "\r\n"))
}

func payloadSequence(metadata string, data []byte, finish bool) string {
	encoded := base64.RawStdEncoding.EncodeToString(data)
	var result strings.Builder
	for start := 0; start < len(encoded); start += chunkSize {
		end := min(start+chunkSize, len(encoded))
		more := 0
		if end < len(encoded) {
			more = 1
		}
		if start == 0 {
			fmt.Fprintf(&result, "\x1b]72;%s:m=%d;%s\x1b\\", metadata, more, encoded[start:end])
		} else {
			fmt.Fprintf(&result, "\x1b]72;m=%d;%s\x1b\\", more, encoded[start:end])
		}
	}
	if finish {
		fmt.Fprintf(&result, "\x1b]72;%s:m=0;\x1b\\", metadata)
	}
	return result.String()
}

type Parser struct {
	dropFields fields
	dropData   []byte
	dropActive bool
}

type fields struct {
	typeCode  byte
	x, y      int
	operation Operation
	more      *bool
}

func (p *Parser) Parse(sequence []byte) (Event, bool) {
	body := strings.TrimSuffix(strings.TrimSuffix(string(sequence), "\x1b\\"), "\a")
	body = strings.TrimPrefix(body, "\x1b]72;")
	metadata, payload, _ := strings.Cut(body, ";")
	parsed := parseFields(metadata)
	if len(payload) > maxDropBytes {
		p.reset()
		return Event{Kind: DropDataError, Error: "drop data is too large"}, true
	}
	if p.dropActive {
		if len(p.dropData)+len(payload) > maxDropBytes {
			p.reset()
			return Event{Kind: DropDataError, Error: "drop data is too large"}, true
		}
		p.dropData = append(p.dropData, payload...)
		if parsed.more != nil && *parsed.more {
			return Event{}, false
		}
		fields, data := p.dropFields, p.dropData
		p.reset()
		return dropDataEvent(fields.x, data)
	}
	if parsed.typeCode == 'r' && (parsed.more != nil && *parsed.more || parsed.more == nil && payload != "") {
		p.dropActive, p.dropFields = true, parsed
		p.dropData = append(p.dropData, payload...)
		return Event{}, false
	}
	return eventFromParts(parsed, payload)
}

func (p *Parser) reset() {
	p.dropFields, p.dropData, p.dropActive = fields{}, nil, false
}

func parseFields(metadata string) fields {
	result := fields{x: -1, y: -1}
	for _, field := range strings.Split(metadata, ":") {
		key, value, found := strings.Cut(field, "=")
		if !found || len(key) != 1 {
			continue
		}
		if key == "t" && len(value) == 1 {
			result.typeCode = value[0]
			continue
		}
		parsed, err := strconv.Atoi(value)
		if err != nil {
			continue
		}
		switch key {
		case "x":
			result.x = parsed
		case "y":
			result.y = parsed
		case "o":
			result.operation = Operation(parsed)
		case "m":
			more := parsed != 0
			result.more = &more
		}
	}
	return result
}

func eventFromParts(fields fields, payload string) (Event, bool) {
	switch fields.typeCode {
	case 'm', 'M':
		if fields.typeCode == 'm' && fields.x == -1 && fields.y == -1 {
			return Event{Kind: DropLeave}, true
		}
		mime := 0
		for index, offered := range strings.Fields(payload) {
			if offered == uriListMIME {
				mime = index + 1
				break
			}
		}
		if mime == 0 || fields.operation < Copy || fields.operation > Either {
			return Event{Kind: DropUnsupported, Final: fields.typeCode == 'M'}, true
		}
		return Event{Kind: DropOffer, MIME: mime, Operation: fields.operation, Final: fields.typeCode == 'M'}, true
	case 'r':
		if payload == "" && fields.more != nil && !*fields.more {
			return Event{}, false
		}
		return dropDataEvent(fields.x, []byte(payload))
	case 'R':
		return Event{Kind: DropDataError, MIME: fields.x, Error: payload}, true
	case 'o':
		if fields.x < 0 || fields.y < 0 {
			return Event{}, false
		}
		return Event{Kind: DragOffer, X: fields.x, Y: fields.y}, true
	case 'e':
		switch fields.x {
		case 4:
			return Event{Kind: DragEnded}, true
		case 5:
			return Event{Kind: DragDataRequested, MIME: fields.y}, true
		default:
			return Event{Kind: DragOther}, true
		}
	case 'E':
		if payload == "OK" {
			return Event{Kind: DragOther}, true
		}
		return Event{Kind: DragError, Error: payload}, true
	default:
		return Event{}, false
	}
}

func dropDataEvent(mime int, encoded []byte) (Event, bool) {
	data, err := base64.StdEncoding.DecodeString(string(encoded))
	if err != nil {
		data, err = base64.RawStdEncoding.DecodeString(string(encoded))
	}
	if err != nil {
		return Event{Kind: DropDataError, MIME: mime, Error: "invalid drop data"}, true
	}
	paths, unsupported := parseURIList(string(data))
	return Event{Kind: DropData, MIME: mime, Paths: paths, UnsupportedSchemes: unsupported}, true
}

func parseURIList(data string) ([]string, []string) {
	var paths, unsupported []string
	seenPaths, seenSchemes := map[string]bool{}, map[string]bool{}
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		u, err := url.Parse(line)
		if err != nil || !strings.EqualFold(u.Scheme, "file") || u.Host != "" && !strings.EqualFold(u.Host, "localhost") {
			scheme := "invalid"
			if err == nil && u.Scheme != "" {
				scheme = strings.ToLower(u.Scheme)
			}
			if !seenSchemes[scheme] {
				unsupported = append(unsupported, scheme)
				seenSchemes[scheme] = true
			}
			continue
		}
		path, err := url.PathUnescape(u.EscapedPath())
		if err != nil || strings.ContainsRune(path, 0) || !filepath.IsAbs(path) {
			continue
		}
		path = filepath.Clean(filepath.FromSlash(path))
		if !seenPaths[path] {
			paths = append(paths, path)
			seenPaths[path] = true
		}
	}
	return paths, unsupported
}
