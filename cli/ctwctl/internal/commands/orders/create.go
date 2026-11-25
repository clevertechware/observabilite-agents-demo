package orders

import (
	"context"
	"errors"
	"fmt"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/order"
	"github.com/urfave/cli/v3"
)

func createOrder(ctx context.Context, cmd *cli.Command, deps Dependencies) error {
	s, err := deps.LoadSettings(cmd)
	if err != nil {
		return err
	}

	quantity := cmd.Int("quantity")
	created, err := order.NewOrders(deps.NewOrderClient(s)).Create(ctx, cmd.String("sku"), quantity)
	if errors.Is(err, order.ErrInvalidQuantity) {
		return fmt.Errorf("--quantity must be strictly positive, got %d", quantity)
	}
	if err != nil {
		return err
	}

	return printOrder(cmd, s, created, "created")
}
