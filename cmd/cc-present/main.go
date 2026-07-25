// Command cc-present: Ad-hoc live web artifacts for Claude sessions — approval boards, choices, and rich content whose every click streams back to the agent.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/yasyf/cc-present/internal/cli"
	applog "github.com/yasyf/cc-present/internal/log"
	"github.com/yasyf/daemonkit/trust"
)

func main() {
	if handled, err := trust.RunVerifierChild(os.Args[1:], os.Stdout); handled {
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	applog.Setup()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := cli.NewRootCmd().ExecuteContext(ctx); err != nil {
		// Minimal error handling: report on stderr and exit non-zero. As the CLI
		// grows, map typed errors to exit codes here (see STYLEGUIDE.md § Error Handling).
		fmt.Fprintln(os.Stderr, "cc-present:", err)
		os.Exit(1)
	}
}
