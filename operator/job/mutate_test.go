package job

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
)

func TestMutateBatchJob_ActiveDeadlineSeconds(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, k8upv1.AddToScheme(scheme))

	podConfig := &k8upv1.PodConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "short-deadline", Namespace: "ns"},
		Spec: k8upv1.PodConfigSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
			ActiveDeadlineSeconds: ptr.To[int64](60),
		}}},
	}

	tests := map[string]struct {
		deadline     *int64
		podConfigRef string
		wantJob      *int64
		wantPod      *int64
	}{
		"NoDeadline": {},
		"DeadlineSetOnJobAndPod": {
			deadline: ptr.To[int64](500),
			wantJob:  ptr.To[int64](500),
			wantPod:  ptr.To[int64](500),
		},
		"PodConfigOverridesPodDeadline": {
			deadline:     ptr.To[int64](500),
			podConfigRef: "short-deadline",
			wantJob:      ptr.To[int64](500),
			wantPod:      ptr.To[int64](60),
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(podConfig).Build()
			backup := &k8upv1.Backup{ObjectMeta: metav1.ObjectMeta{Name: "backup", Namespace: "ns"}}
			backup.Spec.ActiveDeadlineSeconds = tc.deadline
			if tc.podConfigRef != "" {
				backup.Spec.PodConfigRef = &corev1.LocalObjectReference{Name: tc.podConfigRef}
			}
			batchJob := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "backup", Namespace: "ns"}}

			err := MutateBatchJob(context.Background(), batchJob, backup, NewConfig(c, backup, "s3:endpoint/bucket"), c)
			require.NoError(t, err)

			assert.Equal(t, ptrValue(tc.wantJob), ptrValue(batchJob.Spec.ActiveDeadlineSeconds), "job deadline")
			assert.Equal(t, ptrValue(tc.wantPod), ptrValue(batchJob.Spec.Template.Spec.ActiveDeadlineSeconds), "pod deadline")
			assert.Equal(t, ptrValue(tc.deadline), ptrValue(backup.Spec.ActiveDeadlineSeconds), "backup spec must not change")
		})
	}
}

// ptrValue makes failures print the deadline instead of a pointer address.
func ptrValue(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}

func TestMutateBatchJob_BackoffLimit(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, k8upv1.AddToScheme(scheme))
	c := fake.NewClientBuilder().WithScheme(scheme).Build()

	newBatchJob := func() *batchv1.Job {
		return &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "job", Namespace: "ns"}}
	}

	check := &k8upv1.Check{ObjectMeta: metav1.ObjectMeta{Name: "check", Namespace: "ns"}}
	restore := &k8upv1.Restore{ObjectMeta: metav1.ObjectMeta{Name: "restore", Namespace: "ns"}}

	tests := map[string]struct {
		obj         k8upv1.JobObject
		setLimit    func(limit *int32)
		limit       *int32
		wantBackoff *int32
	}{
		"CheckWithoutLimitKeepsKubernetesDefault": {
			obj:      check,
			setLimit: func(l *int32) { check.Spec.BackoffLimit = l },
		},
		"CheckWithLimit": {
			obj:         check,
			setLimit:    func(l *int32) { check.Spec.BackoffLimit = l },
			limit:       ptr.To[int32](1),
			wantBackoff: ptr.To[int32](1),
		},
		"RestoreWithLimit": {
			obj:         restore,
			setLimit:    func(l *int32) { restore.Spec.BackoffLimit = l },
			limit:       ptr.To[int32](2),
			wantBackoff: ptr.To[int32](2),
		},
		"RestoreWithZeroLimitDisablesRetries": {
			obj:         restore,
			setLimit:    func(l *int32) { restore.Spec.BackoffLimit = l },
			limit:       ptr.To[int32](0),
			wantBackoff: ptr.To[int32](0),
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			tc.setLimit(tc.limit)
			batchJob := newBatchJob()

			err := MutateBatchJob(context.Background(), batchJob, tc.obj, NewConfig(c, tc.obj, "s3:endpoint/bucket"), c)
			require.NoError(t, err)

			assert.Equal(t, tc.wantBackoff, batchJob.Spec.BackoffLimit, "job backoff limit")
			if tc.limit != nil {
				assert.NotSame(t, tc.limit, batchJob.Spec.BackoffLimit, "job must get its own copy")
			}
		})
	}
}
