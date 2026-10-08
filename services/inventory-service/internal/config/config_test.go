package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/clevertechware/observabilite-agents-demo/services/inventory-service/internal/config"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		want    config.Config
		wantErr bool
	}{
		{
			name: "applies defaults for port and fault",
			env:  map[string]string{"PRICING_URL": "http://pricing"},
			want: config.Config{PricingURL: "http://pricing", Port: "8080"},
		},
		{
			name: "reads every variable",
			env: map[string]string{
				"PRICING_URL": "http://pricing", "DATABASE_URL": "postgres://db", "PORT": "9090",
				"FAULT_SLOW_QUERY_SECONDS": "2.5",
			},
			want: config.Config{
				PricingURL: "http://pricing", DatabaseURL: "postgres://db", Port: "9090", FaultSlowQuerySeconds: 2.5,
			},
		},
		{
			name:    "rejects missing pricing url",
			env:     map[string]string{},
			wantErr: true,
		},
		{
			name:    "rejects non numeric fault",
			env:     map[string]string{"PRICING_URL": "http://pricing", "FAULT_SLOW_QUERY_SECONDS": "slow"},
			wantErr: true,
		},
		{
			name:    "rejects negative fault",
			env:     map[string]string{"PRICING_URL": "http://pricing", "FAULT_SLOW_QUERY_SECONDS": "-1"},
			wantErr: true,
		},
		{
			name:    "rejects NaN fault",
			env:     map[string]string{"PRICING_URL": "http://pricing", "FAULT_SLOW_QUERY_SECONDS": "NaN"},
			wantErr: true,
		},
		{
			name:    "rejects positive infinite fault",
			env:     map[string]string{"PRICING_URL": "http://pricing", "FAULT_SLOW_QUERY_SECONDS": "+Inf"},
			wantErr: true,
		},
		{
			name:    "rejects negative infinite fault",
			env:     map[string]string{"PRICING_URL": "http://pricing", "FAULT_SLOW_QUERY_SECONDS": "-Inf"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := config.Load(func(key string) string { return tt.env[key] })
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
