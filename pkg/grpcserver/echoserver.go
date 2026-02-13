package grpcserver

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"math/big"
	"strings"

	"github.com/ricoberger/echoserver/pkg/grpcserver/middleware/requestid"
	pb "github.com/ricoberger/echoserver/pkg/grpcserver/proto"

	"github.com/fullstorydev/grpcurl"
	"github.com/jhump/protoreflect/grpcreflect"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
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
	_, span := tracer.Start(ctx, "Echo")
	defer span.End()
	span.SetAttributes(attribute.Key("message").String(r.GetMessage()))

	return &pb.EchoResponse{
		Message: r.GetMessage(),
	}, nil
}

func (e *echoserver) Status(ctx context.Context, r *pb.StatusRequest) (*pb.StatusResponse, error) {
	_, span := tracer.Start(ctx, "Status")
	defer span.End()
	span.SetAttributes(attribute.Key("status").String(r.GetStatus()))

	randomStatusCodes := []grpccodes.Code{grpccodes.OK, grpccodes.OK, grpccodes.OK, grpccodes.OK, grpccodes.OK, grpccodes.InvalidArgument, grpccodes.NotFound, grpccodes.Internal, grpccodes.Unavailable}

	if r.GetStatus() == "" || r.GetStatus() == "random" {
		index, err := rand.Int(rand.Reader, big.NewInt(int64(len(randomStatusCodes))))
		if err != nil {
			slog.ErrorContext(ctx, "Failed to generate random index.", slog.Any("error", err))
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())

			return &pb.StatusResponse{}, grpcstatus.Error(grpccodes.Internal, err.Error())
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

	return &pb.StatusResponse{}, grpcstatus.Error(grpccodes.Internal, "Unknown status parameter")
}

func (e *echoserver) Request(ctx context.Context, r *pb.RequestRequest) (*pb.RequestResponse, error) {
	_, span := tracer.Start(ctx, "Request")
	defer span.End()
	span.SetAttributes(attribute.Key("uri").String(r.GetUri()))
	span.SetAttributes(attribute.Key("method").String(r.GetMethod()))
	span.SetAttributes(attribute.Key("message").String(r.GetMessage()))

	conn, _ := grpc.NewClient(r.GetUri(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
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
		slog.ErrorContext(ctx, "Failed to create request parser and formatter.", slog.Any("error", err))
		return &pb.RequestResponse{}, grpcstatus.Error(grpccodes.Internal, err.Error())
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
			slog.ErrorContext(ctx, "Invoke failed.", slog.Any("error", err))
			return &pb.RequestResponse{}, grpcstatus.Error(grpccodes.Internal, err.Error())
		}
	}

	return &pb.RequestResponse{
		Message: output.String(),
	}, grpcstatus.Error(h.Status.Code(), h.Status.Message())
}
