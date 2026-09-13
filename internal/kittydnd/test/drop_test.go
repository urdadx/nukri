package test

import (
	"encoding/base64"
	"reflect"
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
	if got := kittydnd.AcceptDropSequence(kittydnd.Either); got != "\x1b]72;t=m:o=2;text/uri-list\x1b\\" {
		t.Fatalf("accept sequence = %q", got)
	}
	if got := kittydnd.RequestDropDataSequence(2); got != "\x1b]72;t=r:x=2\x1b\\" {
		t.Fatalf("request sequence = %q", got)
	}
	if got := kittydnd.FinishDropSequence(kittydnd.Copy); got != "\x1b]72;t=r:o=1\x1b\\" {
		t.Fatalf("finish sequence = %q", got)
	}
}
