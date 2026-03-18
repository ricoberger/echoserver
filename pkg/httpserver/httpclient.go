package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptrace"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

var defaultHTTPClient = &http.Client{
	Transport: otelhttp.NewTransport(
		http.DefaultTransport,
		otelhttp.WithClientTrace(func(ctx context.Context) *httptrace.ClientTrace {
			return otelhttptrace.NewClientTrace(ctx, otelhttptrace.WithoutSubSpans())
		}),
	),
}

type Duration struct {
	time.Duration
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		d.Duration = time.Duration(value)
		return nil
	case string:
		var err error
		d.Duration, err = time.ParseDuration(value)
		if err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("invalid duration")
	}
}

type HTTPClientOptions struct {
	Timeout   *Duration `json:"timeout"`
	Transport struct {
		TLSHandshakeTimeout    *Duration `json:"tlsHandshakeTimeout"`
		DisableKeepAlives      *bool     `json:"disableKeepAlives"`
		DisableCompression     *bool     `json:"disableCompression"`
		MaxIdleConns           *int      `json:"maxIdleConns"`
		MaxIdleConnsPerHost    *int      `json:"maxIdleConnsPerHost"`
		MaxConnsPerHost        *int      `json:"maxConnsPerHost"`
		IdleConnTimeout        *Duration `json:"idleConnTimeout"`
		ResponseHeaderTimeout  *Duration `json:"responseHeaderTimeout"`
		ExpectContinueTimeout  *Duration `json:"expectContinueTimeout"`
		MaxResponseHeaderBytes *int64    `json:"maxResponseHeaderBytes"`
		WriteBufferSize        *int      `json:"writeBufferSize"`
		ReadBufferSize         *int      `json:"readBufferSize"`
		ForceAttemptHTTP2      *bool     `json:"forceAttemptHTTP2"`
	} `json:"transport"`
}

func getHTTPClient(options *HTTPClientOptions) *http.Client {
	if options == nil {
		return defaultHTTPClient
	}

	transport := &http.Transport{}

	if options.Transport.TLSHandshakeTimeout != nil {
		transport.TLSHandshakeTimeout = options.Transport.TLSHandshakeTimeout.Duration
	}
	if options.Transport.DisableKeepAlives != nil {
		transport.DisableKeepAlives = *options.Transport.DisableKeepAlives
	}
	if options.Transport.DisableCompression != nil {
		transport.DisableCompression = *options.Transport.DisableCompression
	}
	if options.Transport.MaxIdleConns != nil {
		transport.MaxIdleConns = *options.Transport.MaxIdleConns
	}
	if options.Transport.MaxIdleConnsPerHost != nil {
		transport.MaxIdleConnsPerHost = *options.Transport.MaxIdleConnsPerHost
	}
	if options.Transport.MaxConnsPerHost != nil {
		transport.MaxConnsPerHost = *options.Transport.MaxConnsPerHost
	}
	if options.Transport.IdleConnTimeout != nil {
		transport.IdleConnTimeout = options.Transport.IdleConnTimeout.Duration
	}
	if options.Transport.ResponseHeaderTimeout != nil {
		transport.ResponseHeaderTimeout = options.Transport.ResponseHeaderTimeout.Duration
	}
	if options.Transport.ExpectContinueTimeout != nil {
		transport.ExpectContinueTimeout = options.Transport.ExpectContinueTimeout.Duration
	}
	if options.Transport.MaxResponseHeaderBytes != nil {
		transport.MaxResponseHeaderBytes = *options.Transport.MaxResponseHeaderBytes
	}
	if options.Transport.WriteBufferSize != nil {
		transport.WriteBufferSize = *options.Transport.WriteBufferSize
	}
	if options.Transport.ReadBufferSize != nil {
		transport.ReadBufferSize = *options.Transport.ReadBufferSize
	}
	if options.Transport.ForceAttemptHTTP2 != nil {
		transport.ForceAttemptHTTP2 = *options.Transport.ForceAttemptHTTP2
	}

	client := &http.Client{
		Transport: otelhttp.NewTransport(
			transport,
			otelhttp.WithClientTrace(func(ctx context.Context) *httptrace.ClientTrace {
				return otelhttptrace.NewClientTrace(ctx, otelhttptrace.WithoutSubSpans())
			}),
		),
	}

	if options.Timeout != nil {
		client.Timeout = options.Timeout.Duration
	}

	return client
}
