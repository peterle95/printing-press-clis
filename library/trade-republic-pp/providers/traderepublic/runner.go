package traderepublic

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
)

// RunRequest describes one direct process invocation. Argv is passed to
// exec.CommandContext without a shell, so values are never re-parsed as shell
// syntax.
type RunRequest struct {
	Argv   []string
	Dir    string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// Runner isolates process execution so the private-protocol compatibility
// adapter can be tested without pytr or a Trade Republic account.
type Runner interface {
	Run(context.Context, RunRequest) error
}

// ExecRunner executes an argv vector directly with context cancellation.
type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, request RunRequest) error {
	if len(request.Argv) == 0 || request.Argv[0] == "" {
		return fmt.Errorf("empty command")
	}
	cmd := exec.CommandContext(ctx, request.Argv[0], request.Argv[1:]...)
	cmd.Dir = request.Dir
	cmd.Stdin = request.Stdin
	cmd.Stdout = request.Stdout
	cmd.Stderr = request.Stderr
	return cmd.Run()
}

// boundedBuffer accepts the complete write while retaining only max bytes.
// This prevents a noisy or compromised helper process from exhausting memory.
type boundedBuffer struct {
	buf       bytes.Buffer
	max       int
	truncated bool
}

func newBoundedBuffer(max int) *boundedBuffer {
	return &boundedBuffer{max: max}
}

func (b *boundedBuffer) Write(value []byte) (int, error) {
	original := len(value)
	remaining := b.max - b.buf.Len()
	if remaining > 0 {
		if remaining < len(value) {
			_, _ = b.buf.Write(value[:remaining])
			b.truncated = true
		} else {
			_, _ = b.buf.Write(value)
		}
	} else if len(value) > 0 {
		b.truncated = true
	}
	return original, nil
}

func (b *boundedBuffer) String() string { return b.buf.String() }
