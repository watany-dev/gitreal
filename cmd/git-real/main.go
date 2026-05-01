package main

import (
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/watany-dev/gitreal/internal/cli"
)

// Build-time variables populated via -ldflags by the Makefile and the release
// workflow. They are surfaced via `git real --version` so users can verify that
// a binary corresponds to a specific source revision and SHA256SUMS entry.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func Main(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	for _, arg := range args {
		if arg == "--version" || arg == "-V" {
			printVersion(stdout)
			return 0
		}
	}
	return cli.Run(ctx, args, stdout, stderr)
}

func printVersion(w io.Writer) {
	_, _ = io.WriteString(w, "git-real "+version+" ("+commit+", built "+date+")\n")
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(Main(ctx, os.Args[1:], os.Stdout, os.Stderr))
}
