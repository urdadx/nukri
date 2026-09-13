//go:build !unix

package kittydnd

import (
	"errors"
	"io"
)

type Input struct{}

func OpenInput() (*Input, error) { return nil, errors.New("Kitty drag and drop requires Unix") }

func (*Input) Read([]byte) (int, error) { return 0, io.EOF }
func (*Input) Close() error             { return nil }
func (*Input) Events() <-chan Event     { return nil }
