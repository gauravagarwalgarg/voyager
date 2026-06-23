package main

import (
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

	logger.Info("Starting Voyager Facts Service",
		zap.String("version", "0.1.0"),
		zap.String("grpc_port", "50052"),
	)

	srv := grpc.NewServer()

	// Register health check
	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(srv, healthSrv)

	// TODO: Register FactsService implementation
	// factsv1.RegisterFactsServiceServer(srv, factsServer)

	reflection.Register(srv)

	lis, err := net.Listen("tcp", ":50052")
	if err != nil {
		logger.Fatal("Failed to listen", zap.Error(err))
	}

	go func() {
		logger.Info("Facts Service listening", zap.String("addr", ":50052"))
		if err := srv.Serve(lis); err != nil {
			logger.Fatal("gRPC server failed", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down Facts Service...")
	srv.GracefulStop()
	logger.Info("Facts Service stopped")
}
