package observer

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vshn/k8up/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
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
		callback:   func() {},
	}

	ojs := make(map[string]ObservableJob)
	ojs["some-name"] = oj
	fmt.Println(len(ojs))

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
