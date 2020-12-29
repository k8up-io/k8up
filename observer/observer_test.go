package observer

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/vshn/k8up/api/v1alpha1"
)

func TestObserver_IsConcurrentJobsLimitReached(t *testing.T) {

	o := &Observer{
		events:       make(chan ObservableJob, 10),
		observedJobs: make(map[string]ObservableJob),
		log:          ctrl.Log.WithName("observer-test"),
		mutex:        sync.Mutex{},
	}

	// empty observedJobs
	isLimitReached := o.IsConcurrentJobsLimitReached(v1alpha1.BackupType, 1)
	assert.False(t, isLimitReached)

	oj := ObservableJob{
		Job: &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name: "some-name",
			},
		},
		JobType:    v1alpha1.BackupType,
		Event:      Running,
		Exclusive:  true,
		Repository: "some-repo",
		callbacks:  []ObservableJobCallback{},
	}

	ojs := make(map[string]ObservableJob)
	ojs["some-name"] = oj
	o.observedJobs = ojs

	// 1 observedJob and limit = 1
	isLimitReached = o.IsConcurrentJobsLimitReached(v1alpha1.BackupType, 1)
	assert.True(t, isLimitReached)

	// another job type not present in observableJobs
	isLimitReached = o.IsConcurrentJobsLimitReached(v1alpha1.ArchiveType, 1)
	assert.False(t, isLimitReached)

	// limit is 0
	isLimitReached = o.IsConcurrentJobsLimitReached(v1alpha1.ArchiveType, 0)
	assert.False(t, isLimitReached)
}

func TestObserver_AreAllCallbacksInvoked(t *testing.T) {
	for expectedInvocations := 0; expectedInvocations < 3; expectedInvocations++ {
		t.Run(fmt.Sprintf("Given%dCallbacks_WhenHandleEvent_ThenExpect%[1]vInvocations", expectedInvocations), func(t *testing.T) {
			// Given
			var actualInvocations = new(int)
			ojc := func(job ObservableJob) {
				*actualInvocations++
			}

			callbacks := make([]ObservableJobCallback, 0)
			for i := 0; i < expectedInvocations; i++ {
				callbacks = append(callbacks, ojc)
			}

			oj := ObservableJob{
				Job: &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "some-name",
						Namespace: "default",
					},
				},
				JobType:    v1alpha1.BackupType,
				Event:      Suceeded,
				Exclusive:  true,
				Repository: "some-repo",
				callbacks:  callbacks,
			}

			observedJobs := map[string]ObservableJob{
				"default/some-name": oj,
			}

			o := &Observer{
				events:       make(chan ObservableJob, 10),
				observedJobs: observedJobs,
				log:          ctrl.Log.WithName("observer-test"),
				mutex:        sync.Mutex{},
			}

			// When
			o.handleEvent(oj)

			// Then
			assert.Equal(t, expectedInvocations, *actualInvocations)
		})
	}
}

func TestObserver_AreOnlyExpectedCallbacksInvoked(t *testing.T) {
	tests := map[string]struct {
		givenEventType   EventType
		expectInvocation bool
	}{
		"GivenStatusSucceeded_WhenHandleEvent_ThenExpectInvocation": {
			givenEventType:   Suceeded,
			expectInvocation: true,
		},
		"GivenStatusFailed_WhenHandleEvent_ThenExpectInvocation": {
			givenEventType:   Failed,
			expectInvocation: true,
		},
		"GivenStatusDelete_WhenHandleEvent_ThenExpectInvocation": {
			givenEventType:   Delete,
			expectInvocation: true,
		},
		"GivenStatusRunning_WhenHandleEvent_ThenExpectNoInvocation": {
			givenEventType:   Running,
			expectInvocation: false,
		},
		"GivenStatusCreate_WhenHandleEvent_ThenExpectNoInvocation": {
			givenEventType:   Create,
			expectInvocation: false,
		},
		"GivenStatusUpdate_WhenHandleEvent_ThenExpectNoInvocation": {
			givenEventType:   Update,
			expectInvocation: false,
		},
	}
	for testCase, testParameter := range tests {
		t.Run(testCase, func(t *testing.T) {
			// Given
			var actualInvocations = new(int)
			ojc := func(job ObservableJob) {
				*actualInvocations++
			}

			oj := ObservableJob{
				Job: &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "some-name",
						Namespace: "default",
					},
				},
				JobType:    v1alpha1.BackupType,
				Event:      testParameter.givenEventType,
				Exclusive:  true,
				Repository: "some-repo",
				callbacks:  []ObservableJobCallback{ojc},
			}

			observedJobs := map[string]ObservableJob{
				"default/some-name": oj,
			}

			o := &Observer{
				events:       make(chan ObservableJob, 10),
				observedJobs: observedJobs,
				log:          ctrl.Log.WithName("observer-test"),
				mutex:        sync.Mutex{},
			}

			// When
			o.handleEvent(oj)

			// Then
			assert.Equal(t, testParameter.expectInvocation, *actualInvocations == 1)
		})
	}
}
