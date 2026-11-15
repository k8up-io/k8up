package schedulecontroller

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/job"
	"github.com/k8up-io/k8up/v2/operator/scheduler"
)

func TestScheduleReconciler_Deprovision_DecreasesSchedulesGauge(t *testing.T) {
	const ns = "deprovision-ns"
	ctx := context.TODO()

	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, k8upv1.AddToScheme(scheme))

	fclient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}},
			&k8upv1.Schedule{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "schedule",
					Namespace:  ns,
					Finalizers: []string{k8upv1.ScheduleFinalizerName},
				},
				Spec: k8upv1.ScheduleSpec{
					Backup: &k8upv1.BackupSchedule{ScheduleCommon: &k8upv1.ScheduleCommon{Schedule: "0 0 1 1 *"}},
					Check:  &k8upv1.CheckSchedule{ScheduleCommon: &k8upv1.ScheduleCommon{Schedule: "0 0 1 1 *"}},
				},
			}).
		Build()

	schedule := &k8upv1.Schedule{}
	require.NoError(t, fclient.Get(ctx, client.ObjectKey{Namespace: ns, Name: "schedule"}, schedule))

	handler := ScheduleHandler{Config: job.Config{Client: fclient}, schedule: schedule}
	require.NoError(t, handler.createJobList(ctx, scheduler.GetScheduler()))
	assert.Equal(t, 2.0, schedulesGaugeValue(t, ns), "gauge after registering the schedule")

	r := &ScheduleReconciler{Kube: fclient}
	_, err := r.Deprovision(ctx, schedule)
	require.NoError(t, err)

	assert.False(t, scheduler.GetScheduler().HasSchedule(keyOf(schedule, k8upv1.BackupType)))
	assert.Equal(t, 0.0, schedulesGaugeValue(t, ns), "gauge after deprovisioning the schedule")
}

func schedulesGaugeValue(t *testing.T, namespace string) float64 {
	families, err := metrics.Registry.Gather()
	require.NoError(t, err)
	for _, family := range families {
		if family.GetName() != "k8up_schedules_gauge" {
			continue
		}
		for _, m := range family.GetMetric() {
			for _, label := range m.GetLabel() {
				if label.GetName() == "namespace" && label.GetValue() == namespace {
					return m.GetGauge().GetValue()
				}
			}
		}
	}
	return 0
}
