#!/bin/sh

# Check if the worker process is running
if ! pgrep -f "worker" > /dev/null; then
  echo "Worker process is not running"
  exit 1
fi

# Check if the process has been running for at least 5 seconds
# This helps avoid false positives during startup
UPTIME=$(ps -o etimes= -p $(pgrep -f "worker"))
if [ -z "$UPTIME" ] || [ "$UPTIME" -lt 5 ]; then
  echo "Worker process is starting up"
  exit 0
fi

# Check if worker can connect to Redis
if command -v redis-cli > /dev/null; then
  if ! redis-cli -h ${REDIS_HOST:-redis} -p ${REDIS_PORT:-6379} ping > /dev/null; then
    echo "Cannot connect to Redis"
    exit 1
  fi
fi

# Check if worker can connect to database
# This is a simple check - in production you might want to use a more sophisticated approach
if command -v pg_isready > /dev/null; then
  if ! pg_isready -h ${DB_HOST:-postgres} -p ${DB_PORT:-5432} -U ${DB_USER:-taskuser} > /dev/null; then
    echo "Cannot connect to PostgreSQL"
    exit 1
  fi
fi

# All checks passed
echo "Worker is healthy"
exit 0 