package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoot_WhenOutputFormatIsUnknown_ReturnsError(t *testing.T) {
	t.Parallel()

	_, err := run(t, dependencies{}, "-o", "yaml", "config", "view")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "yaml")
}
