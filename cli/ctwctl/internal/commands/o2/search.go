package o2

import (
	"fmt"
	"strings"
	"time"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/config"
	"github.com/urfave/cli/v3"
)

func searcherFor(s config.Settings, deps Dependencies) (LogSearcher, error) {
	var missing []string
	if s.O2.User == "" {
		missing = append(missing, "o2.user (CTWCTL_O2_USER)")
	}
	if s.O2.Token == "" {
		missing = append(missing, "o2.token (CTWCTL_O2_TOKEN)")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing OpenObserve credentials: %s", strings.Join(missing, ", "))
	}

	return deps.NewLogSearcher(s), nil
}

func parseSearchOptions(cmd *cli.Command) (time.Duration, int, error) {
	sinceValue := cmd.String("since")
	since, err := time.ParseDuration(sinceValue)
	if err != nil || since <= 0 {
		return 0, 0, fmt.Errorf("invalid --since %q: a positive duration is expected, for example 15m or 1h", sinceValue)
	}

	limit := cmd.Int("limit")
	if limit <= 0 {
		return 0, 0, fmt.Errorf("--limit must be strictly positive, got %d", limit)
	}

	return since, limit, nil
}
