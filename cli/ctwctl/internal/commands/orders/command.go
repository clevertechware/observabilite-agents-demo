// Package orders provides the order-service commands for ctwctl.
package orders

import (
	"context"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/config"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/order"
	"github.com/urfave/cli/v3"
)

// Dependencies contains the configuration and the services used by the order commands.
type Dependencies struct {
	// LoadSettings resolves the settings of the invoked command.
	LoadSettings func(*cli.Command) (config.Settings, error)
	// NewOrderClient creates an order-service client from the resolved configuration.
	NewOrderClient func(config.Settings) order.Client
}

// NewCommand creates the order command tree. Every field of deps is required.
func NewCommand(deps Dependencies) *cli.Command {
	return &cli.Command{
		Name:  "orders",
		Usage: "order-service orders, through the public host",
		Commands: []*cli.Command{
			{
				Name:  "create",
				Usage: "create an order (POST /orders)",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "sku", Usage: "item reference, for example SKU-001", Required: true},
					&cli.IntFlag{Name: "quantity", Usage: "ordered quantity", Value: 1},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return createOrder(ctx, cmd, deps)
				},
			},
			{
				Name:      "get",
				Usage:     "read an order (GET /orders/{id})",
				ArgsUsage: "<id>",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return getOrder(ctx, cmd, deps)
				},
			},
		},
	}
}
