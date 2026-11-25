package orders

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/order"
	"github.com/urfave/cli/v3"
)

func getOrder(ctx context.Context, cmd *cli.Command, deps Dependencies) error {
	if cmd.Args().Len() != 1 {
		return errors.New("an order id is expected: ctwctl orders get <id>")
	}

	id, err := strconv.ParseInt(cmd.Args().First(), 10, 64)
	if err != nil {
		return fmt.Errorf("invalid order id %q", cmd.Args().First())
	}

	s, err := deps.LoadSettings(cmd)
	if err != nil {
		return err
	}

	got, err := order.NewOrders(deps.NewOrderClient(s)).Get(ctx, id)
	if err != nil {
		return err
	}

	return printOrder(cmd, s, got, "read")
}
