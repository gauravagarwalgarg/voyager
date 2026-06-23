# Voyager

A cloud-native image sharing platform with random facts built for scale.

**Go · gRPC · Protobuf · NSQ · Kubernetes · Prometheus · Grafana · Tempo**

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                            Kubernetes Cluster                            │
│                                                                         │
│  ┌──────────────┐     ┌──────────────┐     ┌──────────────┐           │
│  │  API Gateway │────▶│  Image Svc   │────▶│  Object Store│           │
│  │  (gRPC/REST) │     │  (gRPC)      │     │  (MinIO/S3)  │           │
│  └──────┬───────┘     └──────┬───────┘     └──────────────┘           │
│         │                    │                                          │
│         │              ┌─────▼──────┐                                  │
│         │              │    NSQ     │ (async messaging)                 │
│         │              └─────┬──────┘                                  │
│         │                    │                                          │
│  ┌──────▼───────┐     ┌─────▼──────┐     ┌──────────────┐           │
│  │  Facts Svc   │     │  Worker Svc │     │  PostgreSQL  │           │
│  │  (gRPC)      │     │  (consumer) │     │  (metadata)  │           │
│  └──────────────┘     └─────────────┘     └──────────────┘           │
│                                                                         │
│  ┌─────────────────────────────────────────────────────────────┐       │
│  │                    Observability Stack                        │       │
│  │  Prometheus (metrics) · Grafana (dashboards) · Tempo (traces)│       │
│  └─────────────────────────────────────────────────────────────┘       │
└─────────────────────────────────────────────────────────────────────────┘
```

## Services

| Service | Responsibility | Protocol | Port |
|---------|---------------|----------|------|
| `api-gateway` | HTTP/REST → gRPC translation, auth, rate limiting | HTTP + gRPC | 8080, 9090 |
| `image-svc` | Image upload, retrieval, metadata CRUD | gRPC | 50051 |
| `facts-svc` | Random fact generation and serving | gRPC | 50052 |
| `worker-svc` | Async image processing (resize, thumbnail, EXIF) | NSQ consumer | |

## Tech Stack

| Layer | Technology | Why |
|-------|-----------|-----|
| Language | Go 1.22+ | Performance, concurrency, small binaries |
| RPC | gRPC + Protobuf | Type-safe, fast, streaming support |
| Messaging | NSQ | Distributed, no single point of failure, simple |
| Database | PostgreSQL | Relational metadata, JSONB for flexible fields |
| Object Store | MinIO (S3-compatible) | Local dev, drop-in for AWS S3 in prod |
| Orchestration | Kubernetes (k3s locally) | Industry standard, scales from 1 to 1000 nodes |
| Metrics | Prometheus | Pull-based, PromQL, service discovery |
| Dashboards | Grafana | Visualization, alerting, multi-datasource |
| Tracing | Grafana Tempo | Distributed tracing, OpenTelemetry native |
| Service Mesh | (future: Istio/Linkerd) | mTLS, traffic control |

## Project Layout

```
voyager/
├── cmd/                           # Service entry points
│   ├── api-gateway/main.go
│   ├── image-svc/main.go
│   ├── facts-svc/main.go
│   └── worker-svc/main.go
├── proto/                         # Protobuf definitions (source of truth)
│   ├── image/v1/image.proto
│   ├── facts/v1/facts.proto
│   └── buf.yaml
├── internal/                      # Private application code
│   ├── gateway/                   # API gateway logic
│   ├── image/                     # Image service domain
│   ├── facts/                     # Facts service domain
│   ├── worker/                    # Async worker logic
│   ├── middleware/                # Auth, logging, tracing, rate limit
│   └── pkg/                       # Shared internal libs
│       ├── config/                # Viper config loader
│       ├── database/              # PostgreSQL connection pool
│       ├── messaging/             # NSQ producer/consumer
│       ├── observability/         # OTEL + Prometheus setup
│       └── storage/               # MinIO/S3 client
├── deploy/                        # Kubernetes manifests
│   ├── base/                      # Kustomize base
│   │   ├── api-gateway.yaml
│   │   ├── image-svc.yaml
│   │   ├── facts-svc.yaml
│   │   ├── worker-svc.yaml
│   │   ├── nsq.yaml
│   │   ├── postgres.yaml
│   │   ├── minio.yaml
│   │   └── kustomization.yaml
│   ├── observability/             # Prometheus + Grafana + Tempo
│   │   ├── prometheus.yaml
│   │   ├── grafana.yaml
│   │   ├── tempo.yaml
│   │   └── kustomization.yaml
│   └── overlays/
│       ├── local/                 # k3s / Docker Desktop overrides
│       └── production/            # Cloud overrides (HPA, ingress)
├── configs/                       # Service configs (YAML)
│   ├── api-gateway.yaml
│   ├── image-svc.yaml
│   ├── facts-svc.yaml
│   └── worker-svc.yaml
├── scripts/                       # Dev scripts
│   ├── setup-k3s.sh
│   ├── seed-data.sh
│   └── gen-proto.sh
├── test/                          # Integration & e2e tests
│   ├── integration/
│   └── e2e/
├── docs/                          # Architecture docs
│   ├── architecture.md
│   ├── api-reference.md
│   ├── deployment.md
│   └── observability.md
├── docker-compose.yml             # Local dev (all services)
├── Dockerfile                     # Multi-stage build
├── Makefile                       # Build, test, deploy commands
├── go.mod
├── go.sum
└── buf.gen.yaml                   # Protobuf code generation config
```

## Quick Start (Local)

### Prerequisites
- Go 1.22+
- Docker + Docker Compose
- `buf` CLI (protobuf tooling)
- `k3s` or Docker Desktop with Kubernetes enabled

### Run Everything Locally

```bash
# 1. Generate protobuf code
make proto

# 2. Start infrastructure (Postgres, NSQ, MinIO, Prometheus, Grafana, Tempo)
docker-compose up -d

# 3. Run services (hot-reload with air)
make run-all

# 4. Or deploy to local k3s
make deploy-local
```

### Access Points

| Service | URL |
|---------|-----|
| API Gateway (REST) | http://localhost:8080 |
| API Gateway (gRPC) | localhost:9090 |
| Grafana | http://localhost:3000 |
| Prometheus | http://localhost:9091 |
| Tempo (traces) | http://localhost:3200 |
| MinIO Console | http://localhost:9001 |
| NSQ Admin | http://localhost:4171 |

## API Examples

### Upload Image

```bash
curl -X POST http://localhost:8080/v1/images \
  -F "file=@photo.jpg" \
  -F "title=Sunset in Himalayas" \
  -F "tags=nature,mountains"
```

### Get Random Fact

```bash
curl http://localhost:8080/v1/facts/random
```

### List Images with Facts

```bash
curl http://localhost:8080/v1/feed?limit=10
```

## Observability

### Metrics (Prometheus)

Every service exposes `/metrics` with:
- Request latency histograms (p50, p95, p99)
- Request count by method/status
- Active gRPC connections
- NSQ queue depth
- Image processing duration
- Database connection pool stats

### Tracing (Tempo)

Full distributed traces across:
```
HTTP Request → API Gateway → gRPC call → Image Service → DB query
                                      → NSQ publish → Worker → MinIO upload
```

### Dashboards (Grafana)

Pre-built dashboards:
- **Service Overview**: RED metrics (Rate, Errors, Duration) per service
- **Infrastructure**: CPU, memory, pod restarts, network
- **NSQ Queues**: Depth, in-flight, requeue rate
- **Database**: Connection pool, query latency, lock waits

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| gRPC between services | Type safety, streaming, 10x faster than REST for internal calls |
| REST at the gateway | Browser/mobile clients need HTTP, gateway translates to gRPC |
| NSQ over Kafka | Simpler to operate, no ZooKeeper, good enough for this scale |
| PostgreSQL over MongoDB | Relational integrity for image metadata, JSONB for flexibility |
| MinIO locally | S3-compatible, swap for real S3 in production with zero code changes |
| Kustomize over Helm | Simpler, no templating language, overlays for env-specific config |
| OpenTelemetry | Vendor-neutral, works with Tempo/Jaeger/Zipkin |

## Make Targets

```bash
make proto          # Generate Go code from .proto files
make build          # Build all service binaries
make test           # Run unit tests
make test-int       # Run integration tests
make lint           # golangci-lint
make docker-build   # Build Docker images
make deploy-local   # Deploy to local k3s
make deploy-obs     # Deploy observability stack
make run-all        # Run all services locally (no k8s)
make seed           # Seed database with sample data
make clean          # Remove build artifacts
```

## Contributing

1. Fork and clone
2. `make proto` to generate protobuf code
3. `docker-compose up -d` to start dependencies
4. Make your changes
5. `make test && make lint`
6. Submit PR

## License

MIT
