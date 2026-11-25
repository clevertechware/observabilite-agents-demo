package orders

import (
	"fmt"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/commands/cmdutil"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/config"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/order"
	"github.com/urfave/cli/v3"
)

func printOrder(cmd *cli.Command, s config.Settings, item order.Order, verb string) error {
	if s.Output == config.OutputJSON {
		return cmdutil.WriteJSON(cmdutil.Writer(cmd), item)
	}

	_, err := fmt.Fprintf(cmdutil.Writer(cmd), "order %d %s: %s x%d, unit price %s, total %s\n",
		item.ID, verb, item.SKU, item.Quantity, item.UnitPrice, item.Total)
	if err == nil && item.Location != "" {
		_, err = fmt.Fprintf(cmdutil.Writer(cmd), "Location: %s\n", item.Location)
	}

	return err
}
