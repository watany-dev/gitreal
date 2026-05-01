package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/watany-dev/gitreal/internal/challenge"
)

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printHelp(stdout)
		return 0
	}

	switch args[0] {
	case "help", "-h", "--help":
		printHelp(stdout)
		return 0
	case "init":
		return writeLine(stdout, "git-real init: repository bootstrap complete")
	case "status":
		return writeLine(stdout, "git-real status: enabled=false armed=false")
	case "once":
		return runTimedCommand("once", args[1:], stdout, stderr)
	case "start":
		return runTimedCommand("start", args[1:], stdout, stderr)
	case "arm":
		return writeLine(stdout, "git-real arm: destructive mode enabled")
	case "disarm":
		return writeLine(stdout, "git-real disarm: dry-run mode enabled")
	case "rescue":
		return runRescue(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "git-real: unknown command: %s\n", args[0])
		printHelp(stderr)
		return 2
	}
}

func runTimedCommand(name string, args []string, stdout, stderr io.Writer) int {
	graceSeconds, err := parseGraceSeconds(args, stderr)
	if err != nil {
		return 2
	}

	return writeLine(stdout, fmt.Sprintf("git-real %s: challenge window set to %d seconds", name, graceSeconds))
}

func runRescue(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "git-real rescue: expected subcommand list or restore <ref>")
		return 2
	}

	switch args[0] {
	case "list":
		if len(args) != 1 {
			fmt.Fprintln(stderr, "git-real rescue list: unexpected arguments")
			return 2
		}

		return writeLine(stdout, "git-real rescue list: no backups yet")
	case "restore":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "git-real rescue restore: expected exactly one backup ref")
			return 2
		}

		return writeLine(stdout, fmt.Sprintf("git-real rescue restore: would restore %s", args[1]))
	default:
		fmt.Fprintf(stderr, "git-real rescue: unknown subcommand: %s\n", args[0])
		return 2
	}
}

func parseGraceSeconds(args []string, stderr io.Writer) (int, error) {
	fs := flag.NewFlagSet("git-real", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	graceSeconds := fs.Int("grace-seconds", challenge.DefaultGraceSeconds, "challenge window in seconds")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "git-real: %v\n", err)
		return 0, err
	}

	if fs.NArg() != 0 {
		err := fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
		fmt.Fprintf(stderr, "git-real: %v\n", err)
		return 0, err
	}

	return challenge.NormalizeGraceSeconds(*graceSeconds), nil
}

func printHelp(w io.Writer) {
	fmt.Fprintln(w, `git-real - BeReal-inspired punishment CLI for Git

Usage:
  git real init
  git real status
  git real once [--grace-seconds=120]
  git real start [--grace-seconds=120]
  git real arm
  git real disarm
  git real rescue list
  git real rescue restore <backup-ref>`)
}

func writeLine(w io.Writer, line string) int {
	fmt.Fprintln(w, line)
	return 0
}
