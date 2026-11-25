// Package orders provides the order-service commands for ctwctl.
package orders

import (
	"context"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/config"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/order"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/shop"
	"github.com/urfave/cli/v3"
)

// Dependencies contains the services used by the order commands.
type Dependencies struct {
	// NewOrderClient creates an order-service client from the resolved configuration.
	NewOrderClient func(config.Settings) order.Client
}

func (d Dependencies) withDefaults() Dependencies {
	if d.NewOrderClient == nil {
		d.NewOrderClient = func(s config.Settings) order.Client { return shop.NewOrderClient(s.Shop.URL) }
	}

	return d
}

// NewCommand creates the order command tree.
func NewCommand(deps Dependencies) *cli.Command {
	deps = deps.withDefaults()

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
