module github.com/GauravAgarwalGarg/voyager

go 1.22

require (
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.20.0
	github.com/minio/minio-go/v7 v7.0.70
	github.com/nsqio/go-nsq v1.1.0
	go.opentelemetry.io/otel v1.27.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.27.0
	go.opentelemetry.io/otel/sdk v1.27.0
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.52.0
	google.golang.org/grpc v1.64.0
	google.golang.org/protobuf v1.34.2
	github.com/jackc/pgx/v5 v5.6.0
	github.com/prometheus/client_golang v1.19.1
	github.com/spf13/viper v1.19.0
	go.uber.org/zap v1.27.0
)
