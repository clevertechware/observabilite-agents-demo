package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/commands"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := commands.NewRootCommand().Run(ctx, os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "ctwctl : %v\n", err)
		stop()
		os.Exit(1)
	}
}
