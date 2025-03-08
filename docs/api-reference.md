# API Reference

This document provides a comprehensive reference for the Distributed Task Scheduler API.

## Base URL

```
https://tasks-api.example.com/api
```

## Authentication

All API requests require authentication using one of the following methods:

### API Key Authentication

Include your API key in the request header:

```
Authorization: ApiKey your-api-key
```

### JWT Authentication

Include a valid JWT token in the request header:

```
Authorization: Bearer your-jwt-token
```

## Response Format

All responses are returned in JSON format with the following structure:

```json
{
  "data": { ... },  // The response data (may be an object or array)
  "meta": { ... },  // Metadata about the response (pagination, etc.)
  "error": { ... }  // Error information (only present if there's an error)
}
```

## Error Handling

Errors are returned with appropriate HTTP status codes and an error object:

```json
{
  "error": {
    "code": "error_code",
    "message": "Human-readable error message",
    "details": { ... }  // Optional additional error details
  }
}
```

## Common Status Codes

- `200 OK`: Request succeeded
- `201 Created`: Resource created successfully
- `400 Bad Request`: Invalid request parameters
- `401 Unauthorized`: Authentication failed
- `403 Forbidden`: Insufficient permissions
- `404 Not Found`: Resource not found
- `409 Conflict`: Resource conflict
- `422 Unprocessable Entity`: Validation error
- `500 Internal Server Error`: Server error

## Endpoints

### Tasks

#### List Tasks

```
GET /tasks
```

Retrieves a list of tasks.

**Query Parameters:**

| Parameter | Type   | Description                                      |
|-----------|--------|--------------------------------------------------|
| status    | string | Filter by task status (pending, running, etc.)   |
| page      | int    | Page number for pagination (default: 1)          |
| limit     | int    | Number of items per page (default: 20, max: 100) |
| sort      | string | Field to sort by (created_at, priority, etc.)    |
| order     | string | Sort order (asc or desc, default: desc)          |
| tag       | string | Filter by tag                                    |

**Response:**

```json
{
  "data": [
    {
      "id": "task-123",
      "name": "Example Task",
      "description": "This is an example task",
      "command": "echo 'Hello World'",
      "status": "pending",
      "priority": 5,
      "retry_count": 0,
      "max_retries": 3,
      "created_at": "2023-06-15T10:00:00Z",
      "updated_at": "2023-06-15T10:00:00Z",
      "scheduled_at": "2023-06-15T12:00:00Z",
      "started_at": null,
      "completed_at": null,
      "timeout_seconds": 3600,
      "tags": ["example", "demo"],
      "dependencies": ["task-100", "task-101"],
      "worker_id": null,
      "result": null,
      "error": null,
      "checkpoint_data": null
    },
    // ... more tasks
  ],
  "meta": {
    "page": 1,
    "limit": 20,
    "total": 42,
    "pages": 3
  }
}
```

#### Get Task

```
GET /tasks/{id}
```

Retrieves a specific task by ID.

**Path Parameters:**

| Parameter | Type   | Description |
|-----------|--------|-------------|
| id        | string | Task ID     |

**Response:**

```json
{
  "data": {
    "id": "task-123",
    "name": "Example Task",
    "description": "This is an example task",
    "command": "echo 'Hello World'",
    "status": "pending",
    "priority": 5,
    "retry_count": 0,
    "max_retries": 3,
    "created_at": "2023-06-15T10:00:00Z",
    "updated_at": "2023-06-15T10:00:00Z",
    "scheduled_at": "2023-06-15T12:00:00Z",
    "started_at": null,
    "completed_at": null,
    "timeout_seconds": 3600,
    "tags": ["example", "demo"],
    "dependencies": ["task-100", "task-101"],
    "worker_id": null,
    "result": null,
    "error": null,
    "checkpoint_data": null
  }
}
```

#### Create Task

```
POST /tasks
```

Creates a new task.

**Request Body:**

```json
{
  "name": "Example Task",
  "description": "This is an example task",
  "command": "echo 'Hello World'",
  "priority": 5,
  "max_retries": 3,
  "scheduled_at": "2023-06-15T12:00:00Z",
  "timeout_seconds": 3600,
  "tags": ["example", "demo"],
  "dependencies": ["task-100", "task-101"],
  "checkpoint_enabled": true
}
```

**Required Fields:**

- `name`: Task name
- `command`: Command to execute

**Optional Fields:**

- `description`: Task description
- `priority`: Task priority (1-10, default: 5)
- `max_retries`: Maximum retry attempts (default: 3)
- `scheduled_at`: When to schedule the task (ISO 8601 format)
- `timeout_seconds`: Task timeout in seconds (default: 3600)
- `tags`: Array of string tags
- `dependencies`: Array of task IDs that must complete before this task runs
- `checkpoint_enabled`: Whether checkpointing is enabled (default: false)

**Response:**

```json
{
  "data": {
    "id": "task-123",
    "name": "Example Task",
    "description": "This is an example task",
    "command": "echo 'Hello World'",
    "status": "pending",
    "priority": 5,
    "retry_count": 0,
    "max_retries": 3,
    "created_at": "2023-06-15T10:00:00Z",
    "updated_at": "2023-06-15T10:00:00Z",
    "scheduled_at": "2023-06-15T12:00:00Z",
    "started_at": null,
    "completed_at": null,
    "timeout_seconds": 3600,
    "tags": ["example", "demo"],
    "dependencies": ["task-100", "task-101"],
    "worker_id": null,
    "result": null,
    "error": null,
    "checkpoint_data": null,
    "checkpoint_enabled": true
  }
}
```

#### Update Task

```
PUT /tasks/{id}
```

Updates an existing task.

**Path Parameters:**

| Parameter | Type   | Description |
|-----------|--------|-------------|
| id        | string | Task ID     |

**Request Body:**

```json
{
  "name": "Updated Task Name",
  "description": "Updated task description",
  "priority": 8,
  "max_retries": 5,
  "scheduled_at": "2023-06-16T12:00:00Z",
  "timeout_seconds": 7200,
  "tags": ["updated", "example"],
  "dependencies": ["task-100", "task-101", "task-102"]
}
```

**Note:** Only fields that need to be updated should be included in the request.

**Response:**

```json
{
  "data": {
    "id": "task-123",
    "name": "Updated Task Name",
    "description": "Updated task description",
    "command": "echo 'Hello World'",
    "status": "pending",
    "priority": 8,
    "retry_count": 0,
    "max_retries": 5,
    "created_at": "2023-06-15T10:00:00Z",
    "updated_at": "2023-06-15T11:30:00Z",
    "scheduled_at": "2023-06-16T12:00:00Z",
    "started_at": null,
    "completed_at": null,
    "timeout_seconds": 7200,
    "tags": ["updated", "example"],
    "dependencies": ["task-100", "task-101", "task-102"],
    "worker_id": null,
    "result": null,
    "error": null,
    "checkpoint_data": null
  }
}
```

#### Delete Task

```
DELETE /tasks/{id}
```

Deletes a task.

**Path Parameters:**

| Parameter | Type   | Description |
|-----------|--------|-------------|
| id        | string | Task ID     |

**Response:**

```json
{
  "data": {
    "success": true,
    "message": "Task deleted successfully"
  }
}
```

#### Cancel Task

```
POST /tasks/{id}/cancel
```

Cancels a running or pending task.

**Path Parameters:**

| Parameter | Type   | Description |
|-----------|--------|-------------|
| id        | string | Task ID     |

**Response:**

```json
{
  "data": {
    "id": "task-123",
    "status": "cancelled",
    "updated_at": "2023-06-15T11:45:00Z"
  }
}
```

#### Retry Task

```
POST /tasks/{id}/retry
```

Retries a failed task.

**Path Parameters:**

| Parameter | Type   | Description |
|-----------|--------|-------------|
| id        | string | Task ID     |

**Response:**

```json
{
  "data": {
    "id": "task-123",
    "status": "pending",
    "retry_count": 1,
    "updated_at": "2023-06-15T11:50:00Z"
  }
}
```

#### Get Task Logs

```
GET /tasks/{id}/logs
```

Retrieves logs for a specific task.

**Path Parameters:**

| Parameter | Type   | Description |
|-----------|--------|-------------|
| id        | string | Task ID     |

**Query Parameters:**

| Parameter | Type   | Description                                      |
|-----------|--------|--------------------------------------------------|
| limit     | int    | Number of log lines to return (default: 100)     |
| offset    | int    | Offset for pagination (default: 0)               |
| level     | string | Filter by log level (info, error, warn, debug)   |

**Response:**

```json
{
  "data": [
    {
      "timestamp": "2023-06-15T12:01:00Z",
      "level": "info",
      "message": "Task started execution"
    },
    {
      "timestamp": "2023-06-15T12:01:05Z",
      "level": "info",
      "message": "Processing step 1"
    },
    {
      "timestamp": "2023-06-15T12:01:10Z",
      "level": "error",
      "message": "Error in step 2: File not found"
    }
  ],
  "meta": {
    "limit": 100,
    "offset": 0,
    "total": 3
  }
}
```

#### Update Task Checkpoint

```
POST /tasks/{id}/checkpoint
```

Updates the checkpoint data for a running task.

**Path Parameters:**

| Parameter | Type   | Description |
|-----------|--------|-------------|
| id        | string | Task ID     |

**Request Body:**

```json
{
  "checkpoint_data": {
    "current_step": 3,
    "processed_items": 150,
    "last_processed_id": "item-789",
    "custom_data": {
      "key1": "value1",
      "key2": "value2"
    }
  }
}
```

**Response:**

```json
{
  "data": {
    "id": "task-123",
    "checkpoint_data": {
      "current_step": 3,
      "processed_items": 150,
      "last_processed_id": "item-789",
      "custom_data": {
        "key1": "value1",
        "key2": "value2"
      }
    },
    "updated_at": "2023-06-15T12:15:00Z"
  }
}
```

### Workers

#### List Workers

```
GET /workers
```

Retrieves a list of registered workers.

**Query Parameters:**

| Parameter | Type   | Description                                      |
|-----------|--------|--------------------------------------------------|
| status    | string | Filter by worker status (active, idle, offline)  |
| page      | int    | Page number for pagination (default: 1)          |
| limit     | int    | Number of items per page (default: 20, max: 100) |

**Response:**

```json
{
  "data": [
    {
      "id": "worker-001",
      "hostname": "worker-pod-1",
      "status": "active",
      "tasks_completed": 42,
      "tasks_failed": 3,
      "current_tasks": 2,
      "last_heartbeat": "2023-06-15T12:20:00Z",
      "registered_at": "2023-06-01T00:00:00Z",
      "version": "1.0.0",
      "tags": ["general"]
    },
    // ... more workers
  ],
  "meta": {
    "page": 1,
    "limit": 20,
    "total": 5,
    "pages": 1
  }
}
```

#### Get Worker

```
GET /workers/{id}
```

Retrieves a specific worker by ID.

**Path Parameters:**

| Parameter | Type   | Description |
|-----------|--------|-------------|
| id        | string | Worker ID   |

**Response:**

```json
{
  "data": {
    "id": "worker-001",
    "hostname": "worker-pod-1",
    "status": "active",
    "tasks_completed": 42,
    "tasks_failed": 3,
    "current_tasks": 2,
    "current_task_ids": ["task-123", "task-456"],
    "last_heartbeat": "2023-06-15T12:20:00Z",
    "registered_at": "2023-06-01T00:00:00Z",
    "version": "1.0.0",
    "tags": ["general"],
    "resources": {
      "cpu_usage": 0.45,
      "memory_usage": 0.32,
      "disk_usage": 0.15
    }
  }
}
```

#### Get Worker Tasks

```
GET /workers/{id}/tasks
```

Retrieves tasks assigned to a specific worker.

**Path Parameters:**

| Parameter | Type   | Description |
|-----------|--------|-------------|
| id        | string | Worker ID   |

**Query Parameters:**

| Parameter | Type   | Description                                      |
|-----------|--------|--------------------------------------------------|
| status    | string | Filter by task status                            |
| page      | int    | Page number for pagination (default: 1)          |
| limit     | int    | Number of items per page (default: 20, max: 100) |

**Response:**

```json
{
  "data": [
    {
      "id": "task-123",
      "name": "Example Task 1",
      "status": "running",
      "started_at": "2023-06-15T12:10:00Z",
      "priority": 5
    },
    {
      "id": "task-456",
      "name": "Example Task 2",
      "status": "running",
      "started_at": "2023-06-15T12:15:00Z",
      "priority": 8
    }
  ],
  "meta": {
    "page": 1,
    "limit": 20,
    "total": 2,
    "pages": 1
  }
}
```

### System

#### Get System Status

```
GET /system/status
```

Retrieves the overall system status.

**Response:**

```json
{
  "data": {
    "status": "healthy",
    "version": "1.0.0",
    "uptime": 1209600,
    "components": {
      "api_server": {
        "status": "healthy",
        "instance_count": 3
      },
      "scheduler": {
        "status": "healthy",
        "leader_id": "scheduler-0",
        "instance_count": 2
      },
      "workers": {
        "status": "healthy",
        "active_count": 5,
        "idle_count": 2,
        "offline_count": 0
      },
      "database": {
        "status": "healthy",
        "connection_count": 25
      },
      "redis": {
        "status": "healthy",
        "memory_usage": 0.35
      }
    },
    "task_stats": {
      "pending": 15,
      "running": 8,
      "completed_today": 142,
      "failed_today": 3
    },
    "queue_depths": {
      "high_priority": 2,
      "normal_priority": 10,
      "low_priority": 3
    }
  }
}
```

#### Get System Metrics

```
GET /system/metrics
```

Retrieves system metrics.

**Query Parameters:**

| Parameter | Type   | Description                                      |
|-----------|--------|--------------------------------------------------|
| period    | string | Time period (hour, day, week, month)             |
| metrics   | string | Comma-separated list of metrics to include       |

**Response:**

```json
{
  "data": {
    "task_completion_rate": [
      {"timestamp": "2023-06-15T11:00:00Z", "value": 12},
      {"timestamp": "2023-06-15T12:00:00Z", "value": 15},
      {"timestamp": "2023-06-15T13:00:00Z", "value": 10}
    ],
    "error_rate": [
      {"timestamp": "2023-06-15T11:00:00Z", "value": 0.02},
      {"timestamp": "2023-06-15T12:00:00Z", "value": 0.01},
      {"timestamp": "2023-06-15T13:00:00Z", "value": 0.03}
    ],
    "average_execution_time": [
      {"timestamp": "2023-06-15T11:00:00Z", "value": 45.2},
      {"timestamp": "2023-06-15T12:00:00Z", "value": 42.8},
      {"timestamp": "2023-06-15T13:00:00Z", "value": 47.5}
    ]
  },
  "meta": {
    "period": "hour",
    "start_time": "2023-06-15T11:00:00Z",
    "end_time": "2023-06-15T13:00:00Z"
  }
}
```

## Webhook Notifications

The system can send webhook notifications for task events. Configure webhooks in the system settings.

### Webhook Payload Format

```json
{
  "event": "task.completed",
  "timestamp": "2023-06-15T12:30:00Z",
  "task": {
    "id": "task-123",
    "name": "Example Task",
    "status": "completed",
    "result": "Task executed successfully",
    "started_at": "2023-06-15T12:00:00Z",
    "completed_at": "2023-06-15T12:30:00Z"
  }
}
```

### Supported Events

- `task.created`: Task created
- `task.started`: Task execution started
- `task.completed`: Task completed successfully
- `task.failed`: Task execution failed
- `task.cancelled`: Task cancelled
- `task.retrying`: Task being retried
- `worker.registered`: New worker registered
- `worker.offline`: Worker went offline

## Rate Limiting

API requests are rate-limited based on your subscription tier. Rate limit information is included in response headers:

```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1623760800
```

If you exceed the rate limit, you'll receive a `429 Too Many Requests` response.

## Pagination

List endpoints support pagination using the `page` and `limit` query parameters. Pagination metadata is included in the response:

```json
"meta": {
  "page": 1,
  "limit": 20,
  "total": 42,
  "pages": 3
}
```

## Filtering and Sorting

Most list endpoints support filtering using query parameters and sorting using the `sort` and `order` parameters.

Example:

```
GET /tasks?status=pending&sort=priority&order=desc
```

## Versioning

The API is versioned using the `Accept` header:

```
Accept: application/json; version=1.0
```

If not specified, the latest version is used. 