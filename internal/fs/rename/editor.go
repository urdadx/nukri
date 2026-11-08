package rename

import (
	"fmt"
	"path/filepath"
	"strings"
)

// EditorText returns the text to hand to $EDITOR: one line per source with
// its path relative to dir, in order, with a trailing newline. Sources outside
// dir are written as absolute paths. A relative name starting with '#' is
// prefixed with "./" so it is not mistaken for a comment.
func EditorText(dir string, sources []string) string {
	var b strings.Builder
	for _, source := range sources {
		b.WriteString(editorLine(dir, source))
		b.WriteByte('\n')
	}
	return b.String()
}

func editorLine(dir, source string) string {
	source = filepath.Clean(source)
	rel, err := filepath.Rel(filepath.Clean(dir), source)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
		return source
	}
	if strings.HasPrefix(rel, "#") {
		return "./" + rel
	}
	return rel
}

// PlanEditor parses the text returned by the editor into a rename plan.
//
// Lines starting with '#' are comments and a final empty line is ignored; the
// remaining lines must match sources one to one (otherwise a single LineError
// with Line -1 is returned). Line numbers in errors are 0-based indexes into
// edited, comments included. A trailing '\r' is stripped; no other whitespace
// is trimmed. Each line is either an absolute path or a path relative to dir,
// and may move the item into another directory. The result is cleaned, so
// "." and ".." segments are allowed as long as the cleaned path is not the
// filesystem root. Missing parent directories are created by Apply.
// Unchanged lines are dropped.
func PlanEditor(dir string, sources []string, edited string) ([]Rename, []LineError) {
	dir = filepath.Clean(dir)
	if !filepath.IsAbs(dir) {
		return nil, []LineError{{Line: -1, Message: "folder must be an absolute path"}}
	}
	lines := strings.Split(edited, "\n")
	if strings.HasSuffix(edited, "\n") {
		lines = lines[:len(lines)-1]
	}
	type entry struct {
		line int
		text string
	}
	var entries []entry
	for i, line := range lines {
		line = strings.TrimSuffix(line, "\r")
		if strings.HasPrefix(line, "#") {
			continue
		}
		entries = append(entries, entry{line: i, text: line})
	}
	if len(entries) != len(sources) {
		return nil, []LineError{{Line: -1, Message: countMessage(len(sources), len(entries))}}
	}

	var errs []LineError
	cands := make([]candidate, 0, len(sources))
	for i, e := range entries {
		to, msg := editorTarget(dir, e.text)
		if msg != "" {
			errs = append(errs, LineError{Line: e.line, Message: msg})
			continue
		}
		cands = append(cands, candidate{line: e.line, label: e.text, from: filepath.Clean(sources[i]), to: to})
	}
	return finishPlan(cands, errs)
}

// editorTarget resolves one edited line to an absolute, clean path.
func editorTarget(dir, line string) (string, string) {
	switch {
	case line == "":
		return "", "name cannot be empty"
	case strings.ContainsRune(line, 0):
		return "", "name cannot contain NUL"
	}
	to := line
	if !filepath.IsAbs(to) {
		to = filepath.Join(dir, to)
	}
	to = filepath.Clean(to)
	if to == "/" {
		return "", fmt.Sprintf("%q resolves to the filesystem root", line)
	}
	return to, ""
}
