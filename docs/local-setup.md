# Local Development Setup

Complete guide to running Voyager on your machine.

---

## Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Go | 1.22+ | See below |
| Docker | 20+ | [Docker Desktop](https://docs.docker.com/desktop/) (enable WSL2 integration) |
| Docker Compose | v2+ | Bundled with Docker Desktop |
| buf | latest | `go install github.com/bufbuild/buf/cmd/buf@latest` |
| golangci-lint | latest | `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest` |

### Install Go 1.22+ on Ubuntu/WSL

```bash
# Download
wget https://go.dev/dl/go1.22.5.linux-amd64.tar.gz

# Install (replaces any existing Go)
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.22.5.linux-amd64.tar.gz
rm go1.22.5.linux-amd64.tar.gz

# Add to PATH (append to ~/.bashrc)
echo 'export PATH=/usr/local/go/bin:$HOME/go/bin:$PATH' >> ~/.bashrc
source ~/.bashrc

# Remove old system Go if present
sudo apt remove golang-go golang-1.18-go -y 2>/dev/null

# Verify
go version   # → go1.22.5 linux/amd64
```

!!! warning "PATH Priority"
    Ubuntu 22.04 ships Go 1.18 at `/usr/bin/go`. You **must** ensure `/usr/local/go/bin`
    comes first in PATH, otherwise you'll get `atomic.Bool undefined` errors from dependencies
    that require Go 1.19+.

---

## Step-by-Step Setup

### 1. Clone the Repository

```bash
git clone https://github.com/GauravAgarwalGarg/voyager.git
cd voyager
```

### 2. Download Dependencies

```bash
go mod download
go mod tidy
```

### 3. Generate Protobuf Code

```bash
# Install buf if not already
go install github.com/bufbuild/buf/cmd/buf@latest

# Generate Go code from .proto files
make proto
```

!!! note "If `make proto` fails"
    Ensure `$HOME/go/bin` is in your PATH. Run `which buf` to verify.

### 4. Start Infrastructure

This spins up PostgreSQL, NSQ, MinIO, Prometheus, Grafana, and Tempo:

```bash
docker compose up -d
```

Verify all containers are healthy:

```bash
docker compose ps
```

Expected output all services `running`:

```
voyager-grafana      grafana/grafana:latest       running   0.0.0.0:3000->3000/tcp
voyager-minio        minio/minio:latest           running   0.0.0.0:9000-9001->9000-9001/tcp
voyager-nsqadmin     nsqio/nsq:latest             running   0.0.0.0:4171->4171/tcp
voyager-nsqd         nsqio/nsq:latest             running   0.0.0.0:4150-4151->4150-4151/tcp
voyager-nsqlookupd   nsqio/nsq:latest             running   0.0.0.0:4160-4161->4160-4161/tcp
voyager-postgres     postgres:16-alpine           running   0.0.0.0:5432->5432/tcp
voyager-prometheus   prom/prometheus:latest        running   0.0.0.0:9091->9090/tcp
voyager-tempo        grafana/tempo:latest          running   0.0.0.0:3200,4317-4318->3200,4317-4318/tcp
```

### 5. Run Services

Open **4 separate terminals** and run one service in each:

```bash
# Terminal 1 API Gateway (HTTP :8080)
make run-gateway

# Terminal 2 Image Service (gRPC :50051)
make run-image

# Terminal 3 Facts Service (gRPC :50052)
make run-facts

# Terminal 4 Worker (NSQ consumer)
make run-worker
```

You'll see structured JSON logs confirming each service started:

```json
{"level":"info","msg":"Starting Voyager API Gateway","http_port":"8080"}
{"level":"info","msg":"Starting Voyager Image Service","grpc_port":"50051"}
{"level":"info","msg":"Starting Voyager Facts Service","grpc_port":"50052"}
{"level":"info","msg":"Worker Service running, waiting for messages..."}
```

---

## Verify Everything Works

### Health Check

```bash
curl http://localhost:8080/health
```

```json
{"status":"ok","service":"api-gateway","version":"0.1.0"}
```

### Upload an Image

```bash
curl -X POST http://localhost:8080/v1/images \
  -F "file=@photo.jpg" \
  -F "title=Sunset in Himalayas" \
  -F "tags=nature,mountains"
```

### Get a Random Fact

```bash
curl http://localhost:8080/v1/facts/random
```

### Get the Feed

```bash
curl http://localhost:8080/v1/feed?limit=10
```

---

## Access Points

| Service | URL | Credentials |
|---------|-----|-------------|
| API Gateway (REST) | [http://localhost:8080](http://localhost:8080) | |
| Grafana Dashboards | [http://localhost:3000](http://localhost:3000) | admin / admin |
| Prometheus | [http://localhost:9091](http://localhost:9091) | |
| MinIO Console | [http://localhost:9001](http://localhost:9001) | minioadmin / minioadmin |
| NSQ Admin | [http://localhost:4171](http://localhost:4171) | |
| Tempo (traces) | [http://localhost:3200](http://localhost:3200) | |

---

## Hot Reload (Optional)

For auto-restart on code changes, install [Air](https://github.com/cosmtrek/air):

```bash
go install github.com/cosmtrek/air@latest

# Then instead of `make run-gateway`:
air -c .air.gateway.toml
```

---

## Running Tests

```bash
# Unit tests
make test

# Integration tests (requires docker compose up)
make test-int

# Coverage report
make test-coverage
open coverage.html
```

---

## Linting

```bash
# Install linter
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run
make lint
```

---

## Tear Down

```bash
# Stop services (Ctrl+C in each terminal)

# Stop infrastructure
docker compose down

# Remove volumes (full reset)
docker compose down -v

# Remove build artifacts
make clean
```

---

## Troubleshooting

### `atomic.Bool undefined` or `atomic.Pointer undefined`

Your Go version is too old. Run `go version` if it shows < 1.19, fix your PATH:

```bash
export PATH=/usr/local/go/bin:$HOME/go/bin:$PATH
go version  # must be 1.22+
```

### `missing go.sum entry`

```bash
go mod tidy
```

### Docker containers not starting

```bash
# Check logs
docker compose logs postgres
docker compose logs minio

# Reset everything
docker compose down -v
docker compose up -d
```

### Port already in use

```bash
# Find what's using the port
lsof -i :8080
kill -9 <PID>
```

### `buf: command not found`

```bash
go install github.com/bufbuild/buf/cmd/buf@latest
# Ensure ~/go/bin is in PATH
export PATH=$HOME/go/bin:$PATH
```
