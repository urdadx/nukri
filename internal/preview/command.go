package preview

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
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

func runCommand(ctx context.Context, outputLimit int64, executable string, arguments ...string) ([]byte, error) {
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
		if contextError := ctx.Err(); contextError != nil {
			return nil, contextError
		}
		message := bytes.TrimSpace(stderr.buffer.Bytes())
		if len(message) != 0 {
			return nil, fmt.Errorf("%s failed: %w: %s", executable, err, message)
		}
		return nil, fmt.Errorf("%s failed: %w", executable, err)
	}
	return stdout.buffer.Bytes(), nil
}
