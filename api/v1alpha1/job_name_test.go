package v1alpha1_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vshn/k8up/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestJobName(t *testing.T) {
	subjects := map[string]v1alpha1.JobObject{
		"archive-job-name":  &v1alpha1.Archive{ObjectMeta: metav1.ObjectMeta{Name: "job-name"}},
		"backup-job-name":   &v1alpha1.Backup{ObjectMeta: metav1.ObjectMeta{Name: "job-name"}},
		"check-job-name":    &v1alpha1.Check{ObjectMeta: metav1.ObjectMeta{Name: "job-name"}},
		"prune-job-name":    &v1alpha1.Prune{ObjectMeta: metav1.ObjectMeta{Name: "job-name"}},
		"restore-job-name":  &v1alpha1.Restore{ObjectMeta: metav1.ObjectMeta{Name: "job-name"}},
		"schedule-job-name": &v1alpha1.Schedule{ObjectMeta: metav1.ObjectMeta{Name: "job-name"}},
	}

	for expectedName, subject := range subjects {
		t.Run(expectedName, func(t *testing.T) {
			assert.Equal(t, expectedName, subject.GetJobName())
		})
	}
}
