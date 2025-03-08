# Operational Procedures

This document provides guidance for day-to-day operations and maintenance of the Distributed Task Scheduler system.

## Routine Maintenance

### Database Maintenance

#### PostgreSQL Vacuum

Regular vacuuming helps maintain database performance:

```bash
# Connect to the PostgreSQL pod
kubectl exec -it -n task-scheduler postgres-0 -- bash

# Run vacuum on the tasks table
psql -U taskuser -d taskdb -c "VACUUM ANALYZE tasks;"
```

Schedule this monthly, or more frequently for high-traffic systems.

#### Index Maintenance

Rebuild indexes periodically:

```bash
psql -U taskuser -d taskdb -c "REINDEX TABLE tasks;"
```

### Log Rotation

The system uses the Kubernetes logging infrastructure. If you're using a custom logging solution:

1. Ensure log rotation is configured for all components
2. Archive logs older than 30 days
3. Consider using a log aggregation system like ELK or Loki

### Backup Schedule

Follow this backup schedule:

1. **Daily**: PostgreSQL database full backup
2. **Hourly**: Transaction log backup
3. **Weekly**: Full system backup including Redis persistence

## Monitoring and Alerting

### Key Metrics to Monitor

1. **System Health**
   - API Server response time (alert if > 500ms)
   - Queue depth (alert if consistently growing)
   - Error rate (alert if > 1%)

2. **Task Execution**
   - Failed tasks rate (alert if > 5%)
   - Execution time (alert on anomalies)
   - Dead letter queue size (alert if > 10)

3. **Resource Usage**
   - CPU usage (alert if > 80% for 5 minutes)
   - Memory usage (alert if > 80% for 5 minutes)
   - Disk space (alert if > 85%)

### Setting Up Alerts

Example Prometheus alerting rules:

```yaml
groups:
- name: task-scheduler
  rules:
  - alert: HighErrorRate
    expr: rate(api_errors_total[5m]) / rate(api_requests_total[5m]) > 0.01
    for: 2m
    labels:
      severity: warning
    annotations:
      summary: "High API error rate"
      description: "Error rate is {{ $value | humanizePercentage }} for the last 5 minutes"

  - alert: QueueGrowing
    expr: task_queue_size > 100 and rate(task_queue_size[10m]) > 0
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "Task queue is growing"
      description: "Queue has {{ $value }} items and is growing"
```

Add these to your Prometheus configuration.

## Scaling Procedures

### Vertical Scaling

To increase resources for a component:

```bash
# Edit the deployment
kubectl edit deployment api-server -n task-scheduler

# Update resource limits and requests
# resources:
#   requests:
#     memory: "512Mi"  # Increase from 256Mi
#     cpu: "400m"      # Increase from 200m
#   limits:
#     memory: "1Gi"    # Increase from 512Mi
#     cpu: "1000m"     # Increase from 500m
```

### Horizontal Scaling

For handling increased load:

```bash
# Scale workers
kubectl scale deployment worker -n task-scheduler --replicas=15

# Adjust HPA
kubectl edit hpa worker-hpa -n task-scheduler
# Update maxReplicas to desired value
```

## Troubleshooting

### Handling Failed Tasks

For tasks in the dead letter queue:

1. Inspect the task details:
   ```bash
   kubectl exec -it -n task-scheduler postgres-0 -- psql -U taskuser -d taskdb -c "SELECT * FROM tasks WHERE status = 'dead_letter' LIMIT 5;"
   ```

2. Retry specific tasks:
   ```bash
   # Create a script to retry a specific task
   cat <<EOF > retry.sql
   UPDATE tasks SET status = 'pending', retry_count = 0, updated_at = NOW() WHERE id = '[TASK_ID]';
   EOF
   
   kubectl cp retry.sql task-scheduler/postgres-0:/tmp/
   kubectl exec -it -n task-scheduler postgres-0 -- psql -U taskuser -d taskdb -f /tmp/retry.sql
   ```

### Recovering from Worker Node Failures

If a worker node fails:

1. Check for stuck tasks:
   ```bash
   kubectl exec -it -n task-scheduler postgres-0 -- psql -U taskuser -d taskdb -c "SELECT * FROM tasks WHERE status = 'running' AND updated_at < NOW() - INTERVAL '30 minutes';"
   ```

2. Reset stuck tasks:
   ```bash
   kubectl exec -it -n task-scheduler postgres-0 -- psql -U taskuser -d taskdb -c "UPDATE tasks SET status = 'pending', worker_id = NULL WHERE status = 'running' AND updated_at < NOW() - INTERVAL '30 minutes';"
   ```

### Handling Database Connection Issues

If services cannot connect to the database:

1. Check database health:
   ```bash
   kubectl exec -it -n task-scheduler postgres-0 -- pg_isready
   ```

2. Check network policies:
   ```bash
   kubectl describe networkpolicy db-policy -n task-scheduler
   ```

3. Verify service DNS resolution:
   ```bash
   kubectl run -it --rm debug --image=alpine -n task-scheduler -- nslookup postgres
   ```

## Disaster Recovery

### Complete System Restore

In case of catastrophic failure:

1. Create a new Kubernetes cluster if needed
2. Deploy all infrastructure components
3. Restore PostgreSQL from backup:
   ```bash
   # Copy backup to new PostgreSQL pod
   kubectl cp taskdb_backup.sql task-scheduler/postgres-0:/tmp/
   
   # Restore database
   kubectl exec -it -n task-scheduler postgres-0 -- psql -U taskuser -d taskdb -f /tmp/taskdb_backup.sql
   ```

4. Deploy application components
5. Verify system functionality

### Data Recovery

To recover specific data:

1. Identify what needs to be recovered
2. Extract specific data from backups
3. Use database tools to import only the necessary data

## Security Procedures

### Credential Rotation

Rotate credentials regularly:

1. Update database password:
   ```bash
   # Generate a new password
   NEW_PASSWORD=$(openssl rand -base64 20)
   
   # Update in PostgreSQL
   kubectl exec -it -n task-scheduler postgres-0 -- psql -U taskuser -d taskdb -c "ALTER USER taskuser WITH PASSWORD '$NEW_PASSWORD';"
   
   # Update in Kubernetes secret
   kubectl create secret generic db-credentials \
     --namespace=task-scheduler \
     --from-literal=DB_USER=taskuser \
     --from-literal=DB_PASSWORD=$NEW_PASSWORD \
     --dry-run=client -o yaml | kubectl apply -f -
   
   # Restart dependent services
   kubectl rollout restart deployment/api-server deployment/worker statefulset/scheduler -n task-scheduler
   ```

2. Follow similar procedure for Redis password

### Security Auditing

Perform regular security audits:

1. Scan for vulnerabilities:
   ```bash
   # Using Trivy for container scanning
   trivy image ${REGISTRY_HOST}/task-scheduler/api-server:latest
   ```

2. Review Kubernetes RBAC permissions
3. Check network policies for any unintended exposure

## Performance Tuning

### Database Optimization

Optimize PostgreSQL for better performance:

```bash
kubectl exec -it -n task-scheduler postgres-0 -- bash

# Edit postgresql.conf
vi /var/lib/postgresql/data/postgresql.conf

# Add or modify:
# shared_buffers = 256MB
# effective_cache_size = 768MB
# work_mem = 4MB
# maintenance_work_mem = 64MB
```

### Worker Optimization

Tune worker settings for better task execution:

1. Adjust worker count based on the typical resource usage of your tasks
2. Monitor and optimize the concurrent task count per worker
3. Consider creating specialized worker pools for different task types 