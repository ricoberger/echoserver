package grpcserver

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"math/big"
	"strings"
	"time"

	"github.com/ricoberger/echoserver/pkg/grpcserver/middleware/requestid"
	pb "github.com/ricoberger/echoserver/pkg/grpcserver/proto"
	"github.com/ricoberger/echoserver/pkg/simulate"

	"github.com/fullstorydev/grpcurl"
	"github.com/jhump/protoreflect/grpcreflect"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	rpb "google.golang.org/grpc/reflection/grpc_reflection_v1"
	grpcstatus "google.golang.org/grpc/status"
)

type Echoserver interface {
	pb.EchoserverServer
}

type echoserver struct {
	pb.UnimplementedEchoserverServer
}

func NewEchoserver() Echoserver {
	return &echoserver{}
}

func (e *echoserver) Echo(ctx context.Context, r *pb.EchoRequest) (*pb.EchoResponse, error) {
	ctx, span := tracer.Start(ctx, "Echo")
	defer span.End()
	span.SetAttributes(attribute.Key("rpc.parameter.message").String(r.GetMessage()))
	slog.DebugContext(ctx, "Echo request received.", slog.String("message", r.GetMessage()))

	return &pb.EchoResponse{
		Message: r.GetMessage(),
	}, nil
}

func (e *echoserver) Status(ctx context.Context, r *pb.StatusRequest) (*pb.StatusResponse, error) {
	ctx, span := tracer.Start(ctx, "Status")
	defer span.End()
	span.SetAttributes(attribute.Key("rpc.parameter.status").String(r.GetStatus()))
	slog.DebugContext(ctx, "Status request received.", slog.String("status", r.GetStatus()))

	randomStatusCodes := []grpccodes.Code{grpccodes.OK, grpccodes.OK, grpccodes.OK, grpccodes.OK, grpccodes.OK, grpccodes.InvalidArgument, grpccodes.NotFound, grpccodes.Internal, grpccodes.Unavailable}

	if r.GetStatus() == "" || r.GetStatus() == "random" {
		index, err := rand.Int(rand.Reader, big.NewInt(int64(len(randomStatusCodes))))
		if err != nil {
			return &pb.StatusResponse{}, e.handleError(ctx, span, grpccodes.Internal, "Failed to generate random index.", err)
		}

		status := randomStatusCodes[index.Int64()]

		return &pb.StatusResponse{}, grpcstatus.Error(status, status.String())
	}

	statusCodesMap := map[string]grpccodes.Code{
		"OK":                  grpccodes.OK,
		"CANCELLED":           grpccodes.Canceled,
		"UNKNOWN":             grpccodes.Unknown,
		"INVALID_ARGUMENT":    grpccodes.InvalidArgument,
		"DEADLINE_EXCEEDED":   grpccodes.DeadlineExceeded,
		"NOT_FOUND":           grpccodes.NotFound,
		"ALREADY_EXISTS":      grpccodes.AlreadyExists,
		"PERMISSION_DENIED":   grpccodes.PermissionDenied,
		"RESOURCE_EXHAUSTED":  grpccodes.ResourceExhausted,
		"FAILED_PRECONDITION": grpccodes.FailedPrecondition,
		"ABORTED":             grpccodes.Aborted,
		"OUT_OF_RANGE":        grpccodes.OutOfRange,
		"UNIMPLEMENTED":       grpccodes.Unimplemented,
		"INTERNAL":            grpccodes.Internal,
		"UNAVAILABLE":         grpccodes.Unavailable,
		"DATA_LOSS":           grpccodes.DataLoss,
		"UNAUTHENTICATED":     grpccodes.Unauthenticated,
	}

	if status, ok := statusCodesMap[r.GetStatus()]; ok {
		return &pb.StatusResponse{}, grpcstatus.Error(status, status.String())
	}

	return &pb.StatusResponse{}, e.handleError(ctx, span, grpccodes.Internal, "Invalid 'status' parameter.", fmt.Errorf("unknown status %q", r.GetStatus()))
}

func (e *echoserver) Request(ctx context.Context, r *pb.RequestRequest) (*pb.RequestResponse, error) {
	ctx, span := tracer.Start(ctx, "Request")
	defer span.End()
	span.SetAttributes(attribute.Key("rpc.parameter.uri").String(r.GetUri()))
	span.SetAttributes(attribute.Key("rpc.parameter.method").String(r.GetMethod()))
	span.SetAttributes(attribute.Key("rpc.parameter.message").String(r.GetMessage()))
	slog.DebugContext(ctx, "Request request received.", slog.String("uri", r.GetUri()), slog.String("method", r.GetMethod()), slog.String("message", r.GetMessage()))

	conn, err := grpc.NewClient(r.GetUri(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
	if err != nil {
		return &pb.RequestResponse{}, e.handleError(ctx, span, grpccodes.Internal, "Failed to create grpc client.", err)
	}
	defer conn.Close()

	reflectionClient := grpcreflect.NewClientV1(ctx, rpb.NewServerReflectionClient(conn))
	defer reflectionClient.Reset()

	rf, formatter, err := grpcurl.RequestParserAndFormatter(
		grpcurl.Format("json"),
		grpcurl.DescriptorSourceFromServer(ctx, reflectionClient),
		strings.NewReader(r.GetMessage()),
		grpcurl.FormatOptions{EmitJSONDefaultFields: true},
	)
	if err != nil {
		return &pb.RequestResponse{}, e.handleError(ctx, span, grpccodes.Internal, "Failed to create request parser and formatter.", err)
	}

	var output bytes.Buffer
	var status grpcstatus.Status
	h := &grpcurl.DefaultEventHandler{
		Out:            &output,
		Formatter:      formatter,
		VerbosityLevel: 0,
		Status:         &status,
	}

	var headers []string
	for key, value := range r.GetHeaders() {
		headers = append(headers, fmt.Sprintf("%s: %s", key, value))
	}
	if requestId := requestid.Get(ctx); requestId != "" {
		headers = append(headers, fmt.Sprintf("%s: %s", requestid.RequestIDHeader, requestId))
	}
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		requestIds := md.Get(requestid.RequestIDHeader)
		if len(requestIds) > 0 {
			headers = append(headers, fmt.Sprintf("%s: %s", requestid.RequestIDHeader, requestIds[0]))
		}
	}

	err = grpcurl.InvokeRPC(
		ctx,
		grpcurl.DescriptorSourceFromServer(ctx, reflectionClient),
		conn,
		r.GetMethod(),
		headers,
		h,
		rf.Next,
	)
	if err != nil {
		if errStatus, ok := grpcstatus.FromError(err); ok {
			h.Status = errStatus
		} else {
			return &pb.RequestResponse{}, e.handleError(ctx, span, grpccodes.Internal, "Invoke failed.", err)
		}
	}

	return &pb.RequestResponse{
		Message: output.String(),
	}, grpcstatus.Error(h.Status.Code(), h.Status.Message())
}

// handleError logs the given error, records it on the span and returns a gRPC
// error with the given status code.
func (e *echoserver) handleError(ctx context.Context, span trace.Span, code grpccodes.Code, message string, err error) error {
	slog.ErrorContext(ctx, message, slog.Any("error", err))
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())

	return grpcstatus.Error(code, err.Error())
}

func (e *echoserver) Simulate(ctx context.Context, r *pb.SimulateRequest) (*pb.SimulateResponse, error) {
	ctx, span := tracer.Start(ctx, "Simulate")
	defer span.End()
	span.SetAttributes(attribute.Key("rpc.parameter.type").String(r.GetType()))
	span.SetAttributes(attribute.Key("rpc.parameter.duration").String(r.GetDuration()))
	slog.DebugContext(ctx, "Simulate request received.", slog.String("type", r.GetType()), slog.String("duration", r.GetDuration()))

	if r.GetType() == "" {
		return &pb.SimulateResponse{}, e.handleError(ctx, span, grpccodes.InvalidArgument, "Invalid 'type' parameter.", fmt.Errorf("type parameter is missing"))
	}

	if r.GetDuration() == "" {
		return &pb.SimulateResponse{}, e.handleError(ctx, span, grpccodes.InvalidArgument, "Invalid 'duration' parameter.", fmt.Errorf("duration parameter is missing"))
	}

	duration, err := time.ParseDuration(r.GetDuration())
	if err != nil {
		return &pb.SimulateResponse{}, e.handleError(ctx, span, grpccodes.InvalidArgument, "Invalid 'duration' parameter.", fmt.Errorf("failed to parse 'duration' parameter: %w", err))
	}

	var message string
	switch r.GetType() {
	case "cpu":
		simulate.CPU(ctx, duration)
		message = fmt.Sprintf("simulated %q for %s", r.GetType(), duration)
	case "memory":
		if r.GetSize() <= 0 {
			return &pb.SimulateResponse{}, e.handleError(ctx, span, grpccodes.InvalidArgument, "Invalid 'size' parameter.", fmt.Errorf("size parameter must be greater than zero"))
		}
		span.SetAttributes(attribute.Key("rpc.parameter.size").Int64(r.GetSize()))
		simulate.Memory(ctx, duration, int(r.GetSize()))
		message = fmt.Sprintf("simulated %q for %s (%d bytes)", r.GetType(), duration, r.GetSize())
	case "goroutines":
		if r.GetCount() <= 0 {
			return &pb.SimulateResponse{}, e.handleError(ctx, span, grpccodes.InvalidArgument, "Invalid 'count' parameter.", fmt.Errorf("count parameter must be greater than zero"))
		}
		span.SetAttributes(attribute.Key("rpc.parameter.count").Int64(r.GetCount()))
		simulate.Goroutines(ctx, duration, int(r.GetCount()))
		message = fmt.Sprintf("simulated %q for %s (%d goroutines)", r.GetType(), duration, r.GetCount())
	case "mutex":
		if r.GetWorkers() <= 0 {
			return &pb.SimulateResponse{}, e.handleError(ctx, span, grpccodes.InvalidArgument, "Invalid 'workers' parameter.", fmt.Errorf("workers parameter must be greater than zero"))
		}
		span.SetAttributes(attribute.Key("rpc.parameter.workers").Int64(r.GetWorkers()))
		simulate.Mutex(ctx, duration, int(r.GetWorkers()))
		message = fmt.Sprintf("simulated %q for %s (%d workers)", r.GetType(), duration, r.GetWorkers())
	case "block":
		if r.GetWorkers() <= 0 {
			return &pb.SimulateResponse{}, e.handleError(ctx, span, grpccodes.InvalidArgument, "Invalid 'workers' parameter.", fmt.Errorf("workers parameter must be greater than zero"))
		}
		span.SetAttributes(attribute.Key("rpc.parameter.workers").Int64(r.GetWorkers()))
		simulate.Block(ctx, duration, int(r.GetWorkers()))
		message = fmt.Sprintf("simulated %q for %s (%d workers)", r.GetType(), duration, r.GetWorkers())
	default:
		return &pb.SimulateResponse{}, e.handleError(ctx, span, grpccodes.InvalidArgument, "Unknown simulation type.", fmt.Errorf("unknown simulation type %q", r.GetType()))
	}

	return &pb.SimulateResponse{
		Message: message,
	}, nil
}
