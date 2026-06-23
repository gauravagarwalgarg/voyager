package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	logger.Info("Starting Voyager Image Service",
		zap.String("version", "0.1.0"),
		zap.String("grpc_port", "50051"),
	)

	// Create gRPC server with interceptors
	srv := grpc.NewServer(
	// TODO: Add interceptors
	// grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor()),
	// grpc.StreamInterceptor(otelgrpc.StreamServerInterceptor()),
	)

	// Register health check
	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(srv, healthSrv)

	// TODO: Register ImageService implementation
	// imagev1.RegisterImageServiceServer(srv, imageServer)

	// Enable reflection for grpcurl/grpcui
	reflection.Register(srv)

	// Listen
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		logger.Fatal("Failed to listen", zap.Error(err))
	}

	// Serve in goroutine
	go func() {
		logger.Info("Image Service listening", zap.String("addr", ":50051"))
		if err := srv.Serve(lis); err != nil {
			logger.Fatal("gRPC server failed", zap.Error(err))
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down Image Service...")
	srv.GracefulStop()
	logger.Info("Image Service stopped")

	_ = fmt.Sprintf("unused") // suppress import error during scaffold
}
