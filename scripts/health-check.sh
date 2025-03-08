#!/bin/sh

# Check if the scheduler process is running
if ! pgrep -f "scheduler" > /dev/null; then
  echo "Scheduler process is not running"
  exit 1
fi

# Check if the process has been running for at least 5 seconds
# This helps avoid false positives during startup
UPTIME=$(ps -o etimes= -p $(pgrep -f "scheduler"))
if [ -z "$UPTIME" ] || [ "$UPTIME" -lt 5 ]; then
  echo "Scheduler process is starting up"
  exit 0
fi

# Optional: Try to connect to Redis to verify connectivity
if command -v redis-cli > /dev/null; then
  if ! redis-cli -h ${REDIS_HOST:-redis} -p ${REDIS_PORT:-6379} ping > /dev/null; then
    echo "Cannot connect to Redis"
    exit 1
  fi
fi

# All checks passed
echo "Scheduler is healthy"
exit 0 