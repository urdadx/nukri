package kittydnd

import (
	"bytes"
	"io"
	"os"
	"regexp"
	"time"
)

type Runtime struct {
	Enabled   bool
	MachineID string
}

/*
DetectRuntime reports whether drag and drop may be used here. Inside tmux,
zellij or over SSH it is off: multiplexers don't pass the protocol through,
and dropped file paths would name files on another machine. Whether the
terminal supports the protocol is asked by OpenInput.
*/
func DetectRuntime() Runtime {
	if os.Getenv("TMUX") != "" || os.Getenv("ZELLIJ") != "" || os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_TTY") != "" {
		return Runtime{}
	}
	return Runtime{Enabled: true}
}

// probeTimeout bounds the wait for a terminal that answers neither probe query.
const probeTimeout = time.Second

// ProbeSequence asks the terminal whether it supports the drag-and-drop
// protocol, then for its primary device attributes, which every terminal
// answers.
func ProbeSequence() string { return "\x1b]72;t=q\x1b\\\x1b[c" }

var (
	queryReplyIntroducer = []byte("\x1b]72;t=q")
	deviceAttributes     = regexp.MustCompile(`\x1b\[\?[0-9;]*c`)
)

/*
Probe reads the terminal's answers to ProbeSequence from r. The terminal
supports drag and drop if it answers the query before the device attributes.
Probe returns once the device attributes arrive or r fails, with any other
input read meanwhile, such as keys typed during startup, which belongs to the
program.
*/
func Probe(r io.Reader) (supported bool, rest []byte) {
	var buffer []byte
	chunk := make([]byte, 256)
	for {
		n, err := r.Read(chunk)
		buffer = append(buffer, chunk[:n]...)
		if attributes := deviceAttributes.FindIndex(buffer); attributes != nil {
			reply := queryReply(buffer)
			supported = reply != nil && reply[0] < attributes[0]
			return supported, without(buffer, reply, attributes)
		}
		if err != nil {
			reply := queryReply(buffer)
			return reply != nil, without(buffer, reply)
		}
	}
}

// queryReply returns the bounds of the terminal's complete answer to the
// protocol query in buffer, or nil.
func queryReply(buffer []byte) []int {
	start := bytes.Index(buffer, queryReplyIntroducer)
	if start < 0 {
		return nil
	}
	end := oscEnd(buffer[start:], 0)
	if end < 0 {
		return nil
	}
	return []int{start, start + end}
}

// without returns buffer minus the given [start, end) spans, which must not
// overlap. A nil span is skipped.
func without(buffer []byte, spans ...[]int) []byte {
	var kept []int
	for _, span := range spans {
		if span != nil {
			kept = append(kept, span...)
		}
	}
	if len(kept) == 4 && kept[2] < kept[0] {
		kept[0], kept[1], kept[2], kept[3] = kept[2], kept[3], kept[0], kept[1]
	}
	var rest []byte
	from := 0
	for index := 0; index < len(kept); index += 2 {
		rest = append(rest, buffer[from:kept[index]]...)
		from = kept[index+1]
	}
	return append(rest, buffer[from:]...)
}
