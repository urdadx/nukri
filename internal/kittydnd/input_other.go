//go:build !unix

package kittydnd

import "errors"

type Input struct{}

func OpenInput() (*Input, error) { return nil, errors.New("Kitty drag and drop requires Unix") }
