# Voyager Observability Guide

This document describes how to observe the Voyager system running locally via `docker compose up`.

---

## Service Endpoints

| Service       | URL                          | Purpose                     |
|---------------|------------------------------|-----------------------------|
| API Gateway   | http://localhost:8080        | REST API + Frontend         |
| Frontend      | http://localhost:3001        | Nginx-served SPA (alt)      |
| Grafana       | http://localhost:3000        | Dashboards & visualization  |
| Prometheus    | http://localhost:9091        | Metrics collection          |
| Tempo         | http://localhost:3200        | Distributed tracing         |
| NSQ Admin     | http://localhost:4171        | Message queue monitoring    |
| MinIO Console | http://localhost:9001        | Object storage browser      |
| PostgreSQL    | localhost:5432               | Database (voyager/voyager-dev) |

---

## Grafana

**Login:** `admin` / `admin` (or anonymous access enabled)

### Pre-configured Data Sources

- **Prometheus** → `http://prometheus:9090`
- **Tempo** → `http://tempo:3200`

### Dashboards to Create/Import

#### 1. API Gateway Overview

Build a dashboard with these panels:

- **Request Rate** Graph panel
  ```promql
  rate(voyager_gateway_requests_total[5m])
  ```

- **Request Duration (p95)** Graph panel
  ```promql
  histogram_quantile(0.95, rate(voyager_gateway_request_duration_seconds_bucket[5m]))
  ```

- **Error Rate** Stat panel
  ```promql
  sum(rate(voyager_gateway_requests_total{status=~"5.."}[5m])) / sum(rate(voyager_gateway_requests_total[5m]))
  ```

- **Active Connections** Gauge
  ```promql
  voyager_gateway_active_connections
  ```

#### 2. Image Processing Pipeline

- **Queue Depth** shows backpressure in the image upload pipeline
  ```promql
  voyager_image_queue_depth
  ```

- **Processing Latency** time from upload to thumbnail generation
  ```promql
  histogram_quantile(0.99, rate(voyager_image_processing_duration_seconds_bucket[5m]))
  ```

- **Worker Throughput** images processed per second
  ```promql
  rate(voyager_worker_images_processed_total[5m])
  ```

#### 3. Infrastructure Health

- **PostgreSQL Connections**
  ```promql
  voyager_postgres_connections_active
  ```

- **MinIO Storage Used**
  ```promql
  voyager_minio_storage_used_bytes
  ```

- **NSQ Message Backlog**
  ```promql
  nsq_depth
  ```

### Tips

- Use **Explore** view to correlate metrics with traces
- Set up alerts for error rate > 5% or p95 latency > 500ms
- Use dashboard variables (e.g., `$service`) to filter across services

---

## Prometheus

**URL:** http://localhost:9091

### Key Queries

| What | Query |
|------|-------|
| Gateway uptime | `voyager_gateway_uptime_seconds` |
| Total requests by endpoint | `voyager_gateway_requests_total` |
| Request rate (5m window) | `rate(voyager_gateway_requests_total[5m])` |
| NSQ topic depth | `nsq_depth{topic="image-uploaded"}` |
| Go goroutines | `go_goroutines{job="api-gateway"}` |
| Memory usage | `process_resident_memory_bytes{job="api-gateway"}` |

### Targets

Navigate to **Status → Targets** to verify all scrape jobs are healthy:

- `api-gateway` → `host.docker.internal:8080/metrics`
- `image-svc` → `host.docker.internal:50051/metrics`
- `facts-svc` → `host.docker.internal:50052/metrics`
- `worker-svc` → `host.docker.internal:50053/metrics`
- `nsq` → `nsqd:4151/stats` (prometheus format)

### Alerts to Watch For

- Scrape target down (any service missing)
- High cardinality labels (check `/api/v1/status/tsdb`)

---

## Tempo (Distributed Tracing)

**URL:** http://localhost:3200

### How to See Traces

1. Open Grafana → **Explore** → Select **Tempo** data source
2. Search by:
   - **Service Name:** `api-gateway`, `image-svc`, `facts-svc`, `worker-svc`
   - **Operation:** `GET /v1/feed`, `POST /v1/images`, etc.
   - **Duration:** Filter for slow requests (e.g., > 200ms)
   - **Status:** Filter for errors

### Trace Flow (Image Upload)

A typical image upload trace spans these services:

```
┌─────────────┐    ┌───────────┐    ┌──────┐    ┌─────┐
│ API Gateway │───▶│ Image Svc │───▶│ MinIO│    │ NSQ │
│  (142ms)    │    │  (86ms)   │    │(52ms)│    │(12ms│
└─────────────┘    └───────────┘    └──────┘    └─────┘
       │                  │              │           │
       ├──── validate ────┤              │           │
       │                  ├── upload ────┤           │
       │                  ├── publish ───────────────┤
       └──── respond ─────┘
```

### Key Span Attributes

| Attribute | Description |
|-----------|-------------|
| `http.method` | GET, POST, etc. |
| `http.url` | Full request URL |
| `http.status_code` | Response status |
| `image.id` | Image identifier (custom) |
| `nsq.topic` | NSQ topic published to |
| `db.statement` | SQL query (if any) |

### OTLP Configuration

Services export traces via OTLP gRPC to `tempo:4317`. Ensure environment variable is set:

```
OTEL_EXPORTER_OTLP_ENDPOINT=http://tempo:4317
```

---

## NSQ Admin

**URL:** http://localhost:4171

### What to Look For

#### Topics

- **`image-uploaded`** published when a new image is uploaded via the gateway
- **`image-processed`** published when worker finishes thumbnail generation
- **`fact-requested`** (optional) fact generation events

#### Key Metrics

| Metric | Healthy Range | Action if Exceeded |
|--------|--------------|-------------------|
| Depth | 0–10 | Normal backlog. If > 50, workers may be overwhelmed |
| In-Flight | 0–5 | Messages currently being processed |
| Requeue Count | 0 | Non-zero means processing failures |
| Timeout Count | 0 | Non-zero means workers are too slow |
| Message Count | Growing | Total messages ever published |

#### Channels

Each topic should have at least one channel (consumer):
- `image-uploaded` → channel `worker-svc` (consumed by worker)
- Check that channel depth matches topic depth

#### Health Checks

- Navigate to **Nodes** to verify `nsqd` is connected to `nsqlookupd`
- **Counter** tab shows message flow rate
- If depth grows continuously → worker is down or too slow

---

## MinIO (Object Storage)

**URL:** http://localhost:9001
**Login:** `minioadmin` / `minioadmin`

### How to Browse Uploaded Files

1. Log in to the MinIO Console
2. Navigate to **Object Browser** in the sidebar
3. Look for the `voyager-images` bucket

### Bucket Structure

```
voyager-images/
├── img-001/
│   ├── original.jpg
│   ├── thumbnail_256.jpg
│   └── thumbnail_512.jpg
├── img-002/
│   ├── original.png
│   └── ...
└── ...
```

### What to Verify

- **Bucket exists** If `voyager-images` doesn't exist, the image-svc should auto-create it on startup
- **Object count** Should match the number of uploaded images
- **Object sizes** Originals should be reasonable (< 10MB each)
- **Access policy** Bucket should be private (no public access)

### Useful Operations

- **Download** Click any object to download and verify content
- **Delete** Remove test uploads to free space
- **Metrics** Check **Monitoring → Metrics** for:
  - Total storage used
  - Number of objects
  - API request rate
  - Error rate

### S3 API Access

You can also browse via CLI:

```bash
# Install mc (MinIO client)
mc alias set voyager http://localhost:9000 minioadmin minioadmin

# List buckets
mc ls voyager

# List objects in bucket
mc ls voyager/voyager-images --recursive

# Get bucket stats
mc stat voyager/voyager-images
```

---

## Quick Health Check Script

Run this to verify all services are responding:

```bash
#!/bin/bash
echo "=== Voyager Health Check ==="

# API Gateway
curl -s http://localhost:8080/health | jq .

# Prometheus targets
curl -s http://localhost:9091/api/v1/targets | jq '.data.activeTargets[] | {job: .labels.job, health: .health}'

# NSQ stats
curl -s http://localhost:4151/stats | jq '.topics[] | {name: .topic_name, depth: .depth}'

# MinIO
curl -s http://localhost:9000/minio/health/live && echo "MinIO: OK"

# Postgres
pg_isready -h localhost -p 5432 -U voyager && echo "Postgres: OK"

echo "=== Done ==="
```

---

## Troubleshooting

| Symptom | Likely Cause | Fix |
|---------|-------------|-----|
| Gateway returns 502 | Backend service down | Check `docker compose ps`, restart failed service |
| Traces not appearing | OTLP endpoint misconfigured | Verify `OTEL_EXPORTER_OTLP_ENDPOINT` env var |
| NSQ depth growing | Worker crashed or slow | Check worker logs: `docker compose logs worker-svc` |
| Prometheus target DOWN | Service not exposing /metrics | Verify service is running and metrics endpoint works |
| MinIO connection refused | Container not ready | Wait for healthcheck or restart: `docker compose restart minio` |
| Grafana shows "No data" | Wrong time range or query | Check time picker (last 5m), verify data source connection |
