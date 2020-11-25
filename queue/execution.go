// queue contains a priority queue for each observed repository. If an exclusive
// executor gets add it will have the highest prio and thus handled before all
// non exclusive executors.

package queue

import (
	"sync"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	execution *ExecutionQueue = newExecutionQueue()

	promLabels = []string{
		"namespace",
		"jobType",
	}

	queueGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "k8up_jobs_queued_gauge",
		Help: "How many jobs are currently queued up",
	}, promLabels)
)

// Executor defines an interface for the execution queue.
type Executor interface {
	// Triggers the actual job
	Execute() error
	// Exclusive will return true, if the job is an
	// exclusive job and can't be run together with
	// other jobs on the same repository.
	Exclusive() bool
	// Logger returns the logger in the job's context so we can
	// Associate the logs with the actual job.
	Logger() logr.Logger
	// GetType() returns the type of the CDR that the job will execute
	GetType() string
	// GetNamespace() returns the namespace of the CDR that the job will execute
	GetNamespace() string
	// GetName() string
	GetRepository() string
	// TODO: ability to mark job as skipped && metric for that
}

// ExecutionQueue handles the queues for each different repository it finds.
type ExecutionQueue struct {
	mutex  sync.Mutex
	queues map[string]*priorityQueue
}

func init() {
	// Register custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(queueGauge)
}

func newExecutionQueue() *ExecutionQueue {
	queues := make(map[string]*priorityQueue)
	return &ExecutionQueue{queues: queues}
}

// Add adds an Executor to the queue.
func (eq *ExecutionQueue) Add(exec Executor) {
	eq.mutex.Lock()
	defer eq.mutex.Unlock()
	repository := exec.GetRepository()
	if _, exists := eq.queues[repository]; !exists {
		eq.queues[repository] = newPriorityQueue()
	}
	eq.queues[repository].add(exec)
	eq.incQueueGauge(exec.GetNamespace(), exec.GetType())
}

// Get returns and removes and executor from the given repository. If the
// queue for the repository is empty it will be removed completely.
func (eq *ExecutionQueue) Get(repository string) Executor {
	eq.mutex.Lock()
	defer eq.mutex.Unlock()
	entry := eq.queues[repository].get()
	if eq.queues[repository].Len() == 0 {
		delete(eq.queues, repository)
		eq.decQueueGauge(entry.GetNamespace(), entry.GetType())
	}
	return entry
}

// IsEmpty checks if the queue for the given repository is empty.
func (eq *ExecutionQueue) IsEmpty(repository string) bool {
	eq.mutex.Lock()
	defer eq.mutex.Unlock()
	repoQueue := eq.queues[repository]
	return repoQueue == nil || repoQueue.Len() == 0
}

// GetExecQueue will return the queue singleton.
func GetExecQueue() *ExecutionQueue {
	return execution
}

// GetRepositories returns a list of all repositories that are currently
// handled by the queue.
func (eq *ExecutionQueue) GetRepositories() []string {
	eq.mutex.Lock()
	defer eq.mutex.Unlock()
	repositories := make([]string, 0, len(execution.queues))
	for repository := range execution.queues {
		repositories = append(repositories, repository)
	}
	return repositories
}

func (eq *ExecutionQueue) incQueueGauge(namespace, jobType string) {
	queueGauge.WithLabelValues(namespace, jobType).Inc()
}

func (eq *ExecutionQueue) decQueueGauge(namespace, jobType string) {
	queueGauge.WithLabelValues(namespace, jobType).Dec()
}
