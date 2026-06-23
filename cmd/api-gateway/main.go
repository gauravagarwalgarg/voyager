package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
)

func main() {
	// Initialize structured logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	logger.Info("Starting Voyager API Gateway",
		zap.String("version", "0.1.0"),
		zap.String("http_port", "8080"),
		zap.String("grpc_port", "9090"),
	)

	// HTTP server (REST → gRPC gateway)
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","service":"api-gateway","version":"0.1.0"}`)
	})

	// Metrics endpoint (Prometheus)
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		// TODO: prometheus.Handler().ServeHTTP(w, r)
		w.WriteHeader(http.StatusOK)
	})

	// TODO: Register gRPC-Gateway handlers for image and facts services
	// pb.RegisterImageServiceHandlerFromEndpoint(ctx, mux, imageServiceAddr, opts)
	// pb.RegisterFactsServiceHandlerFromEndpoint(ctx, mux, factsServiceAddr, opts)

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP server failed", zap.Error(err))
		}
	}()

	logger.Info("API Gateway running", zap.String("addr", ":8080"))

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited cleanly")
}
