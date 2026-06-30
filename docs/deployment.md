# Production Deployment

Guide to deploying Voyager to production environments.

---

## Deployment Options

| Method | Use Case | Complexity |
|--------|----------|------------|
| Docker Compose | Single-server, staging | Low |
| Kubernetes (k3s) | Local production-like | Medium |
| Kubernetes (EKS/GKE) | Full production | High |

---

## Option 1: Docker Compose (Single Server)

Best for staging or small-scale production on a single VM.

### Build Images

```bash
# Build all service images
make docker-build

# This creates:
#   voyager-api-gateway
#   voyager-image-svc
#   voyager-facts-svc
#   voyager-worker-svc
```

### Deploy

```bash
# Start everything (infra + services)
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d
```

### Production docker-compose override

Create `docker-compose.prod.yml`:

```yaml
services:
  api-gateway:
    image: voyager-api-gateway
    ports:
      - "8080:8080"
      - "9090:9090"
    environment:
      - VOYAGER_ENV=production
      - VOYAGER_DB_HOST=postgres
      - VOYAGER_DB_PASSWORD=${DB_PASSWORD}
      - VOYAGER_NSQ_ADDR=nsqd:4150
      - VOYAGER_MINIO_ENDPOINT=minio:9000
    depends_on:
      postgres:
        condition: service_healthy
      nsqd:
        condition: service_started
    restart: unless-stopped
    deploy:
      resources:
        limits:
          memory: 256M
          cpus: '0.5'

  image-svc:
    image: voyager-image-svc
    ports:
      - "50051:50051"
    environment:
      - VOYAGER_ENV=production
      - VOYAGER_DB_HOST=postgres
      - VOYAGER_DB_PASSWORD=${DB_PASSWORD}
      - VOYAGER_MINIO_ENDPOINT=minio:9000
      - VOYAGER_MINIO_ACCESS_KEY=${MINIO_ACCESS_KEY}
      - VOYAGER_MINIO_SECRET_KEY=${MINIO_SECRET_KEY}
    depends_on:
      postgres:
        condition: service_healthy
      minio:
        condition: service_healthy
    restart: unless-stopped

  facts-svc:
    image: voyager-facts-svc
    ports:
      - "50052:50052"
    environment:
      - VOYAGER_ENV=production
      - VOYAGER_DB_HOST=postgres
      - VOYAGER_DB_PASSWORD=${DB_PASSWORD}
    depends_on:
      postgres:
        condition: service_healthy
    restart: unless-stopped

  worker-svc:
    image: voyager-worker-svc
    environment:
      - VOYAGER_ENV=production
      - VOYAGER_NSQ_ADDR=nsqd:4150
      - VOYAGER_MINIO_ENDPOINT=minio:9000
    depends_on:
      - nsqd
      - minio
    restart: unless-stopped
    deploy:
      replicas: 2
```

### Environment Variables

Create `.env` for production secrets:

```bash
DB_PASSWORD=strong-random-password-here
MINIO_ACCESS_KEY=production-access-key
MINIO_SECRET_KEY=production-secret-key
```

---

## Option 2: Kubernetes (k3s Local Production)

Best for production-like testing on your own machine or a single node.

### Install k3s

```bash
curl -sfL https://get.k3s.io | sh -

# Verify
kubectl get nodes
```

### Deploy Infrastructure

```bash
# Deploy observability (Prometheus, Grafana, Tempo)
make deploy-obs

# Deploy all services
make deploy-local
```

### Verify

```bash
kubectl get pods -n voyager
kubectl get svc -n voyager
```

### Access Services

```bash
# Port-forward the gateway
kubectl port-forward svc/api-gateway 8080:8080 -n voyager

# Test
curl http://localhost:8080/health
```

---

## Option 3: Cloud Kubernetes (EKS / GKE / AKS)

### Prerequisites

- Kubernetes cluster provisioned
- `kubectl` configured
- Container registry (ECR, GCR, or Docker Hub)
- External PostgreSQL (RDS, Cloud SQL)
- External object storage (S3, GCS)

### Push Images to Registry

```bash
# Tag for your registry
REGISTRY=your-account.dkr.ecr.us-east-1.amazonaws.com

for svc in api-gateway image-svc facts-svc worker-svc; do
  docker tag voyager-$svc $REGISTRY/voyager-$svc:latest
  docker push $REGISTRY/voyager-$svc:latest
done
```

### Deploy with Kustomize

```bash
# Production overlay
kubectl apply -k deploy/overlays/production/
```

### Production Overlay Structure

```
deploy/overlays/production/
├── kustomization.yaml
├── namespace.yaml
├── ingress.yaml           # External HTTPS access
├── hpa.yaml               # Horizontal Pod Autoscaler
├── secrets.yaml           # Sealed secrets
└── patches/
    ├── api-gateway.yaml   # Replica count, resources
    ├── image-svc.yaml
    └── worker-svc.yaml    # Multiple replicas for throughput
```

### Ingress (HTTPS)

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: voyager-ingress
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/rate-limit: "100"
spec:
  tls:
    - hosts:
        - api.voyager.example.com
      secretName: voyager-tls
  rules:
    - host: api.voyager.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: api-gateway
                port:
                  number: 8080
```

### Horizontal Pod Autoscaler

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: api-gateway-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: api-gateway
  minReplicas: 2
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
    - type: Resource
      resource:
        name: memory
        target:
          type: Utilization
          averageUtilization: 80
```

---

## CI/CD Pipeline

The GitHub Actions workflow builds, tests, and deploys automatically:

```yaml
# .github/workflows/ci.yml handles:
# 1. Lint (golangci-lint)
# 2. Test (unit + integration)
# 3. Build Docker images
# 4. Push to registry (on main)
# 5. Deploy to staging (on main)
# 6. Deploy to production (on tag)
```

### Manual Deploy

```bash
# Deploy to staging
make deploy-local

# Deploy to production (after tagging)
git tag v1.0.0
git push --tags
# CI picks up the tag and deploys to production
```

---

## Health Checks & Monitoring

### Liveness & Readiness Probes

Every service exposes:

| Endpoint | Purpose |
|----------|---------|
| `/health` | Liveness is the process alive? |
| `/ready` | Readiness can it serve traffic? (checks DB, NSQ connections) |

### Grafana Dashboards

After deployment, access Grafana at `:3000` (or via ingress):

- **Service Overview**: Request rate, error rate, latency (RED metrics)
- **Infrastructure**: CPU, memory, pod count
- **NSQ Queues**: Message depth, in-flight, requeue rate
- **Database**: Connection pool, query latency

### Alerts

Prometheus alerting rules for:

- Service down > 30s
- Error rate > 5% for 5 minutes
- P99 latency > 2s
- NSQ queue depth > 1000
- Disk usage > 80%

---

## Scaling Guidelines

| Service | Scale Strategy | Notes |
|---------|---------------|-------|
| api-gateway | Horizontal (HPA) | Stateless, scales linearly |
| image-svc | Horizontal (HPA) | CPU-bound during upload |
| facts-svc | Horizontal (HPA) | Low resource, scale for availability |
| worker-svc | Horizontal (replicas) | More workers = faster processing |
| PostgreSQL | Vertical + read replicas | Use managed DB in production |
| MinIO | Replace with S3/GCS | Object storage scales independently |
| NSQ | Add nsqd nodes | Built for horizontal scaling |

---

## Security Checklist

- [ ] TLS/HTTPS everywhere (cert-manager + Let's Encrypt)
- [ ] Database credentials in Kubernetes Secrets (or Vault)
- [ ] MinIO credentials rotated regularly
- [ ] Network policies restricting pod-to-pod traffic
- [ ] Rate limiting on API gateway
- [ ] RBAC for Kubernetes access
- [ ] Container images scanned for vulnerabilities
- [ ] No root containers (`runAsNonRoot: true`)
- [ ] Resource limits set on all pods
- [ ] Audit logging enabled
