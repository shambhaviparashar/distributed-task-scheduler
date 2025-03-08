# Distributed Task Scheduler Architecture

This document describes the architecture of the Distributed Task Scheduler system, its components, and their interactions.

## System Overview

The Distributed Task Scheduler is a robust, fault-tolerant system designed to reliably schedule and execute tasks in a distributed environment. It provides features like task dependency resolution, retries with exponential backoff, and checkpointing for resumable tasks.

### Key Features

- **Distributed coordination** with leader election
- **Fault tolerance** with automatic failover
- **Task dependencies** and scheduling based on dependencies
- **Retry mechanism** with exponential backoff
- **Task checkpointing** for resumable tasks
- **Priority queues** for task execution ordering
- **Metrics and monitoring** for operational visibility
- **Horizontal scalability** for all components

## Architecture Diagram

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│                 │     │                 │     │                 │
│   API Server    │◄────┤    Scheduler    │◄────┤     Workers     │
│                 │     │   (Active HA)   │     │  (Horizontally  │
│                 │     │                 │     │    Scalable)    │
└────────┬────────┘     └────────┬────────┘     └────────┬────────┘
         │                       │                       │
         │                       │                       │
         ▼                       ▼                       │
┌────────┴────────┐     ┌────────┴────────┐             │
│                 │     │                 │             │
│   PostgreSQL    │     │     Redis       │◄────────────┘
│  (Persistent    │     │ (Coordination   │
│    Storage)     │     │   & Queues)     │
│                 │     │                 │
└─────────────────┘     └─────────────────┘

                  ┌─────────────────┐
                  │                 │
                  │  Prometheus &   │
                  │    Grafana      │
                  │  (Monitoring)   │
                  │                 │
                  └─────────────────┘
```

## Component Details

### API Server

The API Server provides a RESTful interface for task management.

**Responsibilities**:
- Create, read, update, and delete tasks
- Provide task status information
- Expose health check and metrics endpoints

**Implementation**:
- Built with Go's standard HTTP package and Chi router
- Stateless design for horizontal scalability
- Configurable via environment variables

**Endpoints**:
- `POST /api/tasks` - Create a new task
- `GET /api/tasks` - List all tasks
- `GET /api/tasks/{id}` - Get task details
- `PUT /api/tasks/{id}` - Update task
- `DELETE /api/tasks/{id}` - Delete task
- `/health/*` - Health check endpoints
- `/metrics` - Prometheus metrics endpoint

### Scheduler

The Scheduler determines which tasks are ready to run based on their schedules and dependencies.

**Responsibilities**:
- Identify tasks due for execution
- Resolve task dependencies
- Place ready tasks onto the execution queue
- Handle task priorities

**Implementation**:
- Uses Redis-based distributed locking for leader election
- Only one active scheduler instance at a time
- Automatic failover if the leader fails
- Uses cron parsing for schedule evaluation

**Fault Tolerance**:
- Multiple instances deployed in active-passive configuration
- Heartbeat mechanism to detect failures
- Automatic promotion of standby to leader

### Worker

Workers consume tasks from the queue and execute them.

**Responsibilities**:
- Execute tasks in their own environment
- Handle retries with exponential backoff
- Manage task timeouts
- Report task status and results
- Support task checkpointing

**Implementation**:
- Horizontally scalable
- Configurable concurrency per worker
- Graceful shutdown with in-flight task completion

**Fault Tolerance**:
- Heartbeat mechanism to detect failures
- Stale task detection and recovery
- Automatic task reassignment on worker failure

### Storage Layer

#### PostgreSQL

PostgreSQL provides persistent storage for task definitions and state.

**Stored Data**:
- Task definitions
- Task execution history
- Task dependencies
- Task state and status

**Schema**:
- `tasks` table with comprehensive task information
- `dead_letter_tasks` table for permanently failed tasks

**Features**:
- Transaction support for ACID guarantees
- Optimistic concurrency control via versioning
- Indexes for efficient querying

#### Redis

Redis serves as both a coordination mechanism and messaging queue.

**Used For**:
- Distributed locking for leader election
- Task queues with priority support
- Worker registration and heartbeats
- Fast, in-memory operations

**Queue Implementation**:
- Sorted sets for priority queues
- Publish/subscribe for worker communication
- Key expiration for heartbeat detection

## Interactions and Data Flow

1. **Task Creation**:
   - Client submits task via API Server
   - API Server validates and stores task in PostgreSQL
   - Task starts in `pending` state

2. **Task Scheduling**:
   - Scheduler queries for pending tasks
   - Validates schedules and dependencies
   - Places eligible tasks in Redis queue with priority

3. **Task Execution**:
   - Worker pulls task from Redis queue
   - Updates task status to `running`
   - Executes task command
   - Updates task with results
   - Handles retries if needed

4. **Task Completion**:
   - Worker updates task status to `completed` or `failed`
   - If dependencies exist, dependent tasks are re-evaluated
   - Failed tasks may be retried or moved to dead letter queue

## Fault Tolerance Mechanisms

### Leader Election

The scheduler employs a leader election mechanism to ensure only one scheduler is active at a time:

1. Each scheduler instance attempts to acquire a lock in Redis
2. Only one instance can hold the lock (the leader)
3. The leader periodically refreshes the lock
4. If the leader fails, another instance acquires the lock
5. The new leader recovers in-flight tasks

### Worker Fault Detection

Workers are monitored for failures:

1. Workers send periodic heartbeats to Redis
2. The scheduler monitors these heartbeats
3. If a worker fails to send heartbeats, it's considered failed
4. Tasks assigned to the failed worker are reassigned

### Task Recovery

The system employs multiple mechanisms to recover from task failures:

1. **Retry Logic**: Failed tasks are retried with exponential backoff
2. **Checkpointing**: Long-running tasks can save progress
3. **Dead Letter Queue**: Persistently failing tasks are moved for analysis
4. **Stale Task Detection**: Tasks that haven't updated status are reassigned

## Scaling Considerations

The system is designed to scale horizontally:

- **API Server**: Stateless, can scale to handle request load
- **Scheduler**: Active-passive with automatic leader election
- **Workers**: Horizontally scalable based on queue depth
- **Database**: Can be configured with read replicas for scaling reads

## Security Architecture

1. **Network Isolation**:
   - Network policies restrict pod-to-pod communication
   - Ingress controller handles external traffic

2. **Authentication & Authorization**:
   - API authentication via JWT or API keys
   - Role-based access control for operations
   - Database credentials stored in Kubernetes Secrets

3. **Data Protection**:
   - TLS for all external communication
   - Encrypted secrets
   - Secure command execution environment

## Monitoring Architecture

The system includes comprehensive monitoring:

1. **Prometheus Metrics**:
   - Task execution metrics
   - Queue depth and latency
   - System resource utilization

2. **Health Checks**:
   - Liveness probes for process health
   - Readiness probes for service availability
   - Deep health checks for dependencies

3. **Logging**:
   - Structured JSON logs
   - Consistent correlation IDs across components
   - Log levels configurable at runtime

## Future Extensions

The architecture supports future extensions:

1. **Multi-tenancy**: Isolation between different users or teams
2. **Workflow Orchestration**: Complex task graphs and dependencies
3. **Custom Executors**: Specialized execution environments
4. **Event-driven Tasks**: Trigger tasks based on external events
5. **Distributed Tracing**: End-to-end visibility of task execution 