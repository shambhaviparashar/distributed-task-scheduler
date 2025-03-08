# Distributed Task Scheduler with Fault Tolerance

A robust distributed task scheduling system built in Go with support for task dependencies, retries, and fault tolerance.

## Architecture Overview

The system consists of several core components:

1. **API Server**: REST API for task management
2. **Scheduler**: Reads due tasks and resolves dependencies
3. **Worker**: Executes tasks and handles retries
4. **Database**: PostgreSQL for task persistence
5. **Queue**: Redis for task distribution

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│             │    │             │    │             │
│  API Server │◄───┤  Scheduler  │◄───┤   Workers   │
│             │    │             │    │             │
└──────┬──────┘    └──────┬──────┘    └──────┬──────┘
       │                  │                  │
       ▼                  ▼                  │
┌─────────────┐    ┌─────────────┐          │
│             │    │             │          │
│ PostgreSQL  │    │    Redis    │◄─────────┘
│             │    │             │
└─────────────┘    └─────────────┘
```

## Development Setup

### Prerequisites

- Git: https://git-scm.com/download/win
- Go (latest stable): https://golang.org/dl/
- Docker Desktop for Windows: https://www.docker.com/products/docker-desktop
- Visual Studio Code: https://code.visualstudio.com/download
- WSL2 (Windows Subsystem for Linux)
- Kubernetes CLI (kubectl)

### Environment Setup

1. **Configure Go Environment**:
   ```
   # Add to your PATH
   set GOPATH=%USERPROFILE%\go
   set PATH=%PATH%;%GOPATH%\bin
   ```

2. **Clone the Repository**:
   ```
   git clone https://github.com/yourusername/taskscheduler.git
   cd taskscheduler
   ```

3. **Start Infrastructure**:
   ```
   # Start Redis
   docker run -d -p 6379:6379 --name redis redis:alpine

   # Start PostgreSQL
   docker run -d -p 5432:5432 -e POSTGRES_PASSWORD=password -e POSTGRES_USER=taskuser -e POSTGRES_DB=taskdb --name postgres postgres:13
   ```

4. **Initialize and Run**:
   ```
   go mod tidy
   go run cmd/api/main.go
   ```

## Project Structure

```
├── cmd/
│   ├── api/        # API Server entry point
│   ├── scheduler/  # Scheduler service entry point
│   └── worker/     # Worker service entry point
├── internal/
│   ├── api/        # API handlers and routes
│   ├── model/      # Data models
│   ├── repository/ # Database operations
│   ├── scheduler/  # Scheduler logic
│   └── worker/     # Worker logic
├── pkg/
│   ├── config/     # Configuration helpers
│   ├── database/   # Database connection
│   └── queue/      # Queue operations
├── migrations/     # Database migrations
├── go.mod          # Go modules file
└── README.md       # This file
```

## API Endpoints

- `POST /api/tasks` - Create a new task
- `GET /api/tasks` - List all tasks
- `GET /api/tasks/{id}` - Get task details
- `PUT /api/tasks/{id}` - Update task
- `DELETE /api/tasks/{id}` - Delete task

## Fault Tolerance and High Availability Features

### Distributed Coordination

- **Leader Election:** Ensures only one scheduler instance is active using Redis-based distributed locking
- **Worker Registration:** Tracks active workers through a heartbeat mechanism
- **Automatic Failover:** Detects and handles failures of both schedulers and workers
- **Health Checks:** Comprehensive health monitoring for all components

### Task Recovery

- **Heartbeat Mechanism:** Workers send periodic heartbeats to indicate they're alive
- **Task Checkpointing:** Allows tasks to be resumed from where they left off
- **Exponential Backoff:** Implements intelligent retry logic with backoffs
- **Dead Letter Queue:** Captures permanently failed tasks for analysis

### State Management

- **Optimistic Concurrency Control:** Prevents conflicting updates to tasks
- **Transaction Support:** Ensures data consistency during critical operations
- **State Machine:** Clear task lifecycle management (pending → scheduled → running → completed/failed)
- **Idempotent Execution:** Tasks can be safely retried without side effects

### Horizontal Scaling

- **Dynamic Worker Scaling:** Workers can be added or removed without disruption
- **Graceful Shutdown:** Allows in-progress tasks to complete before shutdown
- **Priority Queues:** Tasks can be scheduled with different priority levels
- **Fair Scheduling:** Ensures all tasks get processed efficiently

### Observability

- **Prometheus Metrics:** Comprehensive metrics collection for all components
- **Health Check Endpoints:** HTTP endpoints for liveness and readiness probes
- **Structured Logging:** Consistent, parseable logs with relevant context

## Deployment

The system is designed to be deployed as multiple instances of each component:

- Multiple API servers behind a load balancer
- Multiple scheduler instances with leader election (only one is active)
- Multiple worker instances processing tasks in parallel

Each component can be independently scaled based on load requirements. 