package grpcserver

import (
	"context"
	"net"
	"testing"

	pb "github.com/ricoberger/echoserver/pkg/grpcserver/proto"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	grpcstatus "google.golang.org/grpc/status"
)

func TestEcho(t *testing.T) {
	t.Run("should echo the message", func(t *testing.T) {
		e := NewEchoserver()

		resp, err := e.Echo(context.Background(), &pb.EchoRequest{Message: "hello"})
		require.NoError(t, err)
		require.Equal(t, "hello", resp.GetMessage())
	})
}

func TestStatus(t *testing.T) {
	e := NewEchoserver()

	t.Run("should return no error for the OK status", func(t *testing.T) {
		_, err := e.Status(context.Background(), &pb.StatusRequest{Status: "OK"})
		require.NoError(t, err)
	})

	t.Run("should return the matching status code", func(t *testing.T) {
		_, err := e.Status(context.Background(), &pb.StatusRequest{Status: "NOT_FOUND"})
		require.Equal(t, grpccodes.NotFound, grpcstatus.Code(err))
	})

	t.Run("should return an internal error for an unknown status", func(t *testing.T) {
		_, err := e.Status(context.Background(), &pb.StatusRequest{Status: "DOES_NOT_EXIST"})
		require.Equal(t, grpccodes.Internal, grpcstatus.Code(err))
	})

	t.Run("should handle a random status", func(t *testing.T) {
		// A random status may resolve to OK (no error) or to an error status,
		// so we only ensure the call does not panic and returns a response.
		resp, _ := e.Status(context.Background(), &pb.StatusRequest{Status: "random"})
		require.NotNil(t, resp)
	})
}

func TestSimulate(t *testing.T) {
	e := NewEchoserver()

	t.Run("should simulate each type", func(t *testing.T) {
		requests := []*pb.SimulateRequest{
			{Type: "cpu", Duration: "10ms"},
			{Type: "memory", Duration: "10ms", Size: 1024},
			{Type: "goroutines", Duration: "10ms", Count: 4},
			{Type: "mutex", Duration: "10ms", Workers: 4},
			{Type: "block", Duration: "10ms", Workers: 4},
		}

		for _, request := range requests {
			resp, err := e.Simulate(context.Background(), request)
			require.NoError(t, err)
			require.Contains(t, resp.GetMessage(), request.GetType())
		}
	})

	t.Run("should return an error when the type is missing", func(t *testing.T) {
		_, err := e.Simulate(context.Background(), &pb.SimulateRequest{Duration: "10ms"})
		require.Equal(t, grpccodes.InvalidArgument, grpcstatus.Code(err))
	})

	t.Run("should return an error when the duration is missing", func(t *testing.T) {
		_, err := e.Simulate(context.Background(), &pb.SimulateRequest{Type: "cpu"})
		require.Equal(t, grpccodes.InvalidArgument, grpcstatus.Code(err))
	})

	t.Run("should return an error when the duration is invalid", func(t *testing.T) {
		_, err := e.Simulate(context.Background(), &pb.SimulateRequest{Type: "cpu", Duration: "invalid"})
		require.Equal(t, grpccodes.InvalidArgument, grpcstatus.Code(err))
	})

	t.Run("should return an error when a required magnitude parameter is missing", func(t *testing.T) {
		requests := []*pb.SimulateRequest{
			{Type: "memory", Duration: "10ms"},
			{Type: "goroutines", Duration: "10ms"},
			{Type: "mutex", Duration: "10ms"},
			{Type: "block", Duration: "10ms"},
		}

		for _, request := range requests {
			_, err := e.Simulate(context.Background(), request)
			require.Equal(t, grpccodes.InvalidArgument, grpcstatus.Code(err))
		}
	})

	t.Run("should return an error when the type is unknown", func(t *testing.T) {
		_, err := e.Simulate(context.Background(), &pb.SimulateRequest{Type: "unknown", Duration: "10ms"})
		require.Equal(t, grpccodes.InvalidArgument, grpcstatus.Code(err))
	})
}

func TestRequest(t *testing.T) {
	//nolint:noctx
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	pb.RegisterEchoserverServer(grpcServer, NewEchoserver())
	reflection.Register(grpcServer)

	go func() {
		_ = grpcServer.Serve(listener)
	}()
	defer grpcServer.Stop()

	e := NewEchoserver()

	t.Run("should invoke the method on the target server", func(t *testing.T) {
		resp, err := e.Request(context.Background(), &pb.RequestRequest{
			Uri:     listener.Addr().String(),
			Method:  "Echoserver.Echo",
			Message: `{"message":"hi"}`,
		})
		require.NoError(t, err)
		require.Contains(t, resp.GetMessage(), "hi")
	})

	t.Run("should return an error for an unknown method", func(t *testing.T) {
		_, err := e.Request(context.Background(), &pb.RequestRequest{
			Uri:     listener.Addr().String(),
			Method:  "Echoserver.DoesNotExist",
			Message: `{}`,
		})
		require.Error(t, err)
	})

	t.Run("should return an error for an invalid uri", func(t *testing.T) {
		require.NotPanics(t, func() {
			_, err := e.Request(context.Background(), &pb.RequestRequest{
				Uri:     "unknown-scheme://invalid",
				Method:  "Echoserver.Echo",
				Message: `{}`,
			})
			require.Error(t, err)
		})
	})
}
