package test

import (
	"bytes"
	"encoding/base64"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/urdadx/nukri/internal/kittydnd"
)

func TestDropOfferNegotiation(t *testing.T) {
	var parser kittydnd.Parser
	event, ok := parser.Parse([]byte("\x1b]72;t=m:x=2:y=3:o=3;text/plain text/uri-list\x1b\\"))
	if !ok || event.Kind != kittydnd.DropOffer || event.Final || event.MIME != 2 || event.Operation != kittydnd.Either {
		t.Fatalf("unexpected hover offer: %+v, %v", event, ok)
	}
	event, ok = parser.Parse([]byte("\x1b]72;t=M:x=2:y=3:o=1;text/uri-list\x1b\\"))
	if !ok || event.Kind != kittydnd.DropOffer || !event.Final || event.MIME != 1 || event.Operation != kittydnd.Copy {
		t.Fatalf("unexpected final offer: %+v, %v", event, ok)
	}
}

func TestChunkedDropDataParsesLocalURIList(t *testing.T) {
	payload := base64.RawStdEncoding.EncodeToString([]byte("# files\r\nfile:///tmp/a%20b.txt\r\nfile://localhost/tmp/c.txt\r\n"))
	cut := 7
	var parser kittydnd.Parser
	if _, ok := parser.Parse([]byte("\x1b]72;t=r:x=1:m=1;" + payload[:cut] + "\x1b\\")); ok {
		t.Fatal("first chunk unexpectedly produced an event")
	}
	if _, ok := parser.Parse([]byte("\x1b]72;m=1;" + payload[cut:] + "\x1b\\")); ok {
		t.Fatal("continuation unexpectedly produced an event")
	}
	event, ok := parser.Parse([]byte("\x1b]72;t=r:x=1:m=0;\x1b\\"))
	if !ok || event.Kind != kittydnd.DropData || event.MIME != 1 {
		t.Fatalf("unexpected data event: %+v, %v", event, ok)
	}
	want := []string{"/tmp/a b.txt", "/tmp/c.txt"}
	if !reflect.DeepEqual(event.Paths, want) {
		t.Fatalf("paths = %#v, want %#v", event.Paths, want)
	}
}

func TestDropDataReportsRemoteAndUnsupportedURIs(t *testing.T) {
	payload := base64.RawStdEncoding.EncodeToString([]byte("file://remote/tmp/a\nhttps://example.com/a\n"))
	var parser kittydnd.Parser
	event, ok := parser.Parse([]byte("\x1b]72;t=r:x=1:m=0;" + payload + "\x1b\\"))
	if !ok || event.Kind != kittydnd.DropData {
		t.Fatalf("unexpected data event: %+v, %v", event, ok)
	}
	want := []string{"file", "https"}
	if !reflect.DeepEqual(event.UnsupportedSchemes, want) {
		t.Fatalf("schemes = %#v, want %#v", event.UnsupportedSchemes, want)
	}
}

func TestDropResponseSequences(t *testing.T) {
	if got := kittydnd.EnableSequence(); got != "\x1b]72;t=a;text/uri-list\x1b\\\x1b]72;t=o:x=1\x1b\\" {
		t.Fatalf("enable sequence = %q", got)
	}
	if got := kittydnd.AcceptDropSequence(kittydnd.Either); got != "\x1b]72;t=m:o=1;text/uri-list\x1b\\" {
		t.Fatalf("accept sequence = %q", got)
	}
	if got := kittydnd.RequestDropDataSequence(2); got != "\x1b]72;t=r:x=2\x1b\\" {
		t.Fatalf("request sequence = %q", got)
	}
	if got := kittydnd.FinishDropSequence(kittydnd.Copy); got != "\x1b]72;t=r:o=1\x1b\\" {
		t.Fatalf("finish sequence = %q", got)
	}
}

// An OSC 72 introducer that never terminates must not swallow keyboard input
// forever: once it outgrows any valid drop it is dropped and input flows again.
func TestFilterInputRecoversFromUnterminatedSequence(t *testing.T) {
	junk := strings.Repeat("A", 23<<20) // longer than any valid drop sequence
	src := strings.NewReader("ab\x1b]72;" + junk + "q")
	var dst bytes.Buffer
	events := make(chan kittydnd.Event, 1)
	kittydnd.FilterInput(src, &dst, events)
	output := dst.String()
	if !strings.HasPrefix(output, "ab") || !strings.HasSuffix(output, "q") {
		t.Fatalf("keyboard input around the sequence was lost: %d bytes, ends %q", len(output), output[max(0, len(output)-5):])
	}
}

func TestFilterInputExtractsSequenceSplitAcrossReads(t *testing.T) {
	sequence := kittydnd.RejectDropSequence() // any well-formed OSC 72 sequence
	src := io.MultiReader(strings.NewReader("x"+sequence[:3]), strings.NewReader(sequence[3:]+"y"))
	var dst bytes.Buffer
	kittydnd.FilterInput(src, &dst, make(chan kittydnd.Event, 4))
	if dst.String() != "xy" {
		t.Fatalf("output = %q, want the sequence removed", dst.String())
	}
}

// A message arriving between drop data chunks must neither end the drop nor
// become part of its data.
func TestOtherMessageDuringChunkedDropIsHandledSeparately(t *testing.T) {
	payload := base64.RawStdEncoding.EncodeToString([]byte("file:///tmp/a.txt\r\nfile:///tmp/b.txt"))
	var parser kittydnd.Parser
	if _, ok := parser.Parse([]byte("\x1b]72;t=r:x=1:m=1;" + payload[:10] + "\x1b\\")); ok {
		t.Fatal("first chunk unexpectedly produced an event")
	}
	event, ok := parser.Parse([]byte("\x1b]72;t=m:x=-1:y=-1\x1b\\"))
	if !ok || event.Kind != kittydnd.DropLeave {
		t.Fatalf("message during drop = %+v, %v, want DropLeave", event, ok)
	}
	event, ok = parser.Parse([]byte("\x1b]72;m=0;" + payload[10:] + "\x1b\\"))
	if !ok || event.Kind != kittydnd.DropData {
		t.Fatalf("last chunk = %+v, %v, want DropData", event, ok)
	}
	want := []string{"/tmp/a.txt", "/tmp/b.txt"}
	if !reflect.DeepEqual(event.Paths, want) {
		t.Fatalf("paths = %#v, want %#v", event.Paths, want)
	}
}

func TestDataErrorDuringChunkedDropEndsIt(t *testing.T) {
	var parser kittydnd.Parser
	parser.Parse([]byte("\x1b]72;t=r:x=1:m=1;ZmlsZTovLy90bXAv\x1b\\"))
	event, ok := parser.Parse([]byte("\x1b]72;t=R:x=1;EIO\x1b\\"))
	if !ok || event.Kind != kittydnd.DropDataError {
		t.Fatalf("error during drop = %+v, %v", event, ok)
	}
	// The next drop starts clean instead of extending the abandoned one.
	payload := base64.RawStdEncoding.EncodeToString([]byte("file:///tmp/c.txt"))
	event, ok = parser.Parse([]byte("\x1b]72;t=r:x=1:m=0;" + payload + "\x1b\\"))
	if !ok || !reflect.DeepEqual(event.Paths, []string{"/tmp/c.txt"}) {
		t.Fatalf("next drop = %+v, %v", event, ok)
	}
}

// The terminal sends the MIME list with the first move only, so later moves
// keep the offer, with the cell under the pointer.
func TestDropMovesWithoutMIMEListKeepOffer(t *testing.T) {
	var parser kittydnd.Parser
	parser.Parse([]byte("\x1b]72;t=m:x=1:y=1:o=1;text/plain text/uri-list\x1b\\"))
	event, ok := parser.Parse([]byte("\x1b]72;t=m:x=4:y=7:o=1\x1b\\"))
	if !ok || event.Kind != kittydnd.DropOffer || event.MIME != 2 || event.X != 4 || event.Y != 7 {
		t.Fatalf("later move = %+v, %v", event, ok)
	}
	parser.Parse([]byte("\x1b]72;t=m:x=-1:y=-1\x1b\\"))
	if event, _ := parser.Parse([]byte("\x1b]72;t=m:x=4:y=7:o=1\x1b\\")); event.Kind != kittydnd.DropUnsupported {
		t.Fatalf("move after leaving = %+v, want the old offer forgotten", event)
	}
}

// dropOfSize returns drop data sequences, in two chunks, for a URI list of
// exactly size bytes naming /tmp/a.
func dropOfSize(size int) [][]byte {
	uri := "file:///tmp/a\n#"
	encoded := base64.RawStdEncoding.EncodeToString([]byte(uri + strings.Repeat("x", size-len(uri))))
	half := len(encoded) / 2 / 4 * 4
	return [][]byte{
		[]byte("\x1b]72;t=r:x=1:m=1;" + encoded[:half] + "\x1b\\"),
		[]byte("\x1b]72;m=1;" + encoded[half:] + "\x1b\\"),
		[]byte("\x1b]72;t=r:x=1:m=0;\x1b\\"),
	}
}

func parseAll(sequences [][]byte) (kittydnd.Event, bool) {
	var parser kittydnd.Parser
	var event kittydnd.Event
	var ok bool
	for _, sequence := range sequences {
		if event, ok = parser.Parse(sequence); ok {
			break
		}
	}
	return event, ok
}

// The 16 MiB drop limit applies to the decoded data, not its base64 encoding,
// which is a third larger.
func TestDropSizeLimitIsOnDecodedData(t *testing.T) {
	event, ok := parseAll(dropOfSize(16 << 20))
	if !ok || event.Kind != kittydnd.DropData || !reflect.DeepEqual(event.Paths, []string{"/tmp/a"}) {
		t.Fatalf("16 MiB drop = %v %q, want its data", event.Kind, event.Error)
	}
	event, ok = parseAll(dropOfSize(16<<20 + 1))
	if !ok || event.Kind != kittydnd.DropDataError {
		t.Fatalf("drop over 16 MiB = %v, want an error", event.Kind)
	}
}
