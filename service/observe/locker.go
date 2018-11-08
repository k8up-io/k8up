package observe

import (
	"sync"
	"time"
)

const (
	RestoreType JobType = "Restore"
	BackupType  JobType = "Backup"
	PruneType   JobType = "Prune"
	CheckType   JobType = "Check"
)

type JobType string

// sempahoreMap is an interface to make the sync map type safe.
type semaphoreMap interface {
	load(name string) semaphore
	add(name string, sem semaphore)
	remove(name string)
}

// concreteSemaphoreMap implements semaphoreMap
type concreteSemaphoreMap struct {
	sync.Map
}

// Locker handles the semaphores necessary to realize the locking according to
// the following rules:
// Backups can run in parallel, also restores can run during backups.
// Nothing else may run during prunes (exclusive lock), neither restores or backups.
// These rules are only applicable if the jobs run on the same backend!
//
// Due to the complexity of these rules they are handled in their service. The
// locker doesn't contain any of the logic above.
type Locker interface {
	WaitForRun(backend string, jobs []JobType)
	Increment(backend string, job JobType)
	Decrement(backend string, job JobType)
	Remove(backend string)
	IsLocked(backend string, jobs JobType) bool
}

// concreteLocker implements the Locker interface
type concreteLocker struct {
	semaphores semaphoreMap
	mutex      sync.Mutex
}

// semaphore holds a counter for each job type
type semaphore struct {
	backup  int
	prune   int
	restore int
	check   int
}

func newLocker() *concreteLocker {
	locker := &concreteLocker{
		semaphores: &concreteSemaphoreMap{},
	}

	return locker
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

// IsLocked will return true if the semaphore for the given jobType and backend
// isn't zero.
func (c *concreteLocker) IsLocked(backend string, job JobType) bool {
	sem := c.semaphores.load(backend)
	c.mutex.Lock()
	defer c.mutex.Unlock()
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
func (c *concreteLocker) Increment(backend string, job JobType) {
	c.incOrDec(backend, job, true)
}

// Decrement will decrement the semaphore for the backend and job type
func (c *concreteLocker) Decrement(backend string, job JobType) {
	c.incOrDec(backend, job, false)
}

// incOrDec will either increment or decrement the semaphore depenending on the
// value of inc.
func (c *concreteLocker) incOrDec(backend string, job JobType, inc bool) {
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

// Remove removes the backend from the locker
func (c *concreteLocker) Remove(backend string) {
	c.semaphores.remove(backend)
}

// WaitForRun will loop through all job types passed for the given Repository.
// As soon as no more jobs are running it will return.
func (c *concreteLocker) WaitForRun(backend string, jobs []JobType) {

	waiting := true
	for waiting {
		for i := range jobs {

			if c.IsLocked(backend, jobs[i]) {
				// at least one job is still running so we set waiting true and
				// break the loop so we don't override it again.
				waiting = true
				break
			} else {
				waiting = false
			}

		}
		if !waiting {
			return
		}
		// TODO: would be great if we have something here that would
		// block as long as there are no changes in the semaphores, to avoid
		// unnnecessary pollings every second. But this might prove rather
		// complex todo.
		time.Sleep(time.Second * 1)
	}
}
