package kittydnd

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type Runtime struct {
	Enabled   bool
	MachineID string
}

func DetectRuntime() Runtime {
	if os.Getenv("TMUX") != "" || os.Getenv("ZELLIJ") != "" || os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_TTY") != "" {
		return Runtime{}
	}
	term := strings.ToLower(os.Getenv("TERM"))
	program := strings.ToLower(os.Getenv("TERM_PROGRAM"))
	if !strings.Contains(term, "xterm-kitty") && program != "kitty" && os.Getenv("KITTY_WINDOW_ID") == "" {
		return Runtime{}
	}
	output, err := exec.Command("kitty", "--version").CombinedOutput()
	if err != nil || !versionAtLeast(string(output), 0, 47, 0) {
		return Runtime{}
	}
	return Runtime{Enabled: true}
}

func versionAtLeast(output string, major, minor, patch int) bool {
	for _, field := range strings.Fields(output) {
		parts := strings.Split(field, ".")
		if len(parts) < 2 {
			continue
		}
		values := [3]int{}
		valid := true
		for index := range values {
			if index >= len(parts) {
				break
			}
			digits := strings.TrimRightFunc(parts[index], func(r rune) bool { return r < '0' || r > '9' })
			value, err := strconv.Atoi(digits)
			if err != nil {
				valid = false
				break
			}
			values[index] = value
		}
		if valid {
			return values[0] > major || values[0] == major && (values[1] > minor || values[1] == minor && values[2] >= patch)
		}
	}
	return false
}
