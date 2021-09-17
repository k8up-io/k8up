package v1_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	k8upv1 "github.com/k8up-io/k8up/api/v1"
)

func TestJobName(t *testing.T) {
	subjects := map[string]k8upv1.JobObject{
		"archive-job-name":  &k8upv1.Archive{ObjectMeta: metav1.ObjectMeta{Name: "job-name"}},
		"backup-job-name":   &k8upv1.Backup{ObjectMeta: metav1.ObjectMeta{Name: "job-name"}},
		"check-job-name":    &k8upv1.Check{ObjectMeta: metav1.ObjectMeta{Name: "job-name"}},
		"prune-job-name":    &k8upv1.Prune{ObjectMeta: metav1.ObjectMeta{Name: "job-name"}},
		"restore-job-name":  &k8upv1.Restore{ObjectMeta: metav1.ObjectMeta{Name: "job-name"}},
		"schedule-job-name": &k8upv1.Schedule{ObjectMeta: metav1.ObjectMeta{Name: "job-name"}},
	}

	for expectedName, subject := range subjects {
		t.Run(expectedName, func(t *testing.T) {
			assert.Equal(t, expectedName, subject.GetJobName())
		})
	}
}
