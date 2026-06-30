package main

import (
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	logger.Info("Starting Voyager Worker Service",
		zap.String("version", "0.1.0"),
	)

	// TODO: Connect to NSQ
	// consumer, err := nsq.NewConsumer("image.uploaded", "worker", nsq.NewConfig())
	// consumer.AddHandler(nsq.HandlerFunc(processImage))
	// consumer.ConnectToNSQLookupd("nsqlookupd:4161")

	logger.Info("Worker Service running, waiting for messages...")

	// Wait for shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down Worker Service...")
	// consumer.Stop()
	logger.Info("Worker Service stopped")
}
