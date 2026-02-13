package grpcserver

//go:generate protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative proto/echoserver.proto

import (
	"context"
	"log/slog"
	"net"

	"github.com/ricoberger/echoserver/pkg/grpcserver/middleware/instrument"
	"github.com/ricoberger/echoserver/pkg/grpcserver/middleware/requestid"
	pb "github.com/ricoberger/echoserver/pkg/grpcserver/proto"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

var (
	tracer = otel.Tracer("grpcserver")
)

type Config struct {
	Address string `env:"ADDRESS" default:":8081" help:"The address where the gRPC server should listen on."`
}

type Server interface {
	Start()
	Stop()
}

type server struct {
	address    string
	grpcServer *grpc.Server
}

func (s *server) Start() {
	listenConfig := &net.ListenConfig{}
	listener, err := listenConfig.Listen(context.Background(), "tcp", s.address)
	if err != nil {
		slog.Error("Failed to create listener.", slog.Any("error", err))
	}
	slog.Info("Start server...", slog.String("address", listener.Addr().String()))

	if err := s.grpcServer.Serve(listener); err != nil {
		slog.Error("Server died unexpected.", slog.Any("error", err))
	}
}

func (s *server) Stop() {
	s.grpcServer.GracefulStop()
}

func New(config Config) Server {
	echoserver := NewEchoserver()

	grpcOptions := []grpc.ServerOption{
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(
			requestid.UnaryServerInterceptor(),
			instrument.UnaryServerInterceptor(),
		),
		grpc.ChainStreamInterceptor(
			requestid.StreamServerInterceptor(),
			instrument.StreamServerInterceptor(),
		),
	}

	grpcServer := grpc.NewServer(grpcOptions...)
	pb.RegisterEchoserverServer(grpcServer, echoserver)
	reflection.Register(grpcServer)
	grpc_health_v1.RegisterHealthServer(grpcServer, health.NewServer())

	return &server{
		address:    config.Address,
		grpcServer: grpcServer,
	}
}
