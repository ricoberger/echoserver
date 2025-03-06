package tracer

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestShutdown(t *testing.T) {
	client, err := New(Config{Enabled: true, Service: "test", Address: "localhost:4317"})
	require.NotNil(t, client)
	require.NoError(t, err)
	require.NotPanics(t, client.Shutdown)
}

func TestNew(t *testing.T) {
	t.Run("should return tracer", func(t *testing.T) {
		client, err := New(Config{Enabled: true, Service: "test", Address: "localhost:4317"})
		require.NotNil(t, client)
		require.NoError(t, err)
	})

	t.Run("should return tracer if disabled", func(t *testing.T) {
		client, err := New(Config{Enabled: false, Service: "test", Address: "localhost:4317"})
		require.NotNil(t, client)
		require.NoError(t, err)
	})
}
