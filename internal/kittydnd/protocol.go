package kittydnd

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const (
	uriListMIME = "text/uri-list"
	chunkSize   = 4096
)

type EventKind uint8

const (
	DragOffer EventKind = iota
	DragDataRequested
	DragEnded
	DragError
	DragOther
)

type Event struct {
	Kind EventKind
	X    int
	Y    int
	MIME int
}

func EnableSequence(machineID string) string {
	return fmt.Sprintf("\x1b]72;t=o:x=1;%s\x1b\\", machineID)
}

func DisableSequence() string { return "\x1b]72;t=o:x=2\x1b\\" }

func CancelSequence() string { return "\x1b]72;t=E:y=-1\x1b\\" }

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

func parseEvent(sequence []byte) (Event, bool) {
	body := strings.TrimSuffix(strings.TrimSuffix(string(sequence), "\x1b\\"), "\a")
	body = strings.TrimPrefix(body, "\x1b]72;")
	metadata, payload, _ := strings.Cut(body, ";")
	fields := strings.Split(metadata, ":")
	values := make(map[byte]int)
	var eventType byte
	for _, field := range fields {
		key, value, found := strings.Cut(field, "=")
		if !found || len(key) != 1 {
			continue
		}
		var parsed int
		if _, err := fmt.Sscanf(value, "%d", &parsed); err == nil {
			values[key[0]] = parsed
		}
		if key == "t" && len(value) == 1 {
			eventType = value[0]
		}
	}
	switch eventType {
	case 'o':
		return Event{Kind: DragOffer, X: values['x'], Y: values['y']}, true
	case 'e':
		switch values['x'] {
		case 4:
			return Event{Kind: DragEnded}, true
		case 5:
			return Event{Kind: DragDataRequested, MIME: values['y']}, true
		default:
			return Event{Kind: DragOther}, true
		}
	case 'E':
		if payload == "OK" {
			return Event{Kind: DragOther}, true
		}
		return Event{Kind: DragError}, true
	default:
		return Event{}, false
	}
}
