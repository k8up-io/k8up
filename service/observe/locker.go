package observe

import (
	"sync"
)

const (
	RestoreType JobType = "Restore"
	BackupType  JobType = "Backup"
	PruneType   JobType = "Prune"
	CheckType   JobType = "Check"
)

type JobType string

type semaphoreMap interface {
	load(name string) semaphore
	add(name string, sem semaphore)
	remove(name string)
}

type concreteSemaphoreMap struct {
	sync.Map
}

// Locker handles the semaphores necessary to realize the locking according to
// the following rules:
// Backups can run in parallel, also restores can run during backups.
// Nothing else may run during prunes (exclusive lock), neither restores or backups.
// No prunes may run during a restore, but backups can.
// These rules are only applicable if the jobs run on the same backend!
//
// Due to the complexity of these rules they are handled in their service. The
// locker doesn't contain any of the logic above.
type Locker interface {
	IsLocked(backend string, job JobType) bool
	Increment(backend string, job JobType)
	Decrement(backend string, job JobType)
	Remove(backend string)
}

type ConcreteLocker struct {
	semaphores semaphoreMap
	mutex      sync.Mutex
}

type semaphore struct {
	backup  int
	prune   int
	restore int
	check   int
}

func newLocker() *ConcreteLocker {
	return &ConcreteLocker{
		semaphores: &concreteSemaphoreMap{},
	}
}

func (s *concreteSemaphoreMap) load(name string) semaphore {
	if obj, ok := s.Map.Load(name); ok {
		if sem, ok := obj.(semaphore); ok {
			return sem
		}
	}
	return semaphore{}
}

func (s *concreteSemaphoreMap) add(name string, sem semaphore) {
	s.Map.Store(name, sem)
}

func (s *concreteSemaphoreMap) remove(name string) {
	s.Map.Delete(name)
}

// IsLocked will determine if the backend is locked or not. Returns true if a job
// of JobType is still running.
func (c *ConcreteLocker) IsLocked(backend string, job JobType) bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	sem := c.semaphores.load(backend)
	switch job {
	case PruneType:
		return sem.prune > 0
	case RestoreType:
		return sem.restore > 0
	case BackupType:
		return sem.backup > 0
	case CheckType:
		return sem.check > 0
	default:
		return false
	}
}

// Increment will increment the semaphore for the backend and job type
func (c *ConcreteLocker) Increment(backend string, job JobType) {
	c.incOrDec(backend, job, true)
}

// Decrement will decrement the semaphore for the backend and job type
func (c *ConcreteLocker) Decrement(backend string, job JobType) {
	c.incOrDec(backend, job, false)
}

func (c *ConcreteLocker) incOrDec(backend string, job JobType, inc bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	sem := c.semaphores.load(backend)
	switch job {
	case PruneType:
		if inc {
			sem.prune++
		} else {
			sem.prune--
		}
	case BackupType:
		if inc {
			sem.backup++
		} else {
			sem.backup--
		}
	case RestoreType:
		if inc {
			sem.restore++
		} else {
			sem.restore--
		}
	case CheckType:
		if inc {
			sem.check++
		} else {
			sem.check--
		}
	}
	c.semaphores.add(backend, sem)
}

func (c *ConcreteLocker) Remove(backend string) {
	c.semaphores.remove(backend)
}
