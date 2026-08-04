package test

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/urdadx/nukri/internal/core"
	"github.com/urdadx/nukri/internal/file_info"
	"github.com/urdadx/nukri/internal/preview"
	"github.com/urdadx/nukri/internal/preview/terminalimage"
)

func TestManualKittyImagePreview(t *testing.T) {
	if os.Getenv("NUKRI_KITTY_PREVIEW") != "1" {
		t.Skip("set NUKRI_KITTY_PREVIEW=1 to display the sample image")
	}
	if !terminalimage.IsKittyTerminal() {
		t.Skip("manual image preview requires a Kitty terminal")
	}

	path := filepath.Join("..", "..", "test", "file_samples", "video_sample.mp4")
	facts := fileinfo.InspectPath(path, core.File)
	result, err := preview.NewService().Render(context.Background(), preview.Request{Path: path, Facts: facts})
	if err != nil {
		t.Fatal(err)
	}
	video, ok := result.(*preview.VideoPreview)
	if !ok {
		t.Fatalf("result = %T, want *preview.VideoPreview", result)
	}

	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("open terminal: %v", err)
	}
	defer tty.Close()

	renderer := terminalimage.NewKitty(tty)
	columns := 50
	rows := max(1, min(20, video.Frame.Height*columns/(max(video.Frame.Width, 1)*2)))

	// Use the alternate screen so the manual preview does not damage shell output.
	if _, err := fmt.Fprint(tty, "\x1b[?1049h\x1b[2J\x1b[H"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = renderer.DeleteAll()
		_, _ = fmt.Fprint(tty, "\x1b[?1049l")
	}()

	placed, err := renderer.Place(video.Frame, terminalimage.Placement{
		X: 2, Y: 1, Columns: columns, Rows: rows,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer renderer.Delete(placed)

	if _, err := fmt.Fprintf(tty, "\x1b[%d;3Hvideo_sample.mp4 thumbnail (%dx%d) - press Enter to close", rows+3, video.Frame.Width, video.Frame.Height); err != nil {
		t.Fatal(err)
	}
	if _, err := bufio.NewReader(tty).ReadString('\n'); err != nil {
		t.Fatalf("wait for input: %v", err)
	}
}
