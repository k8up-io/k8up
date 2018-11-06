package observe

import (
	"sync"
	"testing"
	"time"
)

func Test_concreteLocker_WaitForRun(t *testing.T) {
	type fields struct {
		semaphores semaphoreMap
		mutex      sync.Mutex
	}
	type args struct {
		backend string
		jobs    []JobType
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
			unlockAfter: 1,
			fields: fields{
				semaphores: &concreteSemaphoreMap{},
				mutex:      sync.Mutex{},
			},
			args: args{
				backend: "test",
				jobs:    []JobType{BackupType},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &concreteLocker{
				semaphores: tt.fields.semaphores,
				mutex:      tt.fields.mutex,
			}

			c.Increment(tt.args.backend, tt.args.jobs[0])

			finished := make(chan bool, 0)

			go func() {
				c.WaitForRun(tt.args.backend, tt.args.jobs)
				finished <- true
			}()

			go func() {
				time.Sleep(time.Second * time.Duration(tt.unlockAfter))
				c.Decrement(tt.args.backend, tt.args.jobs[0])
			}()

			select {
			case <-time.After(25 * time.Second):
				if !tt.allowedToFail {
					t.Fail()
				}
			case <-finished:
				// NOOP
			}
		})
	}
}
