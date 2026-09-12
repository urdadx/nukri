package test

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/urdadx/nukri/internal/kittydnd"
)

func TestURIListPayloadEncodesLocalAbsolutePaths(t *testing.T) {
	directory := t.TempDir()
	file := filepath.Join(directory, "a b-é.txt")
	if err := os.WriteFile(file, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}

	payload := string(kittydnd.URIListPayload([]string{file, "relative", directory}))
	expectedFile := "file://" + strings.ReplaceAll(filepath.ToSlash(file), " ", "%20")
	expectedFile = strings.ReplaceAll(expectedFile, "é", "%C3%A9")
	expectedDirectory := "file://" + filepath.ToSlash(directory) + "/"
	if payload != expectedFile+"\r\n"+expectedDirectory {
		t.Fatalf("unexpected URI list: %q", payload)
	}
}

func TestStartSequenceOffersURIListAndStartsDrag(t *testing.T) {
	path := "/tmp/a b.txt"
	payload, sequence := kittydnd.StartSequence([]string{path}, " a b.txt")
	if string(payload) != "file:///tmp/a%20b.txt" {
		t.Fatalf("unexpected payload: %q", payload)
	}
	encoded := base64.RawStdEncoding.EncodeToString(payload)
	expected := "\x1b]72;t=o:o=3;text/uri-list\x1b\\" +
		"\x1b]72;t=p:x=0:m=0;" + encoded + "\x1b\\" +
		"\x1b]72;t=p:x=0:m=0;\x1b\\" +
		"\x1b]72;t=p:x=-1:y=0:X=6:Y=4:o=1024:m=0;74WbIGEgYi50eHQ\x1b\\" +
		"\x1b]72;t=P:x=-1\x1b\\"
	if sequence != expected {
		t.Fatalf("unexpected drag sequence: %q", sequence)
	}
}

func TestEmptyDragCancelsAndUnknownDataIsRejected(t *testing.T) {
	payload, sequence := kittydnd.StartSequence([]string{"relative"}, "")
	if len(payload) != 0 || sequence != kittydnd.CancelSequence() {
		t.Fatalf("expected cancellation, got payload %q and sequence %q", payload, sequence)
	}
	if sequence := kittydnd.DataSequence(2, []byte("payload")); sequence != "\x1b]72;t=E:y=2;ENOENT\x1b\\" {
		t.Fatalf("unexpected rejection: %q", sequence)
	}
}
