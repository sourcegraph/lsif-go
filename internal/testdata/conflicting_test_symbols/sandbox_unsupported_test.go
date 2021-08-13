// From https://github.com/moby/moby/blob/master/libnetwork/osl/sandbox_unsupported_test.go

// +build !linux

package osl

import (
	"errors"
	"testing"
)

var ErrNotImplemented = errors.New("not implemented")

func newKey(t *testing.T) (string, error) {
	return "", ErrNotImplemented
}

func verifySandbox(t *testing.T, s Sandbox) {
	return
}
