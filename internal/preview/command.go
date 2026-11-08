package preview

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

type limitedBuffer struct {
	buffer bytes.Buffer
	limit  int64
}

func (w *limitedBuffer) Write(value []byte) (int, error) {
	remaining := w.limit - int64(w.buffer.Len())
	if remaining <= 0 {
		return 0, ErrOutputTooLarge
	}
	if int64(len(value)) > remaining {
		_, _ = w.buffer.Write(value[:remaining])
		return int(remaining), ErrOutputTooLarge
	}
	return w.buffer.Write(value)
}

// Preview tools are killed after these limits, so a hung tool can't hold a
// preview worker. Document conversions start a whole office suite and get longer.
const (
	toolTimeout       = 15 * time.Second
	conversionTimeout = 60 * time.Second
)

func runCommand(ctx context.Context, outputLimit int64, executable string, arguments ...string) ([]byte, error) {
	return runCommandWithTimeout(ctx, toolTimeout, outputLimit, executable, arguments...)
}

func runCommandWithTimeout(parent context.Context, timeout time.Duration, outputLimit int64, executable string, arguments ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	stdout := &limitedBuffer{limit: outputLimit}
	stderr := &limitedBuffer{limit: 64 << 10}
	command := exec.CommandContext(ctx, executable, arguments...)
	command.Stdout = stdout
	command.Stderr = stderr
	err := command.Run()
	if errors.Is(err, ErrOutputTooLarge) {
		return nil, ErrOutputTooLarge
	}
	if err != nil {
		if parentError := parent.Err(); parentError != nil {
			return nil, parentError
		}
		if ctx.Err() != nil {
			return nil, fmt.Errorf("%s timed out after %v", executable, timeout)
		}
		message := bytes.TrimSpace(stderr.buffer.Bytes())
		if len(message) != 0 {
			return nil, fmt.Errorf("%s failed: %w: %s", executable, err, message)
		}
		return nil, fmt.Errorf("%s failed: %w", executable, err)
	}
	return stdout.buffer.Bytes(), nil
}
