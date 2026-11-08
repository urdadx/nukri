// Package clipboard copies text to the system clipboard.
package clipboard

import (
	"encoding/base64"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
)

// command is a clipboard tool and when it applies.
type command struct {
	name    string
	args    []string
	display string // environment variable that must be set
}

// commands are tried in order; the first whose display is set and that is
// installed wins.
var commands = []command{
	{name: "wl-copy", display: "WAYLAND_DISPLAY"},
	{name: "xclip", args: []string{"-selection", "clipboard"}, display: "DISPLAY"},
	{name: "xsel", args: []string{"--clipboard", "--input"}, display: "DISPLAY"},
}

/*
Copy puts text on the clipboard with wl-copy, xclip or xsel when a display
server is available, and otherwise asks the terminal to do it with OSC 52,
written to terminal. That also covers SSH sessions. It returns the method used.
*/
func Copy(text string, terminal io.Writer) (string, error) {
	for _, candidate := range commands {
		if os.Getenv(candidate.display) == "" {
			continue
		}
		path, err := exec.LookPath(candidate.name)
		if err != nil {
			continue
		}
		cmd := exec.Command(path, candidate.args...)
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err == nil {
			return candidate.name, nil
		}
	}
	if terminal == nil {
		return "", errors.New("no clipboard tool found (install wl-clipboard or xclip)")
	}
	if _, err := io.WriteString(terminal, OSC52(text)); err != nil {
		return "", err
	}
	return "terminal", nil
}

// OSC52 is the escape sequence asking the terminal to set its clipboard.
func OSC52(text string) string {
	return "\x1b]52;c;" + base64.StdEncoding.EncodeToString([]byte(text)) + "\x07"
}
