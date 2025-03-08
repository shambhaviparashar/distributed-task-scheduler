package metrics

import (
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	once       sync.Once
	instance   *Metrics
	registry   *prometheus.Registry
	registerer prometheus.Registerer
)

// Metrics is a singleton that manages all system metrics
type Metrics struct {
	// Task metrics
	TasksCreated         prometheus.Counter
	TasksScheduled       prometheus.Counter
	TasksCompleted       prometheus.Counter
	TasksFailed          prometheus.Counter
	TasksRetried         prometheus.Counter
	TasksInDeadLetter    prometheus.Counter
	TaskExecutionTime    prometheus.Histogram
	TaskExecutionsByName *prometheus.CounterVec
	TaskDurationByName   *prometheus.HistogramVec
	
	// Queue metrics
	QueueSize            *prometheus.GaugeVec
	QueueLatency         prometheus.Histogram
	
	// Worker metrics
	WorkerCount          prometheus.Gauge
	WorkerUtilization    *prometheus.GaugeVec
	WorkerTasksProcessed *prometheus.CounterVec
	
	// System metrics
	APIRequests          *prometheus.CounterVec
	APIRequestDuration   *prometheus.HistogramVec
	
	// Custom metrics collections for user stats
	customCounters       map[string]*prometheus.CounterVec
	customGauges         map[string]*prometheus.GaugeVec
	customHistograms     map[string]*prometheus.HistogramVec
	
	mu                  sync.RWMutex
}

// GetInstance returns the singleton metrics instance
func GetInstance() *Metrics {
	once.Do(func() {
		// Create a new registry
		registry = prometheus.NewRegistry()
		registerer = promauto.With(registry)
		
		instance = &Metrics{
			// Task metrics
			TasksCreated: registerer.NewCounter(prometheus.CounterOpts{
				Name: "scheduler_tasks_created_total",
				Help: "Total number of tasks created",
			}),
			
			TasksScheduled: registerer.NewCounter(prometheus.CounterOpts{
				Name: "scheduler_tasks_scheduled_total",
				Help: "Total number of tasks scheduled",
			}),
			
			TasksCompleted: registerer.NewCounter(prometheus.CounterOpts{
				Name: "scheduler_tasks_completed_total",
				Help: "Total number of tasks completed successfully",
			}),
			
			TasksFailed: registerer.NewCounter(prometheus.CounterOpts{
				Name: "scheduler_tasks_failed_total",
				Help: "Total number of tasks that failed",
			}),
			
			TasksRetried: registerer.NewCounter(prometheus.CounterOpts{
				Name: "scheduler_tasks_retried_total",
				Help: "Total number of task retries",
			}),
			
			TasksInDeadLetter: registerer.NewCounter(prometheus.CounterOpts{
				Name: "scheduler_tasks_in_dead_letter_total",
				Help: "Total number of tasks moved to dead letter queue",
			}),
			
			TaskExecutionTime: registerer.NewHistogram(prometheus.HistogramOpts{
				Name: "scheduler_task_execution_duration_seconds",
				Help: "Task execution duration in seconds",
				Buckets: prometheus.ExponentialBuckets(0.01, 2, 15), // From 10ms to ~5min
			}),
			
			TaskExecutionsByName: registerer.NewCounterVec(
				prometheus.CounterOpts{
					Name: "scheduler_task_executions_by_name_total",
					Help: "Total number of task executions by task name",
				},
				[]string{"task_name", "status"},
			),
			
			TaskDurationByName: registerer.NewHistogramVec(
				prometheus.HistogramOpts{
					Name: "scheduler_task_duration_by_name_seconds",
					Help: "Task duration by task name in seconds",
					Buckets: prometheus.ExponentialBuckets(0.01, 2, 15),
				},
				[]string{"task_name"},
			),
			
			// Queue metrics
			QueueSize: registerer.NewGaugeVec(
				prometheus.GaugeOpts{
					Name: "scheduler_queue_size",
					Help: "Current number of tasks in queue",
				},
				[]string{"queue_name", "priority"},
			),
			
			QueueLatency: registerer.NewHistogram(prometheus.HistogramOpts{
				Name: "scheduler_queue_latency_seconds",
				Help: "Time tasks spend in queue before being processed",
				Buckets: prometheus.ExponentialBuckets(0.01, 2, 15),
			}),
			
			// Worker metrics
			WorkerCount: registerer.NewGauge(prometheus.GaugeOpts{
				Name: "scheduler_worker_count",
				Help: "Current number of active workers",
			}),
			
			WorkerUtilization: registerer.NewGaugeVec(
				prometheus.GaugeOpts{
					Name: "scheduler_worker_utilization",
					Help: "Worker utilization percentage",
				},
				[]string{"worker_id"},
			),
			
			WorkerTasksProcessed: registerer.NewCounterVec(
				prometheus.CounterOpts{
					Name: "scheduler_worker_tasks_processed_total",
					Help: "Total number of tasks processed by each worker",
				},
				[]string{"worker_id", "status"},
			),
			
			// API metrics
			APIRequests: registerer.NewCounterVec(
				prometheus.CounterOpts{
					Name: "scheduler_api_requests_total",
					Help: "Total number of API requests",
				},
				[]string{"endpoint", "method", "status"},
			),
			
			APIRequestDuration: registerer.NewHistogramVec(
				prometheus.HistogramOpts{
					Name: "scheduler_api_request_duration_seconds",
					Help: "API request duration in seconds",
					Buckets: prometheus.ExponentialBuckets(0.001, 2, 10), // From 1ms to ~1s
				},
				[]string{"endpoint", "method"},
			),
			
			// Initialize maps for custom metrics
			customCounters:   make(map[string]*prometheus.CounterVec),
			customGauges:     make(map[string]*prometheus.GaugeVec),
			customHistograms: make(map[string]*prometheus.HistogramVec),
		}
		
		// Register the metrics with the default registry for /metrics handler
		prometheus.MustRegister(instance.TasksCreated)
		prometheus.MustRegister(instance.TasksScheduled)
		prometheus.MustRegister(instance.TasksCompleted)
		prometheus.MustRegister(instance.TasksFailed)
		prometheus.MustRegister(instance.TasksRetried)
		prometheus.MustRegister(instance.TasksInDeadLetter)
		prometheus.MustRegister(instance.TaskExecutionTime)
		prometheus.MustRegister(instance.TaskExecutionsByName)
		prometheus.MustRegister(instance.TaskDurationByName)
		prometheus.MustRegister(instance.QueueSize)
		prometheus.MustRegister(instance.QueueLatency)
		prometheus.MustRegister(instance.WorkerCount)
		prometheus.MustRegister(instance.WorkerUtilization)
		prometheus.MustRegister(instance.WorkerTasksProcessed)
		prometheus.MustRegister(instance.APIRequests)
		prometheus.MustRegister(instance.APIRequestDuration)
	})
	
	return instance
}

// RecordTaskExecution records metrics for a task execution
func (m *Metrics) RecordTaskExecution(taskName, status string, duration time.Duration) {
	// Update total task counts
	switch status {
	case "completed":
		m.TasksCompleted.Inc()
	case "failed":
		m.TasksFailed.Inc()
	case "retried":
		m.TasksRetried.Inc()
	case "dead_letter":
		m.TasksInDeadLetter.Inc()
	}
	
	// Record execution time
	m.TaskExecutionTime.Observe(duration.Seconds())
	
	// Record by task name
	m.TaskExecutionsByName.WithLabelValues(taskName, status).Inc()
	m.TaskDurationByName.WithLabelValues(taskName).Observe(duration.Seconds())
}

// UpdateQueueSize updates the queue size metric
func (m *Metrics) UpdateQueueSize(queueName string, priority string, size float64) {
	m.QueueSize.WithLabelValues(queueName, priority).Set(size)
}

// RecordQueueLatency records how long a task waited in the queue
func (m *Metrics) RecordQueueLatency(latency time.Duration) {
	m.QueueLatency.Observe(latency.Seconds())
}

// UpdateWorkerCount updates the current worker count
func (m *Metrics) UpdateWorkerCount(count int) {
	m.WorkerCount.Set(float64(count))
}

// UpdateWorkerUtilization updates a worker's utilization percentage
func (m *Metrics) UpdateWorkerUtilization(workerID string, utilization float64) {
	m.WorkerUtilization.WithLabelValues(workerID).Set(utilization)
}

// RecordWorkerTask records a task processed by a worker
func (m *Metrics) RecordWorkerTask(workerID, status string) {
	m.WorkerTasksProcessed.WithLabelValues(workerID, status).Inc()
}

// RecordAPIRequest records an API request
func (m *Metrics) RecordAPIRequest(endpoint, method, status string, duration time.Duration) {
	m.APIRequests.WithLabelValues(endpoint, method, status).Inc()
	m.APIRequestDuration.WithLabelValues(endpoint, method).Observe(duration.Seconds())
}

// RegisterCustomCounter registers a custom counter metric
func (m *Metrics) RegisterCustomCounter(name, help string, labelNames []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, exists := m.customCounters[name]; !exists {
		m.customCounters[name] = registerer.NewCounterVec(
			prometheus.CounterOpts{
				Name: name,
				Help: help,
			},
			labelNames,
		)
	}
}

// IncrementCustomCounter increments a custom counter
func (m *Metrics) IncrementCustomCounter(name string, labelValues ...string) {
	m.mu.RLock()
	counter, exists := m.customCounters[name]
	m.mu.RUnlock()
	
	if exists {
		counter.WithLabelValues(labelValues...).Inc()
	}
}

// RegisterCustomGauge registers a custom gauge metric
func (m *Metrics) RegisterCustomGauge(name, help string, labelNames []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, exists := m.customGauges[name]; !exists {
		m.customGauges[name] = registerer.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: name,
				Help: help,
			},
			labelNames,
		)
	}
}

// SetCustomGauge sets a custom gauge value
func (m *Metrics) SetCustomGauge(name string, value float64, labelValues ...string) {
	m.mu.RLock()
	gauge, exists := m.customGauges[name]
	m.mu.RUnlock()
	
	if exists {
		gauge.WithLabelValues(labelValues...).Set(value)
	}
}

// RegisterCustomHistogram registers a custom histogram metric
func (m *Metrics) RegisterCustomHistogram(name, help string, buckets []float64, labelNames []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, exists := m.customHistograms[name]; !exists {
		m.customHistograms[name] = registerer.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: name,
				Help: help,
				Buckets: buckets,
			},
			labelNames,
		)
	}
}

// ObserveCustomHistogram records a value in a custom histogram
func (m *Metrics) ObserveCustomHistogram(name string, value float64, labelValues ...string) {
	m.mu.RLock()
	histogram, exists := m.customHistograms[name]
	m.mu.RUnlock()
	
	if exists {
		histogram.WithLabelValues(labelValues...).Observe(value)
	}
}

// Handler returns an HTTP handler for exposing metrics
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
}

// APIMiddleware returns middleware for recording API metrics
func (m *Metrics) APIMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Create a response writer that captures the status code
		statusRecorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		
		// Call the next handler
		next.ServeHTTP(statusRecorder, r)
		
		// Record metrics
		duration := time.Since(start)
		statusStr := http.StatusText(statusRecorder.status)
		m.RecordAPIRequest(r.URL.Path, r.Method, statusStr, duration)
	})
}

// statusRecorder is a wrapper for http.ResponseWriter that captures the status code
type statusRecorder struct {
	http.ResponseWriter
	status int
}

// WriteHeader captures the status code
func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
} 