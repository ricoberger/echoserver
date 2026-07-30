package grpcserver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestServer(t *testing.T) {
	t.Run("should start and stop the server", func(t *testing.T) {
		server := New(Config{Address: "127.0.0.1:0"})
		require.NotNil(t, server)

		go server.Start()
		time.Sleep(100 * time.Millisecond)
		server.Stop()
	})

	t.Run("should return without panic when the listener can not be created", func(t *testing.T) {
		server := New(Config{Address: "invalid-address"})
		require.NotNil(t, server)

		require.NotPanics(t, func() {
			server.Start()
		})
	})
}
