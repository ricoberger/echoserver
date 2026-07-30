package httpserver

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDuration(t *testing.T) {
	t.Run("should marshal to a duration string", func(t *testing.T) {
		data, err := json.Marshal(Duration{5 * time.Second})
		require.NoError(t, err)
		require.Equal(t, `"5s"`, string(data))
	})

	t.Run("should unmarshal from a duration string", func(t *testing.T) {
		var d Duration
		require.NoError(t, json.Unmarshal([]byte(`"5s"`), &d))
		require.Equal(t, 5*time.Second, d.Duration)
	})

	t.Run("should unmarshal from a number of nanoseconds", func(t *testing.T) {
		var d Duration
		require.NoError(t, json.Unmarshal([]byte(`5000000000`), &d))
		require.Equal(t, 5*time.Second, d.Duration)
	})

	t.Run("should return an error for an invalid duration string", func(t *testing.T) {
		var d Duration
		require.Error(t, json.Unmarshal([]byte(`"invalid"`), &d))
	})

	t.Run("should return an error for an unsupported type", func(t *testing.T) {
		var d Duration
		require.Error(t, json.Unmarshal([]byte(`true`), &d))
	})

	t.Run("should return an error for invalid json", func(t *testing.T) {
		var d Duration
		require.Error(t, json.Unmarshal([]byte(`{`), &d))
	})
}

func TestGetHTTPClient(t *testing.T) {
	t.Run("should return the default client when options are nil", func(t *testing.T) {
		require.Equal(t, defaultHTTPClient, getHTTPClient(nil))
	})

	t.Run("should build a client from the provided options", func(t *testing.T) {
		enabled := true
		size := 10
		maxBytes := int64(1024)
		duration := &Duration{5 * time.Second}

		options := &HTTPClientOptions{Timeout: duration}
		options.Transport.TLSHandshakeTimeout = duration
		options.Transport.DisableKeepAlives = &enabled
		options.Transport.DisableCompression = &enabled
		options.Transport.MaxIdleConns = &size
		options.Transport.MaxIdleConnsPerHost = &size
		options.Transport.MaxConnsPerHost = &size
		options.Transport.IdleConnTimeout = duration
		options.Transport.ResponseHeaderTimeout = duration
		options.Transport.ExpectContinueTimeout = duration
		options.Transport.MaxResponseHeaderBytes = &maxBytes
		options.Transport.WriteBufferSize = &size
		options.Transport.ReadBufferSize = &size
		options.Transport.ForceAttemptHTTP2 = &enabled

		client := getHTTPClient(options)
		require.NotNil(t, client)
		require.NotEqual(t, defaultHTTPClient, client)
		require.Equal(t, 5*time.Second, client.Timeout)
	})
}
