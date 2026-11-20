package commands

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/shop"
	"github.com/urfave/cli/v3"
)

type orderClient interface {
	Create(ctx context.Context, sku string, quantity int) (shop.Order, error)
	Get(ctx context.Context, id int64) (shop.Order, error)
}

func ordersCommand(deps dependencies) *cli.Command {
	return &cli.Command{
		Name:  "orders",
		Usage: "commandes d'order-service, par l'hôte public",
		Commands: []*cli.Command{
			{
				Name:  "create",
				Usage: "crée une commande (POST /orders)",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "sku", Usage: "référence de l'article, par exemple SKU-001", Required: true},
					&cli.IntFlag{Name: "quantity", Usage: "quantité commandée", Value: 1},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return createOrder(ctx, cmd, deps)
				},
			},
			{
				Name:      "get",
				Usage:     "lit une commande (GET /orders/{id})",
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
	if quantity <= 0 {
		return fmt.Errorf("--quantity doit être strictement positif, reçu %d", quantity)
	}
	s := settingsFrom(ctx)
	order, err := deps.newOrderClient(s).Create(ctx, cmd.String("sku"), quantity)
	if err != nil {
		return err
	}
	return printOrder(cmd, s, order, "créée")
}

func getOrder(ctx context.Context, cmd *cli.Command, deps dependencies) error {
	if cmd.Args().Len() != 1 {
		return errors.New("un identifiant est attendu : ctwctl orders get <id>")
	}
	id, err := strconv.ParseInt(cmd.Args().First(), 10, 64)
	if err != nil {
		return fmt.Errorf("identifiant de commande invalide %q", cmd.Args().First())
	}
	s := settingsFrom(ctx)
	order, err := deps.newOrderClient(s).Get(ctx, id)
	if err != nil {
		return err
	}
	return printOrder(cmd, s, order, "lue")
}

func printOrder(cmd *cli.Command, s settings, order shop.Order, verb string) error {
	if s.Output == outputJSON {
		return writeJSON(writer(cmd), order)
	}
	_, err := fmt.Fprintf(writer(cmd), "commande %d %s : %s x%d, prix unitaire %s, total %s\n",
		order.ID, verb, order.SKU, order.Quantity, order.UnitPrice, order.Total)
	if err == nil && order.Location != "" {
		_, err = fmt.Fprintf(writer(cmd), "Location: %s\n", order.Location)
	}
	return err
}
