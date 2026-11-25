package orders

import (
	"encoding/json"
	"fmt"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/config"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/order"
	"github.com/urfave/cli/v3"
)

func printOrder(cmd *cli.Command, s config.Settings, item order.Order, verb string) error {
	if s.Output == config.OutputJSON {
		return json.NewEncoder(cmd.Root().Writer).Encode(item)
	}

	_, err := fmt.Fprintf(cmd.Root().Writer, "order %d %s: %s x%d, unit price %s, total %s\n",
		item.ID, verb, item.SKU, item.Quantity, item.UnitPrice, item.Total)
	if err == nil && item.Location != "" {
		_, err = fmt.Fprintf(cmd.Root().Writer, "Location: %s\n", item.Location)
	}

	return err
}
