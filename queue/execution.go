package queue

import (
	"sync"

	"github.com/go-logr/logr"
)

var (
	execution *ExecutionQueue = newExecutionQueue()
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
	GetRepository() string
	// TODO: ability to mark job as skipped && metric for that
}

// ExecutionQueue handles the queues for each different repository it finds.
type ExecutionQueue struct {
	mutex  sync.Mutex
	queues map[string]*priorityQueue
}

func newExecutionQueue() *ExecutionQueue {
	queues := make(map[string]*priorityQueue)
	return &ExecutionQueue{queues: queues}
}

func (eq *ExecutionQueue) Add(exec Executor) {
	eq.mutex.Lock()
	defer eq.mutex.Unlock()
	if _, exists := eq.queues[exec.GetRepository()]; !exists {
		eq.queues[exec.GetRepository()] = newPriorityQueue()
	}
	eq.queues[exec.GetRepository()].add(exec)
}

func (eq *ExecutionQueue) Get(repository string) Executor {
	eq.mutex.Lock()
	defer eq.mutex.Unlock()
	entry := eq.queues[repository].get()
	if eq.queues[repository].Len() == 0 {
		delete(eq.queues, repository)
	}
	return entry
}

// IsEmpty checks if the queue for the given repository is empty
func (eq *ExecutionQueue) IsEmpty(repository string) bool {
	eq.mutex.Lock()
	defer eq.mutex.Unlock()
	repoQueue := eq.queues[repository]
	return repoQueue == nil || repoQueue.Len() == 0
}

func GetExecQueue() *ExecutionQueue {
	return execution
}

// GetRepositories returns a list of all repositories that are currently
// handled by the queue.
func (eq *ExecutionQueue) GetRepositories() []string {
	eq.mutex.Lock()
	defer eq.mutex.Unlock()
	repositories := make([]string, len(execution.queues))
	for repository := range execution.queues {
		repositories = append(repositories, repository)
	}
	return repositories
}
