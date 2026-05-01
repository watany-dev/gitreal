package main

import (
	"io"
	"os"

	"github.com/watany-dev/gitreal/internal/cli"
)

func Main(args []string, stdout, stderr io.Writer) int {
	return cli.Run(args, stdout, stderr)
}

func main() {
	os.Exit(Main(os.Args[1:], os.Stdout, os.Stderr))
}
