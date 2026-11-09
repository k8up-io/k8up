package job

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
)

func TestConfig_SetStarted(t *testing.T) {
	ctx := context.Background()
	backup := &k8upv1.Backup{ObjectMeta: metav1.ObjectMeta{Name: "backup", Namespace: "default"}}
	c := newFakeClient(t, backup)

	config := NewConfig(c, backup, "")
	config.SetStarted(ctx, "the job '%v/%v' was created", "default", "backup-job")

	persisted := &k8upv1.Backup{}
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(backup), persisted))
	assert.True(t, persisted.Status.Started)
	assert.True(t, persisted.Status.HasStarted())

	ready := meta.FindStatusCondition(persisted.Status.Conditions, k8upv1.ConditionReady.String())
	require.NotNil(t, ready)
	assert.Equal(t, metav1.ConditionTrue, ready.Status)
	assert.Equal(t, k8upv1.ReasonReady.String(), ready.Reason)
	assert.Equal(t, "the job 'default/backup-job' was created", ready.Message)

	progressing := meta.FindStatusCondition(persisted.Status.Conditions, k8upv1.ConditionProgressing.String())
	require.NotNil(t, progressing)
	assert.Equal(t, metav1.ConditionTrue, progressing.Status)
	assert.Equal(t, k8upv1.ReasonStarted.String(), progressing.Reason)
}

func TestConfig_SetFinished(t *testing.T) {
	ctx := context.Background()
	backup := &k8upv1.Backup{ObjectMeta: metav1.ObjectMeta{Name: "backup", Namespace: "default"}}
	backup.Status.SetStarted("the job 'default/backup-job' was created")
	c := newFakeClient(t, backup)

	config := NewConfig(c, backup, "")
	config.SetFinished(ctx, "default", "backup-job")

	persisted := &k8upv1.Backup{}
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(backup), persisted))
	assert.True(t, persisted.Status.Finished)

	progressing := meta.FindStatusCondition(persisted.Status.Conditions, k8upv1.ConditionProgressing.String())
	require.NotNil(t, progressing)
	assert.Equal(t, metav1.ConditionFalse, progressing.Status)
	assert.Equal(t, k8upv1.ReasonFinished.String(), progressing.Reason)
	assert.Equal(t, "the Job 'default/backup-job' ended", progressing.Message)
}

func newFakeClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, k8upv1.AddToScheme(scheme))

	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(objs...).
		WithObjects(objs...).
		Build()
}
