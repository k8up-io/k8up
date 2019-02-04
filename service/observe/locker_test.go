package observe

import (
	"sync"
	"testing"
	"time"
)

// The waitForRun tests take quite some time due to the sleeps. Use at least
// 60 seconds timeout here.

func Test_concreteLocker_WaitForRun(t *testing.T) {
	type fields struct {
		semaphores semaphoreMap
		mutex      sync.Mutex
	}
	type args struct {
		backend            string
		jobs               []JobName
		register           []JobName
		unlock             map[int]JobType
		jobIndexForWaiting int
	}
	tests := []struct {
		name          string
		allowedToFail bool
		unlockAfter   int
		fields        fields
		args          args
	}{
		{
			name:        "Block for 1 second",
			unlockAfter: 2,
			fields: fields{
				semaphores: &concreteSemaphoreMap{
					mutexMap: make(map[string]semaphore),
				},
				mutex: sync.Mutex{},
			},
			args: args{
				backend:  "test",
				jobs:     []JobName{BackupName},
				register: []JobName{BackupName},
				unlock: map[int]JobType{
					0: JobType{},
				},
				jobIndexForWaiting: 0,
			},
		},
		{
			name:        "Block all jobs",
			unlockAfter: 5,
			fields: fields{
				semaphores: &concreteSemaphoreMap{
					mutexMap: make(map[string]semaphore),
				},
				mutex: sync.Mutex{},
			},
			args: args{
				backend: "test",
				jobs: []JobName{
					BackupName,
					CheckName,
					RestoreName,
					PruneName,
				},
				register: []JobName{
					BackupName,
					CheckName,
					RestoreName,
					PruneName,
				},
				unlock: map[int]JobType{
					0: JobType{},
					1: JobType{},
					2: JobType{},
					3: JobType{},
				},
				jobIndexForWaiting: 3,
			},
		},
		{
			name:        "Register backup during prune",
			unlockAfter: 5,
			fields: fields{
				semaphores: &concreteSemaphoreMap{
					mutexMap: make(map[string]semaphore),
				},
				mutex: sync.Mutex{},
			},
			args: args{
				backend: "test",
				jobs: []JobName{
					BackupName,
				},
				register: []JobName{
					BackupName,
					PruneName,
					BackupName,
				},
				unlock: map[int]JobType{
					0: JobType{},
				},
				jobIndexForWaiting: 1,
			},
		},
		{
			name:        "Lock forever",
			unlockAfter: 1,
			fields: fields{
				semaphores: &concreteSemaphoreMap{
					mutexMap: make(map[string]semaphore),
				},
				mutex: sync.Mutex{},
			},
			args: args{
				backend: "test",
				jobs: []JobName{
					BackupName,
				},
				register: []JobName{
					BackupName,
					PruneName,
					BackupName,
				},
				unlock:             map[int]JobType{},
				jobIndexForWaiting: 1,
			},
			allowedToFail: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &concreteLocker{
				semaphores: tt.fields.semaphores,
				mutex:      tt.fields.mutex,
			}

			var jobToWaitfor JobType

			for i, job := range tt.args.register {
				newJob := c.Increment(tt.args.backend, job)
				if len(tt.args.unlock) >= i && !tt.allowedToFail {
					tt.args.unlock[i] = newJob
				}
				if i == tt.args.jobIndexForWaiting {
					jobToWaitfor = newJob
				}
			}

			finished := make(chan bool, 0)

			go func() {
				c.WaitForRun(jobToWaitfor.Backend, tt.args.jobs)
				finished <- true
			}()

			go func() {
				for i := 0; i < len(tt.args.unlock); i++ {
					time.Sleep(time.Second * time.Duration(tt.unlockAfter))
					c.Decrement(tt.args.unlock[i])
				}
			}()

			select {
			case <-time.After(time.Duration(tt.unlockAfter*2*len(tt.args.unlock)) * time.Second):
				if !tt.allowedToFail {
					t.Fail()
				}
			case <-finished:
				if tt.allowedToFail {
					t.Fail()
				}
				c.Remove(tt.args.backend)
			}
		})
	}
}
