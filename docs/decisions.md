# Architecture Decision Records (ADRs)

Every technology choice has trade-offs. This document explains **what** we chose, **why** we chose it, **what alternatives** we considered, and **what we gave up**.

---

## ADR-001: Go over Python / Node.js / Java

### What
All Voyager services are written in **Go 1.22+**.

### Why
- **Performance**: Go compiles to a single binary that runs nearly as fast as C. Python is 10-100x slower for CPU-bound work like image processing.
- **Concurrency**: Goroutines let us handle thousands of concurrent requests with minimal memory. Each goroutine costs ~2KB vs 1MB per thread in Java.
- **Small binaries**: A Go service compiles to a 10-20MB binary. No runtime needed. Java needs a 200MB JVM.
- **Fast startup**: Go services start in milliseconds. Java services take 5-30 seconds (bad for Kubernetes pod scaling).
- **Simple deployment**: Copy one file. Done. No `pip install`, no `node_modules`, no classpath.

### Alternatives Considered

| Language | Pros | Why Not |
|----------|------|---------|
| Python | Easy to write, huge ecosystem | Too slow for high-throughput, GIL limits concurrency |
| Node.js | Great for I/O, npm ecosystem | Single-threaded, callback hell, no type safety |
| Java | Enterprise ecosystem, JVM performance | Slow startup, heavy memory, verbose boilerplate |
| Rust | Maximum performance, memory safety | Steep learning curve, slower development velocity |

### Trade-offs

| We Get | We Lose |
|--------|---------|
| Fast compilation (2-5 seconds) | Less expressive than Python (more boilerplate) |
| Single binary deployment | Smaller ecosystem than Node/Python |
| Built-in concurrency | No generics until Go 1.18 (now we have them) |
| Strong standard library | Manual error handling (`if err != nil`) |

### Code Example

```go
// Go: Handle 10,000 concurrent connections easily
func main() {
    http.HandleFunc("/upload", handleUpload)
    http.ListenAndServe(":8080", nil)  // Each request gets its own goroutine
}

// Compare with Python (needs async/await, event loop, ASGI):
// async def handle_upload(request):
//     ...
```

---

## ADR-002: gRPC over REST (Internal Communication)

### What
All service-to-service communication uses **gRPC with Protobuf**. Only the API Gateway exposes REST.

### Why
- **Type safety**: The `.proto` file is a contract. Both sides know exactly what to expect. No "field name typo" bugs.
- **Performance**: Binary serialization is 3-10x faster than JSON parsing.
- **Code generation**: `protoc` generates server/client code in Go automatically. No hand-writing HTTP clients.
- **Streaming**: gRPC supports bidirectional streaming natively (live feed of new images).
- **Deadlines**: Built-in timeout propagation. If the gateway sets a 5s deadline, it automatically propagates to downstream services.

### Alternatives Considered

| Option | Pros | Why Not |
|--------|------|---------|
| REST everywhere | Simple, universal | Slow for internal, no type safety, no streaming |
| GraphQL | Flexible queries | Complexity overkill for internal services |
| Apache Thrift | Similar to gRPC | Smaller community, less Go support |
| Message queue only | Fully decoupled | Not suitable for request-response patterns |

### Trade-offs

| We Get | We Lose |
|--------|---------|
| 10x faster than REST (internal) | Can't call from browser directly |
| Automatic code generation | Need to learn Protobuf |
| Built-in streaming | Extra build step (`make proto`) |
| Deadline propagation | Harder to debug (binary, not human-readable) |
| Strong backward compatibility | More initial setup |

### Code Example

```protobuf
// Define once in .proto file:
service ImageService {
  rpc Get(GetRequest) returns (Image);
  rpc List(ListRequest) returns (ListResponse);
  rpc StreamNew(StreamRequest) returns (stream Image);
}
```

```go
// Generated client code (automatic):
client := imagev1.NewImageServiceClient(conn)
image, err := client.Get(ctx, &imagev1.GetRequest{Id: "abc123"})

// Compare with REST client (manual):
// resp, err := http.Get("http://image-svc:50051/v1/images/abc123")
// body, err := io.ReadAll(resp.Body)
// var image Image
// err = json.Unmarshal(body, &image)  // hope the fields match!
```

---

## ADR-003: Protobuf over JSON (Internal Serialization)

### What
All internal data is serialized using **Protocol Buffers** (Protobuf), not JSON.

### Why
- **Binary format**: 3-10x smaller than JSON on the wire.
- **Schema evolution**: Add new fields without breaking old clients (field numbers ensure compatibility).
- **Validation**: Generated code won't accept wrong types. JSON lets anything through.
- **Documentation**: The `.proto` file IS the documentation of your API.

### Alternatives Considered

| Format | Pros | Why Not |
|--------|------|---------|
| JSON | Human readable, universal | No schema, slow parsing, verbose |
| MessagePack | Binary JSON, fast | No schema, no code generation |
| Avro | Schema evolution, compact | Complex, more common in Kafka ecosystem |
| FlatBuffers | Zero-copy access | Too complex for our needs |

### Trade-offs

| We Get | We Lose |
|--------|---------|
| 3-10x smaller messages | Can't read messages in terminal (binary) |
| Type-safe code generation | Need `buf` or `protoc` toolchain |
| Backward-compatible evolution | Learning curve for Protobuf syntax |
| Self-documenting schemas | Extra build step |

### Code Example

```protobuf
// Schema (image.proto):
message Image {
  string id = 1;
  string title = 2;
  // Field 3 was removed — numbers are never reused!
  repeated string tags = 4;
  ImageStatus status = 7;
}

// Adding a new field later (backward compatible):
message Image {
  string id = 1;
  string title = 2;
  repeated string tags = 4;
  ImageStatus status = 7;
  optional string ai_description = 11;  // New! Old clients just ignore it
}
```

---

## ADR-004: NSQ over Kafka / RabbitMQ

### What
Voyager uses **NSQ** for asynchronous messaging between services.

### Why
- **Operational simplicity**: No ZooKeeper, no cluster coordinator, no partition management.
- **Easy setup**: One binary (`nsqd`), one discovery service (`nsqlookupd`). That's it.
- **Distributed by default**: No single point of failure. Each nsqd is independent.
- **Good enough**: For our scale (thousands of messages/sec), NSQ is more than sufficient.
- **Go native**: Written in Go, excellent Go client library.

### Alternatives Considered

| System | Pros | Why Not |
|--------|------|---------|
| Kafka | Massive scale, replay, ordering | Needs ZooKeeper/KRaft, complex ops, overkill |
| RabbitMQ | Feature-rich, AMQP standard | Erlang ops complexity, clustering is fragile |
| Redis Streams | Already have Redis, fast | Not designed for reliable messaging |
| NATS | Extremely fast, simple | Fewer delivery guarantees than NSQ |

### Trade-offs

| We Get | We Lose |
|--------|---------|
| 5-minute setup | No message replay (once consumed, gone) |
| Near-zero operational cost | No strict ordering guarantees |
| Horizontal scaling by adding nsqd nodes | Smaller community than Kafka |
| At-least-once delivery | No exactly-once semantics |
| Simple mental model | No built-in dead letter queue (DIY) |

### Code Example

```go
// Publishing (producer) — that's it. 3 lines.
producer, _ := nsq.NewProducer("nsqd:4150", nsq.NewConfig())
body, _ := json.Marshal(event)
producer.Publish("image.uploaded", body)

// Consuming — similarly simple
consumer, _ := nsq.NewConsumer("image.uploaded", "worker", nsq.NewConfig())
consumer.AddHandler(nsq.HandlerFunc(processImage))
consumer.ConnectToNSQLookupd("nsqlookupd:4161")
```

---

## ADR-005: PostgreSQL over MongoDB

### What
Image metadata and facts are stored in **PostgreSQL 16**.

### Why
- **Relational integrity**: Images have tags, users, statuses — relational queries are natural.
- **JSONB for flexibility**: The `exif` field is JSONB. We get document-style flexibility where we need it, with relational structure everywhere else.
- **Mature and reliable**: 35+ years of development. Battle-tested at every scale.
- **Tooling**: pgAdmin, psql, great monitoring, every ORM supports it.
- **Advanced features**: Full-text search, array types, GIN indexes, CTEs, window functions.

### Alternatives Considered

| Database | Pros | Why Not |
|----------|------|---------|
| MongoDB | Flexible schema, easy start | No JOINs, eventual consistency surprises, vendor lock-in risk |
| MySQL | Popular, fast reads | Fewer features than PostgreSQL (no arrays, weaker JSONB) |
| CockroachDB | Distributed SQL | Overkill complexity for single-region deployment |
| DynamoDB | Serverless, scales infinitely | Vendor lock-in, complex access patterns, expensive at rest |

### Trade-offs

| We Get | We Lose |
|--------|---------|
| Strong consistency (ACID) | Need to manage schema migrations |
| Rich query language (SQL) | Harder to horizontally scale (vs MongoDB) |
| JSONB flexibility where needed | Schema changes require migrations |
| Excellent Go drivers (pgx, pgxpool) | Connection pool management needed |

### Code Example

```sql
-- The best of both worlds: relational + document
SELECT id, title, tags, exif->>'camera' as camera
FROM images
WHERE 'nature' = ANY(tags)
  AND status = 'ready'
  AND (exif->>'gps') IS NOT NULL
ORDER BY created_at DESC
LIMIT 10;

-- Try doing this in MongoDB with proper consistency... 😅
```

---

## ADR-006: MinIO over Filesystem

### What
All image files (raw, thumbnails, medium) are stored in **MinIO** (S3-compatible object storage).

### Why
- **S3 compatibility**: Same API as AWS S3. Zero code changes when moving to production.
- **Separation of concerns**: Files don't live on the same server as code. Scale independently.
- **Presigned URLs**: Let clients download directly from storage, bypassing our services.
- **Multi-server**: Works the same whether you have 1 server or 100.
- **Lifecycle policies**: Auto-delete files older than X days, transition to cold storage.

### Alternatives Considered

| Option | Pros | Why Not |
|--------|------|---------|
| Local filesystem | Zero setup | Doesn't scale, no redundancy, tied to one server |
| Google Cloud Storage | Managed, reliable | Vendor lock-in, no local dev equivalent |
| Ceph | Open source, distributed | Extremely complex to operate |
| SeaweedFS | Fast, simple | Smaller community, less S3 compatible |

### Trade-offs

| We Get | We Lose |
|--------|---------|
| S3 compatibility (portable) | Need to run MinIO locally |
| Presigned URLs (offload serving) | Network hop for every file operation |
| Infinite scale in production | Extra infrastructure component |
| Zero code changes: local → prod | Slightly more complex local setup |

### Code Example

```go
// This code works with both MinIO locally and AWS S3 in production!
client, _ := minio.New(endpoint, &minio.Options{
    Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
    Secure: false,  // true in production
})

// Upload
client.PutObject(ctx, "voyager-images", "raw/img-7f3a.jpg", reader, size, opts)

// Generate download link (expires in 15 min)
url, _ := client.PresignedGetObject(ctx, "voyager-images", "raw/img-7f3a.jpg", 15*time.Minute, nil)
```

---

## ADR-007: Kubernetes over Docker Compose (Production)

### What
Production runs on **Kubernetes** (k3s locally, EKS/GKE in cloud). Docker Compose is only for local development.

### Why
- **Self-healing**: Pod crashes? Kubernetes restarts it automatically.
- **Auto-scaling**: HPA scales workers based on queue depth.
- **Rolling updates**: Deploy new versions with zero downtime.
- **Service discovery**: Services find each other by name (no hardcoded IPs).
- **Resource limits**: Prevent one service from eating all the memory.

### Alternatives Considered

| Option | Pros | Why Not |
|--------|------|---------|
| Docker Compose only | Simple, works locally | No auto-scaling, no self-healing, no rolling updates |
| Docker Swarm | Simpler than K8s | Effectively abandoned by Docker Inc. |
| Nomad (HashiCorp) | Simpler, multi-runtime | Smaller ecosystem, fewer managed options |
| AWS ECS | Managed, simple | AWS lock-in, less portable |

### Trade-offs

| We Get | We Lose |
|--------|---------|
| Industry standard (hire anyone) | Steep learning curve |
| Auto-scaling, self-healing | Operational complexity |
| Rolling deployments | YAML sprawl |
| Rich ecosystem (Helm, operators) | Resource overhead (control plane) |

### Code Example

```yaml
# deploy/base/image-svc.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: image-svc
spec:
  replicas: 2
  selector:
    matchLabels:
      app: image-svc
  template:
    spec:
      containers:
      - name: image-svc
        image: voyager/image-svc:latest
        ports:
        - containerPort: 50051
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          grpc:
            port: 50051
          periodSeconds: 10
```

---

## ADR-008: Kustomize over Helm

### What
Kubernetes manifests are managed with **Kustomize** (built into `kubectl`), not Helm.

### Why
- **No templating language**: Kustomize uses overlays (patches), not Go templates. Easier to read.
- **Built into kubectl**: No extra tool to install (`kubectl apply -k .`).
- **Plain YAML**: Your base manifests are valid Kubernetes YAML. No `{{ .Values.x }}` magic.
- **Overlays**: Environment-specific changes are patches, not variable substitution.

### Alternatives Considered

| Tool | Pros | Why Not |
|------|------|---------|
| Helm | Huge chart ecosystem, templating | Complex template language, hard to debug |
| Raw YAML | No tools needed | Copy-paste between environments |
| Jsonnet | Powerful language | Too complex, another language to learn |
| CDK8s | TypeScript/Python for manifests | Extra compilation step, overkill |

### Trade-offs

| We Get | We Lose |
|--------|---------|
| Plain, readable YAML | No parameterized charts |
| No extra tools | Can't publish reusable "charts" |
| Easy to understand diffs | Less powerful than Helm templates |
| Built into kubectl | Smaller community than Helm |

### Code Example

```yaml
# deploy/base/kustomization.yaml (what's common across all environments)
resources:
  - api-gateway.yaml
  - image-svc.yaml
  - facts-svc.yaml
  - worker-svc.yaml
  - nsq.yaml
  - postgres.yaml
  - minio.yaml

# deploy/overlays/production/kustomization.yaml (production-specific)
resources:
  - ../../base
patchesStrategicMerge:
  - increase-replicas.yaml
  - production-resources.yaml
```

---

## ADR-009: OpenTelemetry over Vendor-Specific SDKs

### What
All instrumentation uses **OpenTelemetry** (OTel) as the standard. No vendor-specific SDKs.

### Why
- **Vendor neutral**: Switch from Tempo to Jaeger to Datadog without changing application code.
- **Industry standard**: CNCF graduated project. Every major vendor supports it.
- **Unified**: One SDK for traces, metrics, and logs (instead of three different libraries).
- **Auto-instrumentation**: gRPC, HTTP, database drivers are instrumented automatically.

### Alternatives Considered

| Option | Pros | Why Not |
|--------|------|---------|
| Jaeger client | Mature, battle-tested | Vendor-specific, deprecated in favor of OTel |
| Datadog APM | Great UI, easy setup | Expensive, vendor lock-in |
| AWS X-Ray SDK | Integrated with AWS | AWS lock-in, limited outside AWS |
| Zipkin client | Simple, lightweight | Less active development, fewer features |

### Trade-offs

| We Get | We Lose |
|--------|---------|
| Swap backends freely | Slightly more complex initial setup |
| One SDK for everything | OTel is still evolving (some APIs in beta) |
| CNCF backing (won't die) | More verbose configuration |
| Auto-instrumentation for gRPC/HTTP | Learning curve for OTel concepts |

### Code Example

```go
// Initialize once, works with any backend:
exporter, _ := otlptracegrpc.New(ctx,
    otlptracegrpc.WithEndpoint("tempo:4317"),  // Change this to switch backends
)
tp := trace.NewTracerProvider(trace.WithBatcher(exporter))
otel.SetTracerProvider(tp)

// Instrument gRPC (automatic spans for every RPC):
srv := grpc.NewServer(
    grpc.StatsHandler(otelgrpc.NewServerHandler()),
)
```

---

## ADR-010: Structured Logging (zap) over fmt.Println

### What
All logging uses **uber-go/zap** with JSON output. No `fmt.Println` or `log.Printf`.

### Why
- **Machine parseable**: JSON logs can be indexed, searched, filtered, and alerted on.
- **Performance**: zap is the fastest Go logger (zero-allocation in hot paths).
- **Context**: Every log line includes service name, trace ID, request ID, and structured fields.
- **Levels**: Debug, Info, Warn, Error, Fatal — filter noise in production.

### Alternatives Considered

| Logger | Pros | Why Not |
|--------|------|---------|
| `fmt.Println` | Zero setup | No structure, no levels, no context, can't search |
| `log` (stdlib) | Always available | No structured fields, no levels |
| `logrus` | Popular, structured | 5-10x slower than zap |
| `zerolog` | Fast, structured | Less popular, smaller community |
| `slog` (Go 1.21+) | Standard library! | Newer, fewer integrations (catching up) |

### Trade-offs

| We Get | We Lose |
|--------|---------|
| Searchable logs (grep by field) | Can't read logs casually in terminal (JSON) |
| Performance (zero-alloc) | More verbose log statements |
| Automatic trace correlation | Extra dependency |
| Alert on specific patterns | Initial setup time |

### Code Example

```go
// Bad: fmt.Println
fmt.Println("uploaded image", imageID, "by user", userID)
// Output: uploaded image abc123 by user user-789
// Can you search this? Filter it? Alert on it? Nope.

// Good: zap structured logging
logger.Info("Image uploaded",
    zap.String("image_id", imageID),
    zap.String("user_id", userID),
    zap.Int64("size_bytes", size),
    zap.Duration("duration", elapsed),
    zap.String("trace_id", traceID),
)
// Output: {"level":"info","ts":"2026-06-19T10:30:00Z","msg":"Image uploaded",
//          "image_id":"abc123","user_id":"user-789","size_bytes":5242880,
//          "duration":"234ms","trace_id":"abc123def456"}
// Now you can: search, filter, alert, correlate with traces!
```

---

## Decision Summary

| # | Decision | Key Reason |
|---|----------|-----------|
| 1 | Go | Performance + simplicity + small containers |
| 2 | gRPC (internal) | Type safety + speed + streaming |
| 3 | Protobuf | Schema evolution + compact + generated code |
| 4 | NSQ | Operational simplicity, good enough for our scale |
| 5 | PostgreSQL | Relational integrity + JSONB flexibility |
| 6 | MinIO | S3-compatible, local/prod parity |
| 7 | Kubernetes | Self-healing + auto-scaling + industry standard |
| 8 | Kustomize | Plain YAML, no template language |
| 9 | OpenTelemetry | Vendor neutral, unified standard |
| 10 | zap logging | Fast, structured, machine-parseable |
