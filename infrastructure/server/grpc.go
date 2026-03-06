package server

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "weather-service/api/proto"
	"weather-service/infrastructure/config"
	grpchandler "weather-service/internal/adapter/grpc"
	"weather-service/internal/usecase"
)

type GRPCServer struct {
	server   *grpc.Server
	listener net.Listener
}

func NewGRPCServer(cfg config.Config, uc *usecase.WeatherUseCase) (*GRPCServer, error) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.GRPCPort))
	if err != nil {
		return nil, fmt.Errorf("listen on port %s: %w", cfg.GRPCPort, err)
	}

	grpcServer := grpc.NewServer()
	handler := grpchandler.NewWeatherHandler(uc)
	pb.RegisterWeatherServiceServer(grpcServer, handler)
	reflection.Register(grpcServer)

	return &GRPCServer{
		server:   grpcServer,
		listener: lis,
	}, nil
}

func (s *GRPCServer) Start(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		log.Infof("gRPC server listening on %s", s.listener.Addr().String())
		if err := s.server.Serve(s.listener); err != nil {
			errCh <- err
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Infof("received signal %v, shutting down...", sig)
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
		log.Info("context cancelled, shutting down...")
	}

	return s.shutdown()
}

func (s *GRPCServer) shutdown() error {
	done := make(chan struct{})

	go func() {
		s.server.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		log.Info("server stopped gracefully")
		return nil
	case <-time.After(30 * time.Second):
		log.Warn("graceful shutdown timed out, forcing stop")
		s.server.Stop()
		return nil
	}
}
