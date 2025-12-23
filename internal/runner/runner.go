package runner

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Runner struct {
	DryRun bool
	Stdout *os.File
	Stderr *os.File
}

func (r Runner) Run(ctx context.Context, name string, args ...string) error {
	if r.Stdout == nil {
		r.Stdout = os.Stdout
	}
	if r.Stderr == nil {
		r.Stderr = os.Stderr
	}
	fmt.Fprintf(r.Stdout, "+ %s %s\n", name, strings.Join(args, " "))
	if r.DryRun {
		return nil
	}
	cctx, cancel := context.WithTimeout(ctx, 60*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(cctx, name, args...)
	cmd.Stdout = r.Stdout
	cmd.Stderr = r.Stderr
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	return cmd.Run()
}

func (r Runner) Output(ctx context.Context, name string, args ...string) (string, error) {
	if r.Stdout == nil {
		r.Stdout = os.Stdout
	}
	if r.Stderr == nil {
		r.Stderr = os.Stderr
	}
	fmt.Fprintf(r.Stdout, "+ %s %s\n", name, strings.Join(args, " "))
	if r.DryRun {
		return "", nil
	}
	cctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(cctx, name, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = r.Stderr
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	err := cmd.Run()
	return strings.TrimSpace(buf.String()), err
}
