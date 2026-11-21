package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/urfave/cli/v3"
)

func searcherFor(s settings, deps dependencies) (logSearcher, error) {
	var missing []string
	if s.O2.User == "" {
		missing = append(missing, "o2.user (CTWCTL_O2_USER)")
	}
	if s.O2.Token == "" {
		missing = append(missing, "o2.token (CTWCTL_O2_TOKEN)")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("identifiants OpenObserve absents : %s", strings.Join(missing, ", "))
	}
	return deps.newLogSearcher(s), nil
}

func parseSearchOptions(cmd *cli.Command) (time.Duration, int, error) {
	sinceValue := cmd.String("since")
	since, err := time.ParseDuration(sinceValue)
	if err != nil || since <= 0 {
		return 0, 0, fmt.Errorf("--since %q invalide : une durée positive est attendue, par exemple 15m ou 1h", sinceValue)
	}
	limit := cmd.Int("limit")
	if limit <= 0 {
		return 0, 0, fmt.Errorf("--limit doit être strictement positif, reçu %d", limit)
	}
	return since, limit, nil
}
