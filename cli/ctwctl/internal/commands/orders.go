package commands

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/order"
	"github.com/urfave/cli/v3"
)

func ordersCommand(deps dependencies) *cli.Command {
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

func createOrder(ctx context.Context, cmd *cli.Command, deps dependencies) error {
	quantity := cmd.Int("quantity")
	s := settingsFrom(ctx)
	created, err := order.NewOrders(deps.newOrderClient(s)).Create(ctx, cmd.String("sku"), quantity)
	if errors.Is(err, order.ErrInvalidQuantity) {
		return fmt.Errorf("--quantity must be strictly positive, got %d", quantity)
	}
	if err != nil {
		return err
	}

	return printOrder(cmd, s, created, "created")
}

func getOrder(ctx context.Context, cmd *cli.Command, deps dependencies) error {
	if cmd.Args().Len() != 1 {
		return errors.New("an order id is expected: ctwctl orders get <id>")
	}

	id, err := strconv.ParseInt(cmd.Args().First(), 10, 64)
	if err != nil {
		return fmt.Errorf("invalid order id %q", cmd.Args().First())
	}

	s := settingsFrom(ctx)
	got, err := order.NewOrders(deps.newOrderClient(s)).Get(ctx, id)
	if err != nil {
		return err
	}

	return printOrder(cmd, s, got, "read")
}

func printOrder(cmd *cli.Command, s settings, item order.Order, verb string) error {
	if s.Output == outputJSON {
		return writeJSON(writer(cmd), item)
	}

	_, err := fmt.Fprintf(writer(cmd), "order %d %s: %s x%d, unit price %s, total %s\n",
		item.ID, verb, item.SKU, item.Quantity, item.UnitPrice, item.Total)
	if err == nil && item.Location != "" {
		_, err = fmt.Fprintf(writer(cmd), "Location: %s\n", item.Location)
	}

	return err
}
