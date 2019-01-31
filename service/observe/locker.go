package observe

import (
	"sync"
	"time"
)

const (
	RestoreName JobName = "Restore"
	BackupName  JobName = "Backup"
	PruneName   JobName = "Prune"
	CheckName   JobName = "Check"
)

// JobName is a type for the various job names.
type JobName string

// JobType Contains the name, timestamp and sequence of each job in the locks.
// The timestamp is used for reporting only.
type JobType struct {
	Name      JobName
	Timestamp time.Time
	sequence  int64
	Backend   string
}

// sempahoreMap is an interface to make the sync map type safe.
type semaphoreMap interface {
	load(name string) semaphore
	add(name string, sem semaphore)
	remove(name string)
}

// concreteSemaphoreMap implements semaphoreMap
type concreteSemaphoreMap struct {
	mutexMap map[string]semaphore
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
	WaitForRun(backend string, jobs []JobName)
	Increment(backend string, job JobName) JobType
	Decrement(job JobType)
	Remove(backend string)
	IsLocked(backend string, jobs JobName) bool
}

// concreteLocker implements the Locker interface
type concreteLocker struct {
	semaphores semaphoreMap
	mutex      sync.Mutex
	sequence   int64
}

// semaphore holds a counter for each job type
type semaphore struct {
	backup  []JobType
	prune   []JobType
	restore []JobType
	check   []JobType
}

func newLocker() *concreteLocker {
	locker := &concreteLocker{
		semaphores: &concreteSemaphoreMap{
			mutexMap: make(map[string]semaphore),
		},
	}

	return locker
}

func (s *concreteSemaphoreMap) load(name string) semaphore {
	if sem, ok := s.mutexMap[name]; ok {
		return sem
	}
	return semaphore{
		backup:  []JobType{},
		prune:   []JobType{},
		restore: []JobType{},
		check:   []JobType{},
	}
}

func (s *concreteSemaphoreMap) add(name string, sem semaphore) {
	s.mutexMap[name] = sem
}

func (s *concreteSemaphoreMap) remove(name string) {
	delete(s.mutexMap, name)
}

// IsLocked will return true if the semaphore for the given jobType and backend
// isn't zero. It does not consider the timestamp.
func (c *concreteLocker) IsLocked(backend string, job JobName) bool {
	sem := c.semaphores.load(backend)
	c.mutex.Lock()
	defer c.mutex.Unlock()
	switch job {
	case PruneName:
		return len(sem.prune) > 0
	case RestoreName:
		return len(sem.restore) > 0
	case BackupName:
		return len(sem.backup) > 0
	case CheckName:
		return len(sem.check) > 0
	default:
		return false
	}
}

func (c *concreteLocker) isLockedBefore(sequence int64, backend string, job JobName) bool {
	if c.IsLocked(backend, job) {
		c.mutex.Lock()
		defer c.mutex.Unlock()
		sem := c.semaphores.load(backend)
		switch job {
		case PruneName:
			return c.isBefore(sem.prune, sequence)
		case RestoreName:
			return c.isBefore(sem.restore, sequence)
		case BackupName:
			return c.isBefore(sem.backup, sequence)
		case CheckName:
			return c.isBefore(sem.check, sequence)
		default:
			return false
		}
	}
	return false
}

func (c *concreteLocker) isBefore(registered []JobType, sequence int64) bool {
	for _, reg := range registered {
		if reg.sequence <= sequence {
			return true
		}
	}
	return false
}

// Increment will increment the semaphore for the backend and job type and
// returns an object containing information about the lock. It will be needed
// to decrement the semaphore again.
func (c *concreteLocker) Increment(backend string, jobName JobName) JobType {
	job := JobType{
		Name:      jobName,
		sequence:  c.incrementSequence(),
		Timestamp: time.Now(),
		Backend:   backend,
	}
	c.incOrDec(backend, job, true)
	return job
}

// Decrement will decrement the semaphore for the backend and job type
func (c *concreteLocker) Decrement(job JobType) {
	c.incOrDec(job.Backend, job, false)
}

// incOrDec will either increment or decrement the semaphore depenending on the
// value of inc.
func (c *concreteLocker) incOrDec(backend string, job JobType, inc bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	sem := c.semaphores.load(backend)
	switch job.Name {
	case PruneName:
		if inc {
			sem.prune = append(sem.prune, job)
		} else {
			sem.prune = c.removeSequence(sem.prune, job)
		}
	case BackupName:
		if inc {
			sem.backup = append(sem.backup, job)
		} else {
			sem.backup = c.removeSequence(sem.backup, job)
		}
	case RestoreName:
		if inc {
			sem.restore = append(sem.restore, job)
		} else {
			sem.restore = c.removeSequence(sem.restore, job)
		}
	case CheckName:
		if inc {
			sem.check = append(sem.check, job)
		} else {
			sem.check = c.removeSequence(sem.check, job)
		}
	}
	c.semaphores.add(backend, sem)
}

func (c *concreteLocker) removeSequence(registered []JobType, job JobType) []JobType {
	for i, registeredJob := range registered {
		if registeredJob.sequence == job.sequence {
			registered = append(registered[:i], registered[i+1:]...)
		}
	}
	return registered
}

// Remove removes the backend from the locker
func (c *concreteLocker) Remove(backend string) {
	c.semaphores.remove(backend)
}

// WaitForRun will loop through all job types passed for the given Repository.
// As soon as no all of these job types with a lower sequence than the current
// one have finished, it will return.
func (c *concreteLocker) WaitForRun(backend string, jobs []JobName) {

	waiting := true
	for waiting {
		for i := range jobs {

			if c.isLockedBefore(c.sequence, backend, jobs[i]) {
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

func (c *concreteLocker) incrementSequence() int64 {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.sequence++
	return c.sequence
}
