package apps

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// ErrMultipleLaunches is returned by Command when the Exec line accepts a
// single file but several were given; use Commands instead.
var ErrMultipleLaunches = errors.New("exec line accepts one file per launch")

// Command expands the Exec line for paths into a single invocation.
func (a App) Command(paths []string) (string, []string, error) {
	argvs, err := a.commands(paths)
	if err != nil {
		return "", nil, err
	}
	if len(argvs) > 1 {
		return "", nil, fmt.Errorf("%s: %w", a.ID, ErrMultipleLaunches)
	}
	return argvs[0][0], argvs[0][1:], nil
}

// Commands expands the Exec line into one argv per launch. Apps that only
// accept %f or %u get one launch per path. It returns nil if Exec is invalid.
func (a App) Commands(paths []string) [][]string {
	argvs, err := a.commands(paths)
	if err != nil {
		return nil
	}
	return argvs
}

func (a App) commands(paths []string) ([][]string, error) {
	tokens, err := splitExec(a.Exec)
	if err != nil {
		return nil, fmt.Errorf("parse exec for %s: %w", a.ID, err)
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("empty exec for %s", a.ID)
	}
	multi, single := false, false
	for _, t := range tokens {
		for _, c := range fieldCodes(t) {
			switch c {
			case 'F', 'U':
				multi = true
			case 'f', 'u':
				single = true
			}
		}
	}

	var argvs [][]string
	switch {
	case multi || len(paths) == 0:
		argvs = append(argvs, a.expand(tokens, paths))
	case single:
		for _, p := range paths {
			argvs = append(argvs, a.expand(tokens, []string{p}))
		}
	default:
		argvs = append(argvs, append(a.expand(tokens, nil), paths...))
	}
	for _, argv := range argvs {
		if len(argv) == 0 || argv[0] == "" {
			return nil, fmt.Errorf("exec for %s expands to no program", a.ID)
		}
	}
	return argvs, nil
}

// fieldCodes returns the field code letters in an argument (%% excluded).
func fieldCodes(arg string) []byte {
	var codes []byte
	for i := 0; i+1 < len(arg); i++ {
		if arg[i] == '%' {
			if arg[i+1] != '%' {
				codes = append(codes, arg[i+1])
			}
			i++
		}
	}
	return codes
}

// expand substitutes field codes in tokens for the given files.
func (a App) expand(tokens []string, files []string) []string {
	var argv []string
	for _, t := range tokens {
		switch t {
		case "%F":
			argv = append(argv, files...)
			continue
		case "%U":
			for _, f := range files {
				argv = append(argv, fileURI(f))
			}
			continue
		case "%i":
			if a.Icon != "" {
				argv = append(argv, "--icon", a.Icon)
			}
			continue
		}
		var b strings.Builder
		hadCode := false
		for i := 0; i < len(t); i++ {
			if t[i] != '%' || i+1 == len(t) {
				b.WriteByte(t[i])
				continue
			}
			i++
			hadCode = true
			switch t[i] {
			case '%':
				b.WriteByte('%')
			case 'f', 'F':
				if len(files) > 0 {
					b.WriteString(files[0])
				}
			case 'u', 'U':
				if len(files) > 0 {
					b.WriteString(fileURI(files[0]))
				}
			case 'i':
				b.WriteString(a.Icon)
			case 'c':
				b.WriteString(a.Name)
			case 'k':
				b.WriteString(a.File)
			default:
				// deprecated (%d %D %n %N %v %m) or unknown: dropped
			}
		}
		// Drop arguments that consisted only of codes expanding to nothing.
		if s := b.String(); s != "" || !hadCode {
			argv = append(argv, s)
		}
	}
	return argv
}

func fileURI(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	return (&url.URL{Scheme: "file", Path: path}).String()
}

// splitExec tokenizes an Exec value per the Desktop Entry spec: arguments are
// separated by spaces and may be double-quoted, with \" \` \$ \\ escapes
// inside quotes.
func splitExec(s string) ([]string, error) {
	var args []string
	var cur strings.Builder
	inArg, inQuote := false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inQuote && c == '\\' && i+1 < len(s):
			i++
			cur.WriteByte(s[i])
		case inQuote && c == '"':
			inQuote = false
		case inQuote:
			cur.WriteByte(c)
		case c == '"':
			inQuote, inArg = true, true
		case c == ' ' || c == '\t' || c == '\n':
			if inArg {
				args = append(args, cur.String())
				cur.Reset()
				inArg = false
			}
		case c == '\\' && i+1 < len(s):
			i++
			cur.WriteByte(s[i])
			inArg = true
		default:
			cur.WriteByte(c)
			inArg = true
		}
	}
	if inQuote {
		return nil, errors.New("unterminated quote")
	}
	if inArg {
		args = append(args, cur.String())
	}
	return args, nil
}

// Start launches a non-terminal app detached from nukri, one process per
// launch returned by Commands. It does not wait for the processes.
func (a App) Start(paths []string) error {
	if a.Terminal {
		return fmt.Errorf("%s runs in a terminal; launch it via Commands", a.ID)
	}
	argvs, err := a.commands(paths)
	if err != nil {
		return err
	}
	for _, argv := range argvs {
		if err := startDetached(argv, a.Dir); err != nil {
			return fmt.Errorf("start %s: %w", a.ID, err)
		}
	}
	return nil
}

func startDetached(argv []string, dir string) error {
	devnull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open %s: %w", os.DevNull, err)
	}
	defer devnull.Close()
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = devnull, devnull, devnull
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
