// Package adb runs the adb binary without a shell.
package adb

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

// Runner executes adb with the given arguments and returns stdout.
type Runner interface {
	Run(ctx context.Context, args ...string) ([]byte, error)
}

// Exec runs the real adb binary. Arguments are passed directly (no shell).
type Exec struct {
	Bin     string
	Timeout time.Duration
}

func (e Exec) Run(ctx context.Context, args ...string) ([]byte, error) {
	bin := e.Bin
	if bin == "" {
		bin = "adb"
	}
	t := e.Timeout
	if t == 0 {
		t = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, t)
	defer cancel()
	var out, errb bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("adb %v: %w: %s", args, err, bytes.TrimSpace(errb.Bytes()))
	}
	return out.Bytes(), nil
}
