# Architecture

## System Design

Voyager is designed as a set of loosely coupled microservices communicating via gRPC (synchronous) and NSQ (asynchronous). Each service owns its domain and can be scaled independently.

### Service Boundaries

```
┌─────────────────────────────────────────────────────────────┐
│                      API Gateway                             │
│  • HTTP/REST → gRPC translation                            │
│  • JWT authentication                                       │
│  • Rate limiting (token bucket)                            │
│  • Request validation                                       │
│  • OpenTelemetry trace initiation                          │
└───────────┬──────────────────────────┬──────────────────────┘
            │ gRPC                     │ gRPC
            ▼                          ▼
┌───────────────────┐      ┌───────────────────┐
│   Image Service   │      │   Facts Service   │
│                   │      │                   │
│  • Upload/CRUD    │      │  • Random facts   │
│  • Metadata store │      │  • Categories     │
│  • Presigned URLs │      │  • Rate per user  │
│  • Tag search     │      │  • Fact cache     │
└─────────┬─────────┘      └───────────────────┘
          │ NSQ publish
          ▼
┌───────────────────┐      ┌───────────────────┐
│   Worker Service  │      │   Infrastructure  │
│                   │      │                   │
│  • Resize images  │      │  • PostgreSQL     │
│  • Generate thumb │      │  • MinIO (S3)     │
│  • Extract EXIF   │      │  • NSQ            │
│  • Update metadata│      │  • Redis (cache)  │
└───────────────────┘      └───────────────────┘
```

### Communication Patterns

| Pattern | Technology | Use Case |
|---------|-----------|----------|
| Sync request-response | gRPC (unary) | Image CRUD, fact retrieval |
| Async fire-and-forget | NSQ publish | Image uploaded → process later |
| Streaming | gRPC server stream | Live feed of new images |
| Pub/Sub | NSQ topics/channels | Multiple workers processing same event |

### Data Flow: Image Upload

```
1. Client → POST /v1/images (multipart)
2. API Gateway validates auth + rate limit
3. Gateway calls ImageService.Upload(gRPC)
4. Image Service:
   a. Generates UUID for image
   b. Uploads raw file to MinIO (presigned PUT)
   c. Inserts metadata row in PostgreSQL
   d. Publishes "image.uploaded" to NSQ
   e. Returns ImageID to gateway
5. Worker Service (async, via NSQ):
   a. Consumes "image.uploaded" message
   b. Downloads raw image from MinIO
   c. Generates thumbnail (300px)
   d. Generates medium (1200px)
   e. Extracts EXIF data (GPS, camera, date)
   f. Uploads variants to MinIO
   g. Updates metadata in PostgreSQL (status: processed)
6. API Gateway returns 201 + ImageID to client
```

### Data Flow: Feed Request

```
1. Client → GET /v1/feed?limit=10
2. API Gateway → gRPC ImageService.ListImages (with pagination)
3. Image Service queries PostgreSQL (JOIN with facts)
4. Returns list of images + associated random facts
5. Gateway serializes to JSON, responds to client
```

## Protobuf Schema Design

### Image Service

```protobuf
syntax = "proto3";
package voyager.image.v1;

service ImageService {
  rpc Upload(UploadRequest) returns (UploadResponse);
  rpc Get(GetRequest) returns (Image);
  rpc List(ListRequest) returns (ListResponse);
  rpc Delete(DeleteRequest) returns (DeleteResponse);
  rpc StreamNew(StreamRequest) returns (stream Image);
}

message Image {
  string id = 1;
  string title = 2;
  string description = 3;
  repeated string tags = 4;
  string url = 5;
  string thumbnail_url = 6;
  ImageStatus status = 7;
  ImageMetadata metadata = 8;
  google.protobuf.Timestamp created_at = 9;
  string uploader_id = 10;
}

enum ImageStatus {
  IMAGE_STATUS_UNSPECIFIED = 0;
  IMAGE_STATUS_PENDING = 1;
  IMAGE_STATUS_PROCESSING = 2;
  IMAGE_STATUS_READY = 3;
  IMAGE_STATUS_FAILED = 4;
}

message ImageMetadata {
  int32 width = 1;
  int32 height = 2;
  string format = 3;
  int64 size_bytes = 4;
  optional ExifData exif = 5;
}
```

### Facts Service

```protobuf
syntax = "proto3";
package voyager.facts.v1;

service FactsService {
  rpc GetRandom(GetRandomRequest) returns (Fact);
  rpc GetByCategory(GetByCategoryRequest) returns (FactList);
  rpc Create(CreateFactRequest) returns (Fact);
}

message Fact {
  string id = 1;
  string text = 2;
  string category = 3;
  string source = 4;
  google.protobuf.Timestamp created_at = 5;
}
```

## NSQ Message Schema

### Topic: `image.uploaded`

```json
{
  "image_id": "uuid-v4",
  "bucket": "voyager-images",
  "key": "raw/uuid-v4.jpg",
  "uploaded_at": "2026-06-19T10:30:00Z",
  "uploader_id": "user-123"
}
```

### Topic: `image.processed`

```json
{
  "image_id": "uuid-v4",
  "thumbnail_key": "thumb/uuid-v4.jpg",
  "medium_key": "medium/uuid-v4.jpg",
  "width": 4032,
  "height": 3024,
  "exif": { "camera": "iPhone 15 Pro", "gps": "28.6139,77.2090" },
  "processed_at": "2026-06-19T10:30:05Z"
}
```

## Database Schema (PostgreSQL)

```sql
CREATE TABLE images (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title VARCHAR(255) NOT NULL,
    description TEXT,
    tags TEXT[] DEFAULT '{}',
    status VARCHAR(20) DEFAULT 'pending',
    url TEXT,
    thumbnail_url TEXT,
    width INT,
    height INT,
    format VARCHAR(10),
    size_bytes BIGINT,
    exif JSONB DEFAULT '{}',
    uploader_id VARCHAR(100),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_images_status ON images(status);
CREATE INDEX idx_images_tags ON images USING GIN(tags);
CREATE INDEX idx_images_created ON images(created_at DESC);

CREATE TABLE facts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    text TEXT NOT NULL,
    category VARCHAR(50) NOT NULL,
    source VARCHAR(255),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_facts_category ON facts(category);
```

## Observability Design

### OpenTelemetry Integration

Every service initializes an OTEL provider that exports:
- **Traces** → Grafana Tempo (OTLP gRPC)
- **Metrics** → Prometheus (scrape endpoint)
- **Logs** → stdout (structured JSON, collected by k8s)

### Trace Propagation

```
Client request (trace_id: abc123)
  → API Gateway (span: gateway.handle_request)
    → Image Service (span: image.upload, parent: gateway)
      → PostgreSQL (span: db.insert, parent: image)
      → MinIO (span: s3.put_object, parent: image)
      → NSQ (span: nsq.publish, parent: image)
  → Worker (span: worker.process_image, linked to: image.upload)
    → MinIO (span: s3.get_object, parent: worker)
    → MinIO (span: s3.put_object, parent: worker) x2
    → PostgreSQL (span: db.update, parent: worker)
```

### Key Metrics

| Metric | Type | Labels |
|--------|------|--------|
| `voyager_http_requests_total` | Counter | method, path, status |
| `voyager_http_request_duration_seconds` | Histogram | method, path |
| `voyager_grpc_requests_total` | Counter | service, method, code |
| `voyager_grpc_request_duration_seconds` | Histogram | service, method |
| `voyager_nsq_messages_published_total` | Counter | topic |
| `voyager_nsq_messages_consumed_total` | Counter | topic, channel |
| `voyager_nsq_processing_duration_seconds` | Histogram | topic |
| `voyager_image_upload_size_bytes` | Histogram | format |
| `voyager_db_connections_active` | Gauge | service |
| `voyager_db_query_duration_seconds` | Histogram | query |

### Grafana Dashboards

1. **Service Overview (RED)**: Rate/Errors/Duration for each service
2. **gRPC Performance**: Per-method latency percentiles, error rates
3. **NSQ Health**: Queue depth, in-flight, timeout rate, requeue rate
4. **Infrastructure**: Pod CPU/memory, restart count, network I/O
5. **Business Metrics**: Uploads/hour, processing success rate, storage usage

## Scaling Strategy

### Horizontal Scaling

| Service | Scale Trigger | Target |
|---------|--------------|--------|
| API Gateway | CPU > 70% OR request rate > 1000 rps | 2-10 replicas |
| Image Service | CPU > 60% | 2-5 replicas |
| Facts Service | Response time > 50ms p99 | 2-3 replicas |
| Worker Service | NSQ queue depth > 100 | 1-20 replicas |

### Local Development (k3s)

For local dev, everything runs with 1 replica. k3s gives you a real Kubernetes cluster on your PC:

```bash
# Install k3s (lightweight k8s)
curl -sfL https://get.k3s.io | sh -

# Deploy voyager
kubectl apply -k deploy/overlays/local/
```

## Security

| Layer | Mechanism |
|-------|-----------|
| External → Gateway | JWT (RS256) + HTTPS |
| Gateway → Services | mTLS (future) or cluster-internal only |
| Service → Database | TLS connection + scoped credentials |
| Service → MinIO | IAM access key (per-service) |
| NSQ | Cluster-internal only (no auth needed locally) |
