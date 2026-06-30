# Voyager 🚀

Cloud-native microservices platform with observability and event-driven architecture.

**Go · gRPC · Protobuf · NSQ · Kubernetes · Prometheus · Grafana · Tempo**

---

## Quick Start

```bash
# 1. Prerequisites: Go 1.22+, Docker, Docker Compose
# 2. Start infrastructure
docker compose up -d

# 3. Run services (4 terminals)
make run-gateway   # HTTP :8080
make run-image     # gRPC :50051
make run-facts     # gRPC :50052
make run-worker    # NSQ consumer

# 4. Test
curl http://localhost:8080/health
```

→ Full instructions: [Local Setup](local-setup.md)

---

## Documentation

### Setup & Deploy

- [Local Development Setup](local-setup.md) Prerequisites, step-by-step, troubleshooting
- [Production Deployment](deployment.md) Docker Compose, Kubernetes (k3s), Cloud (EKS/GKE)
- [For Go Beginners](getting-started.md) If you know Python/JS but not Go

### Architecture

- [System Architecture](architecture.md) High-level system design and component diagram
- [C4 Diagrams](c4-diagrams.md) Context, Container, Component, Code level views
- [Design Decisions](decisions.md) ADRs and trade-off analysis

### Service Design

- [API Gateway](design-gateway.md) Request routing, auth, rate limiting
- [Image Service](design-image-service.md) Upload, resize, CDN delivery
- [Messaging](design-messaging.md) Event-driven communication between services
- [Observability](design-observability.md) Metrics, tracing, logging stack

---

## Services

| Service | Responsibility | Port |
|---------|---------------|------|
| `api-gateway` | HTTP/REST → gRPC translation, auth, rate limiting | 8080 |
| `image-svc` | Image upload, retrieval, metadata CRUD | 50051 (gRPC) |
| `facts-svc` | Random fact generation and serving | 50052 (gRPC) |
| `worker-svc` | Async image processing (resize, thumbnail) | NSQ consumer |

## Access Points (Local)

| Service | URL |
|---------|-----|
| API Gateway | [http://localhost:8080](http://localhost:8080) |
| Grafana | [http://localhost:3000](http://localhost:3000) (admin/admin) |
| Prometheus | [http://localhost:9091](http://localhost:9091) |
| MinIO Console | [http://localhost:9001](http://localhost:9001) (minioadmin/minioadmin) |
| NSQ Admin | [http://localhost:4171](http://localhost:4171) |
| Tempo | [http://localhost:3200](http://localhost:3200) |
