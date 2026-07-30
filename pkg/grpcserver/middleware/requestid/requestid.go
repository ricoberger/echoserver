package requestid

import (
	"context"

	id "github.com/ricoberger/echoserver/pkg/requestid"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type ctxKeyRequestID int

const RequestIDKey ctxKeyRequestID = 0

const RequestIDHeader = id.Header

func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		requestID := ""
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if values := md.Get(RequestIDHeader); len(values) > 0 {
				requestID = values[0]
			}
		}
		if requestID == "" {
			requestID = id.Generate()
		}
		ctx = context.WithValue(ctx, RequestIDKey, requestID)
		return handler(ctx, req)
	}
}

func StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := ss.Context()
		requestID := ""
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if values := md.Get(RequestIDHeader); len(values) > 0 {
				requestID = values[0]
			}
		}
		if requestID == "" {
			requestID = id.Generate()
		}
		ctx = context.WithValue(ctx, RequestIDKey, requestID)
		wrapped := &wrappedServerStream{
			ServerStream: ss,
			ctx:          ctx,
		}
		return handler(srv, wrapped)
	}
}

type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}

func Get(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if reqID, ok := ctx.Value(RequestIDKey).(string); ok {
		return reqID
	}
	return ""
}
