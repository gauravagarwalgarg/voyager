# C4 Architecture Diagrams

This document provides a complete C4 model of the Voyager platform using Mermaid diagrams. The C4 model describes software architecture at four zoom levels, from the 10,000-foot view down to individual code structures.

---

## Level 1: System Context

The highest level. Who uses Voyager, and what external systems does it talk to?

```mermaid
C4Context
    title Voyager System Context Diagram

    Person(user, "End User", "Uploads images, browses feed, reads random facts")
    Person(admin, "Platform Admin", "Monitors health, manages infrastructure")

    System(voyager, "Voyager Platform", "Cloud-native image sharing platform with random facts")

    System_Ext(cdn, "CDN / Browser", "Delivers cached images to users globally")
    System_Ext(s3prod, "AWS S3", "Production object storage (replaceable with MinIO locally)")
    System_Ext(oauth, "OAuth Provider", "External identity provider for JWT tokens")

    Rel(user, voyager, "Uploads images, browses feed", "HTTPS/REST")
    Rel(admin, voyager, "Views dashboards, queries metrics", "HTTPS")
    Rel(voyager, s3prod, "Stores/retrieves image files", "S3 API")
    Rel(voyager, cdn, "Serves processed images", "HTTPS")
    Rel(user, oauth, "Authenticates", "OIDC/OAuth2")
    Rel(oauth, voyager, "Provides JWT tokens", "JWT")
```

**What this shows**: Voyager is a single system from the outside. Users interact over HTTP. Admins use Grafana dashboards. Image files live in S3-compatible storage.

---

## Level 2: Container Diagram

Zoom in. What containers (services, databases, queues) make up Voyager?

```mermaid
C4Container
    title Voyager Container Diagram

    Person(user, "End User")

    Container_Boundary(k8s, "Kubernetes Cluster") {
        Container(gateway, "API Gateway", "Go, net/http", "HTTP/REST entry point. Translates REST→gRPC, handles auth & rate limiting. Port 8080")
        Container(imgsvc, "Image Service", "Go, gRPC", "Image upload, metadata CRUD, presigned URLs. Port 50051")
        Container(factssvc, "Facts Service", "Go, gRPC", "Random fact generation and serving. Port 50052")
        Container(worker, "Worker Service", "Go, NSQ consumer", "Async image processing: resize, thumbnail, EXIF extraction")

        ContainerDb(postgres, "PostgreSQL", "PostgreSQL 16", "Image metadata, facts, user data")
        ContainerDb(minio, "MinIO", "S3-compatible", "Raw images, thumbnails, medium-size variants")
        Container(nsq, "NSQ", "Message Queue", "Async messaging between services. Topics: image.uploaded, image.processed")

        Container(prometheus, "Prometheus", "Metrics", "Scrapes /metrics from all services")
        Container(grafana, "Grafana", "Dashboards", "Visualizes metrics and traces")
        Container(tempo, "Tempo", "Tracing", "Stores distributed traces via OTLP")
    }

    Rel(user, gateway, "HTTP/REST requests", "JSON over HTTPS")
    Rel(gateway, imgsvc, "gRPC calls", "Protobuf")
    Rel(gateway, factssvc, "gRPC calls", "Protobuf")
    Rel(imgsvc, postgres, "Reads/writes metadata", "SQL/TLS")
    Rel(imgsvc, minio, "Stores raw images", "S3 API")
    Rel(imgsvc, nsq, "Publishes image.uploaded", "TCP")
    Rel(nsq, worker, "Delivers messages", "TCP")
    Rel(worker, minio, "Reads raw, writes processed", "S3 API")
    Rel(worker, postgres, "Updates image status", "SQL/TLS")
    Rel(factssvc, postgres, "Reads facts", "SQL/TLS")
    Rel(gateway, prometheus, "Exposes /metrics", "HTTP pull")
    Rel(imgsvc, prometheus, "Exposes /metrics", "HTTP pull")
    Rel(prometheus, grafana, "Data source", "PromQL")
    Rel(tempo, grafana, "Data source", "TraceQL")
```

**Key insight**: The API Gateway is the only service exposed externally. All internal communication uses gRPC (synchronous) or NSQ (asynchronous). Each service has a single responsibility.

---

## Level 3: Component Diagram API Gateway

What's inside the API Gateway?

```mermaid
C4Component
    title API Gateway - Component Diagram

    Container_Boundary(gateway, "API Gateway") {
        Component(router, "HTTP Router", "net/http ServeMux", "Routes requests to appropriate handlers")
        Component(authMw, "Auth Middleware", "middleware/auth.go", "Validates JWT tokens, extracts user claims")
        Component(rateMw, "Rate Limiter", "middleware/ratelimit.go", "Token bucket algorithm, per-IP and per-user limits")
        Component(traceMw, "Tracing Middleware", "middleware/tracing.go", "Creates root span, injects trace context")
        Component(logMw, "Logging Middleware", "middleware/logging.go", "Structured request/response logging with zap")
        Component(imgHandler, "Image Handler", "gateway/image_handler.go", "Translates REST image requests to gRPC calls")
        Component(factsHandler, "Facts Handler", "gateway/facts_handler.go", "Translates REST fact requests to gRPC calls")
        Component(healthHandler, "Health Handler", "Built-in", "Returns service health status")
        Component(metricsHandler, "Metrics Handler", "promhttp", "Exposes Prometheus metrics endpoint")
        Component(grpcPool, "gRPC Connection Pool", "pkg/grpc_pool.go", "Manages persistent connections to backend services")
    }

    Container(imgsvc, "Image Service", "gRPC :50051")
    Container(factssvc, "Facts Service", "gRPC :50052")

    Rel(router, authMw, "Passes request through")
    Rel(authMw, rateMw, "If authenticated")
    Rel(rateMw, traceMw, "If within limits")
    Rel(traceMw, logMw, "Adds trace context")
    Rel(logMw, imgHandler, "/v1/images/*")
    Rel(logMw, factsHandler, "/v1/facts/*")
    Rel(logMw, healthHandler, "/health")
    Rel(logMw, metricsHandler, "/metrics")
    Rel(imgHandler, grpcPool, "Gets connection")
    Rel(factsHandler, grpcPool, "Gets connection")
    Rel(grpcPool, imgsvc, "gRPC unary/stream")
    Rel(grpcPool, factssvc, "gRPC unary")
```

---

## Level 3: Component Diagram Image Service

What's inside the Image Service?

```mermaid
C4Component
    title Image Service - Component Diagram

    Container_Boundary(imgsvc, "Image Service") {
        Component(grpcServer, "gRPC Server", "google.golang.org/grpc", "Listens on :50051, handles incoming RPCs")
        Component(interceptors, "Interceptors", "otelgrpc, recovery", "Tracing, panic recovery, logging")
        Component(imgImpl, "ImageServiceServer", "image/server.go", "Implements ImageService proto interface")
        Component(repo, "Image Repository", "image/repository.go", "PostgreSQL queries for image metadata")
        Component(storageClient, "Storage Client", "pkg/storage/minio.go", "Uploads/downloads files to MinIO/S3")
        Component(publisher, "NSQ Publisher", "pkg/messaging/producer.go", "Publishes image.uploaded events")
        Component(validator, "Request Validator", "image/validate.go", "Validates upload size, format, required fields")
        Component(healthSrv, "Health Server", "grpc/health", "gRPC health check protocol")
    }

    ContainerDb(postgres, "PostgreSQL")
    ContainerDb(minio, "MinIO")
    Container(nsq, "NSQ")

    Rel(grpcServer, interceptors, "Wraps all RPCs")
    Rel(interceptors, imgImpl, "Calls implementation")
    Rel(imgImpl, validator, "Validates input")
    Rel(imgImpl, repo, "CRUD operations")
    Rel(imgImpl, storageClient, "Upload/presign URLs")
    Rel(imgImpl, publisher, "Publish events")
    Rel(repo, postgres, "SQL queries")
    Rel(storageClient, minio, "S3 API calls")
    Rel(publisher, nsq, "TCP publish")
    Rel(grpcServer, healthSrv, "Health checks")
```

---

## Level 4: Code Diagram Image Service Key Structures

The lowest level. What are the key interfaces, structs, and their relationships?

```mermaid
classDiagram
    class ImageServiceServer {
        <<interface>>
        +Upload(ctx, UploadRequest) UploadResponse
        +Get(ctx, GetRequest) Image
        +List(ctx, ListRequest) ListResponse
        +Delete(ctx, DeleteRequest) DeleteResponse
        +StreamNew(StreamRequest, stream) error
    }

    class imageServer {
        -repo ImageRepository
        -storage StorageClient
        -publisher MessagePublisher
        -logger *zap.Logger
        +Upload(ctx, UploadRequest) UploadResponse
        +Get(ctx, GetRequest) Image
        +List(ctx, ListRequest) ListResponse
        +Delete(ctx, DeleteRequest) DeleteResponse
        +StreamNew(StreamRequest, stream) error
    }

    class ImageRepository {
        <<interface>>
        +Create(ctx, Image) (string, error)
        +GetByID(ctx, string) (Image, error)
        +List(ctx, ListFilter) ([]Image, error)
        +UpdateStatus(ctx, string, Status) error
        +Delete(ctx, string) error
    }

    class postgresRepo {
        -db *pgxpool.Pool
        +Create(ctx, Image) (string, error)
        +GetByID(ctx, string) (Image, error)
        +List(ctx, ListFilter) ([]Image, error)
        +UpdateStatus(ctx, string, Status) error
        +Delete(ctx, string) error
    }

    class StorageClient {
        <<interface>>
        +Upload(ctx, bucket, key, reader) error
        +GetPresignedURL(ctx, bucket, key) (string, error)
        +Delete(ctx, bucket, key) error
    }

    class minioClient {
        -client *minio.Client
        +Upload(ctx, bucket, key, reader) error
        +GetPresignedURL(ctx, bucket, key) (string, error)
        +Delete(ctx, bucket, key) error
    }

    class MessagePublisher {
        <<interface>>
        +Publish(topic string, body []byte) error
        +Stop()
    }

    class nsqProducer {
        -producer *nsq.Producer
        +Publish(topic string, body []byte) error
        +Stop()
    }

    class Image {
        +ID string
        +Title string
        +Description string
        +Tags []string
        +Status ImageStatus
        +URL string
        +ThumbnailURL string
        +Metadata ImageMetadata
        +UploaderID string
        +CreatedAt time.Time
    }

    class ImageMetadata {
        +Width int
        +Height int
        +Format string
        +SizeBytes int64
        +EXIF *ExifData
    }

    ImageServiceServer <|.. imageServer : implements
    ImageRepository <|.. postgresRepo : implements
    StorageClient <|.. minioClient : implements
    MessagePublisher <|.. nsqProducer : implements
    imageServer --> ImageRepository : uses
    imageServer --> StorageClient : uses
    imageServer --> MessagePublisher : uses
    Image --> ImageMetadata : contains
```

---

## Deployment Diagram

How does Voyager map to infrastructure?

```mermaid
graph TB
    subgraph "Kubernetes Cluster (k3s / EKS)"
        subgraph "Namespace: voyager"
            gw[API Gateway<br/>Deployment: 2 replicas]
            img[Image Service<br/>Deployment: 2 replicas]
            facts[Facts Service<br/>Deployment: 2 replicas]
            wrk[Worker Service<br/>Deployment: 1-20 replicas<br/>HPA on queue depth]
        end

        subgraph "Namespace: voyager-infra"
            pg[(PostgreSQL<br/>StatefulSet: 1 replica)]
            minio[(MinIO<br/>StatefulSet: 1 replica)]
            nsqd[NSQd<br/>Deployment: 1 replica]
            nsqlookup[NSQ Lookupd<br/>Deployment: 1 replica]
        end

        subgraph "Namespace: observability"
            prom[Prometheus<br/>StatefulSet]
            graf[Grafana<br/>Deployment]
            tempo2[Tempo<br/>StatefulSet]
        end
    end

    internet((Internet)) --> gw
    gw --> img
    gw --> facts
    img --> pg
    img --> minio
    img --> nsqd
    nsqd --> wrk
    wrk --> pg
    wrk --> minio
    facts --> pg
    prom -.->|scrapes| gw
    prom -.->|scrapes| img
    prom -.->|scrapes| facts
    prom -.->|scrapes| wrk
```

---

## How to Read These Diagrams

| Level | Question It Answers | Audience |
|-------|-------------------|----------|
| 1 - Context | What does Voyager do? Who uses it? | Everyone |
| 2 - Container | What services/databases exist? How do they connect? | Developers, Architects |
| 3 - Component | What's inside each service? What packages? | Developers working on that service |
| 4 - Code | What are the key interfaces and structs? | Developers implementing features |

> **Tip**: Start from Level 1 and zoom in only when you need more detail. Most architecture discussions happen at Level 2.
