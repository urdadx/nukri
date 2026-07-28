package main

import (
	"testing"
	"time"
)

func TestFormatSize(t *testing.T) {
	tests := []struct {
		size int64
		want string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1024 * 1024, "1.0 MiB"},
	}
	for _, test := range tests {
		if got := formatSize(test.size); got != test.want {
			t.Errorf("formatSize(%d) = %q, want %q", test.size, got, test.want)
		}
	}
}

func TestFormatModified(t *testing.T) {
	if got := formatModified(time.Time{}); got != "-" {
		t.Fatalf("zero time = %q, want -", got)
	}
	modified := time.Date(2026, time.July, 28, 14, 5, 0, 0, time.UTC)
	if got := formatModified(modified); got != "2026-07-28 14:05" {
		t.Fatalf("modified = %q", got)
	}
}
