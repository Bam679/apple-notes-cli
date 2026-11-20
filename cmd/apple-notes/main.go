// Command apple-notes is an agent-friendly CLI for Apple Notes.
package main

import (
	"context"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/Bam679/apple-notes-cli/internal/notes"
)

// version is set by the release build (-ldflags "-X main.version=...").
var version = "dev"

func main() {
	if version == "dev" {
		if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
			version = bi.Main.Version
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := notes.NewApp(notes.NewJXA(), version).Main(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}
