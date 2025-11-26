package cmd

import (
	"context"
	"testing"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	dbplugin "github.com/hashicorp/vault/sdk/database/dbplugin/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitialize_Timeout(t *testing.T) {
	tests := []struct {
		name            string
		config          map[string]interface{}
		expectedTimeout time.Duration
		expectError     bool
	}{
		{
			name:            "default timeout",
			config:          map[string]interface{}{},
			expectedTimeout: 20 * time.Second,
		},
		{
			name: "custom timeout string",
			config: map[string]interface{}{
				"timeout": "5s",
			},
			expectedTimeout: 5 * time.Second,
		},
		{
			name: "invalid timeout too small",
			config: map[string]interface{}{
				"timeout": "500ms",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCmd()
			req := dbplugin.InitializeRequest{
				Config: tt.config,
			}

			_, err := c.Initialize(context.Background(), req)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedTimeout, c.Timeout)
			}
		})
	}
}

func TestExecuteScript_Timeout(t *testing.T) {
	// Skip on windows as sleep command might differ or shell might differ
	// But getShell handles it. "timeout" command is not standard on windows.
	// We'll use "sleep" which exists on unix.

	c := newCmd()
	c.Logger = hclog.NewNullLogger()
	c.Timeout = 100 * time.Millisecond

	// Case 1: Command takes longer than timeout
	// sleep 1 should take 1s, which is > 100ms
	ctx := context.Background()
	script := "sleep 1"
	params := map[string]string{}

	err := c.executeScript(ctx, script, params)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "script execution timed out")

	// Case 2: Command finishes within timeout
	c.Timeout = 2 * time.Second
	script = "echo hello"
	err = c.executeScript(ctx, script, params)
	require.NoError(t, err)
}
