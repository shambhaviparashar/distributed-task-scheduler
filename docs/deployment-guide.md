# Deployment Guide for Distributed Task Scheduler

This guide provides step-by-step instructions for deploying the Distributed Task Scheduler system to a Kubernetes environment.

## Prerequisites

Before you begin, ensure you have the following:

- Kubernetes cluster (v1.21+)
- kubectl configured to access your cluster
- Docker installed locally (for development)
- Helm v3 (optional, for additional components)
- Domain names configured (for production deployment)
- SSL certificates or cert-manager for TLS

## Architecture Overview

The system consists of the following components:

- API Server - REST API for task management
- Scheduler - Leader election-based task scheduling service
- Worker - Task execution service
- PostgreSQL - Database for task state
- Redis - Distributed coordination and queue

## Deployment Options

### 1. Using Docker Compose (Development/Testing)

For local development or testing, you can use Docker Compose:

```bash
# Start all services
docker-compose up -d

# View logs
docker-compose logs -f

# Stop all services
docker-compose down
```

### 2. Using Kubernetes

#### Step 1: Create the namespace

```bash
kubectl apply -f kubernetes/namespace.yaml
```

#### Step 2: Create ConfigMap and Secrets

```bash
# Create ConfigMap
kubectl apply -f kubernetes/configmap.yaml

# Create Secrets (replace with your actual passwords)
kubectl create secret generic db-credentials \
  --namespace=task-scheduler \
  --from-literal=DB_USER=taskuser \
  --from-literal=DB_PASSWORD=your-secure-password

kubectl create secret generic redis-credentials \
  --namespace=task-scheduler \
  --from-literal=REDIS_PASSWORD=your-redis-password
```

#### Step 3: Deploy Database and Redis

```bash
kubectl apply -f kubernetes/postgres.yaml
kubectl apply -f kubernetes/redis.yaml

# Wait for them to be ready
kubectl wait --for=condition=ready pod -l app=postgres --timeout=120s -n task-scheduler
kubectl wait --for=condition=ready pod -l app=redis --timeout=120s -n task-scheduler
```

#### Step 4: Deploy Application Components

```bash
# Deploy API server, scheduler, and workers
kubectl apply -f kubernetes/api-server.yaml
kubectl apply -f kubernetes/scheduler.yaml
kubectl apply -f kubernetes/worker.yaml
```

#### Step 5: Deploy Network Policies and Ingress

```bash
# Apply network policies
kubectl apply -f kubernetes/network-policies.yaml

# Apply ingress (ensure you have an ingress controller installed)
kubectl apply -f kubernetes/ingress.yaml
```

#### Step 6: Deploy Monitoring

```bash
# Deploy Prometheus and Grafana
kubectl apply -f kubernetes/monitoring.yaml
```

## Configuration Options

### Environment Variables

The following environment variables can be configured:

| Variable | Description | Default |
|----------|-------------|---------|
| API_PORT | Port for the API server | 8080 |
| METRICS_PORT | Port for metrics scraping | 9090 |
| LOG_LEVEL | Logging level (debug, info, warn, error) | info |
| DB_HOST | PostgreSQL host | postgres |
| DB_PORT | PostgreSQL port | 5432 |
| DB_USER | PostgreSQL user | taskuser |
| DB_PASSWORD | PostgreSQL password | (from secret) |
| DB_NAME | PostgreSQL database name | taskdb |
| REDIS_ADDR | Redis address | redis:6379 |
| REDIS_PASSWORD | Redis password | (from secret) |
| WORKER_COUNT | Number of concurrent tasks per worker | 5 |

## Scaling Guidelines

### API Server

The API server can be scaled horizontally based on load. CPU and memory usage are the main metrics to watch.

```bash
# Scale manually
kubectl scale deployment api-server --replicas=5 -n task-scheduler

# The Horizontal Pod Autoscaler (HPA) is already configured
```

### Scheduler

The scheduler uses a leader election mechanism, so only one instance will be active at a time. Multiple instances provide high availability.

```bash
# Normally you don't need to scale this beyond 2-3 replicas for HA
kubectl scale statefulset scheduler --replicas=3 -n task-scheduler
```

### Workers

Workers can be scaled based on queue depth and CPU/memory usage.

```bash
# Scale manually
kubectl scale deployment worker --replicas=10 -n task-scheduler

# The Horizontal Pod Autoscaler (HPA) is already configured
```

## Monitoring

The system includes Prometheus and Grafana for monitoring. The following metrics are available:

- Task execution metrics (count, duration, success rate)
- Queue metrics (depth, latency)
- Worker metrics (utilization, task processing rate)
- System metrics (CPU, memory, network)

Access Grafana at: http://tasks-metrics.example.com

## Backup and Restore

### PostgreSQL Backup

```bash
# Create a backup
kubectl exec -n task-scheduler postgres-0 -- pg_dump -U taskuser taskdb > taskdb_backup.sql

# Restore from backup
cat taskdb_backup.sql | kubectl exec -i -n task-scheduler postgres-0 -- psql -U taskuser taskdb
```

### Redis Backup

Redis is configured with AOF persistence. For manual backup:

```bash
# Trigger BGSAVE
kubectl exec -n task-scheduler redis-0 -- redis-cli BGSAVE
```

## Troubleshooting

### Common Issues

1. **Scheduler Leader Election Issues**
   - Check Redis connectivity
   - Inspect scheduler logs: `kubectl logs -l app=scheduler -n task-scheduler`

2. **Worker Task Processing Issues**
   - Check task status in database
   - Inspect worker logs: `kubectl logs -l app=worker -n task-scheduler`

3. **API Server Connection Issues**
   - Check network policies
   - Verify service and ingress configuration

### Health Checks

```bash
# Check API server health
kubectl exec -n task-scheduler deploy/api-server -- wget -q -O- http://localhost:8080/health

# Get system information
kubectl exec -n task-scheduler deploy/api-server -- wget -q -O- http://localhost:8080/api/system/info
```

## Disaster Recovery

In case of cluster failure:

1. Restore PostgreSQL data from backups
2. Redeploy all components
3. The system will automatically recover and resume task processing

## Upgrading

For zero-downtime upgrades:

1. Update container images with new version
2. Apply the changes with rolling update strategy:

```bash
kubectl set image deployment/api-server api-server=new-image:tag -n task-scheduler
kubectl set image deployment/worker worker=new-image:tag -n task-scheduler
kubectl set image statefulset/scheduler scheduler=new-image:tag -n task-scheduler
```

## Security Considerations

1. **Secrets Management**
   - All sensitive data is stored in Kubernetes Secrets
   - Consider using a dedicated secrets management solution for production

2. **Network Security**
   - Network policies restrict pod-to-pod communication
   - TLS is enabled for all external traffic

3. **Authentication**
   - JWT authentication is available for API access (configure in ConfigMap)
   - API keys can be enabled for simple authentication 