package instrument

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"strconv"
	"time"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
)

type reporter struct {
	interceptors.CallMeta

	ctx context.Context
}

func (c *reporter) PostCall(err error, duration time.Duration) {
	if errors.Is(err, io.EOF) {
		err = nil
	}

	code := logging.DefaultErrorToCode(err)

	var serverAddress string
	var serverPortStr string
	if peer, ok := peer.FromContext(c.ctx); ok {
		serverAddress, serverPortStr, _ = net.SplitHostPort(peer.Addr.String())
	}
	serverPort := parsePort(serverPortStr)

	fields := []any{
		slog.String("rpc_grpc_status_code", code.String()),
		slog.String("rpc_method", c.Method),
		slog.String("rpc_service", c.Service),
		slog.String("rpc_system", "grpc"),
		slog.String("server_address", serverAddress),
		slog.Int("server_port", serverPort),
		slog.Duration("duration", duration),
	}
	if err != nil {
		fields = append(fields, slog.Any("error", err))
	}

	slog.InfoContext(c.ctx, "Call completed.", fields...)
}

func (c *reporter) PostMsgSend(payload any, err error, duration time.Duration) {
}

func (c *reporter) PostMsgReceive(payload any, err error, duration time.Duration) {
}

func reportable() interceptors.CommonReportableFunc {
	return func(ctx context.Context, c interceptors.CallMeta) (interceptors.Reporter, context.Context) {
		return &reporter{
			CallMeta: c,
			ctx:      ctx,
		}, ctx
	}
}

func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return interceptors.UnaryServerInterceptor(reportable())
}

func StreamServerInterceptor() grpc.StreamServerInterceptor {
	return interceptors.StreamServerInterceptor(reportable())
}

func parsePort(port string) int {
	p, _ := strconv.ParseInt(port, 10, 64)
	return int(p)
}
