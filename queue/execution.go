package queue

import (
	"sync"

	"github.com/go-logr/logr"
)

var (
	execution *ExecutionQueue = newExecutionQueue()
	mutex                     = sync.Mutex{}
)

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
	GetName() string
}

// ExecutionQueue handles the queues for each different repository it finds.
type ExecutionQueue struct {
	queues map[string]*PriorityQueue
}

func newExecutionQueue() *ExecutionQueue {
	queues := make(map[string]*PriorityQueue)
	return &ExecutionQueue{queues: queues}
}

func (eq *ExecutionQueue) Add(repository string, exec Executor) {
	mutex.Lock()
	defer mutex.Unlock()
	if _, exists := eq.queues[repository]; !exists {
		eq.queues[repository] = newPriorityQueue()
	}
	eq.queues[repository].add(exec)
}

func (eq *ExecutionQueue) Get(repository string) Executor {
	mutex.Lock()
	defer mutex.Unlock()
	entry := eq.queues[repository].get()
	if eq.queues[repository].Len() == 0 {
		delete(eq.queues, repository)
	}
	return entry
}

func (eq *ExecutionQueue) GetRawMap() map[string]*PriorityQueue {
	return eq.queues
}

func GetExecQueue() *ExecutionQueue {
	return execution
}

// GetRepositories returns a list of all repositories that are currently
// handled by the queue.
func GetRepositories() []string {
	repositories := make([]string, len(execution.queues))
	for repository := range execution.queues {
		repositories = append(repositories, repository)
	}
	return repositories
}
