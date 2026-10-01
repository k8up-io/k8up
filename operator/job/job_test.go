package job

import (
	"context"
	"testing"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/stretchr/testify/assert"
	apiMeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestSha256Hash(t *testing.T) {
	tests := map[string]struct {
		givenString  string
		goldenString string
	}{
		"EmptyString": {
			givenString:  "",
			goldenString: "",
		},
		"RepositoryS3": {
			givenString:  "s3:endpoint/bucket",
			goldenString: "03ae9513ea3ba4b6d7289c427503e85cb28c11da210f442f89a07093c22af8a",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			actual := Sha256Hash(tc.givenString)
			assert.Equal(t, tc.goldenString, actual)
			assert.LessOrEqual(t, len(actual), 63)
		})
	}
}

func TestSetStartedPersistsStatus(t *testing.T) {
	ctx := context.Background()
	fakeClient, backup := newFakeBackupClient(t)
	config := NewConfig(fakeClient, backup, "")

	config.SetStarted(ctx, "backup job created")

	actual := &k8upv1.Backup{}
	err := fakeClient.Get(ctx, types.NamespacedName{Namespace: backup.Namespace, Name: backup.Name}, actual)
	if !assert.NoError(t, err) {
		return
	}
	assert.True(t, actual.Status.Started)

	ready := apiMeta.FindStatusCondition(actual.Status.Conditions, k8upv1.ConditionReady.String())
	if assert.NotNil(t, ready) {
		assert.Equal(t, metav1.ConditionTrue, ready.Status)
		assert.Equal(t, k8upv1.ReasonReady.String(), ready.Reason)
		assert.Equal(t, "backup job created", ready.Message)
	}

	progressing := apiMeta.FindStatusCondition(actual.Status.Conditions, k8upv1.ConditionProgressing.String())
	if assert.NotNil(t, progressing) {
		assert.Equal(t, metav1.ConditionTrue, progressing.Status)
		assert.Equal(t, k8upv1.ReasonStarted.String(), progressing.Reason)
	}
}

func TestSetFinishedPersistsStatus(t *testing.T) {
	ctx := context.Background()
	fakeClient, backup := newFakeBackupClient(t)
	status := backup.GetStatus()
	status.SetStarted("backup job created")
	backup.SetStatus(status)
	assert.NoError(t, fakeClient.Status().Update(ctx, backup))
	config := NewConfig(fakeClient, backup, "")

	config.SetFinished(ctx, backup.Namespace, "backup-job")

	actual := &k8upv1.Backup{}
	err := fakeClient.Get(ctx, types.NamespacedName{Namespace: backup.Namespace, Name: backup.Name}, actual)
	if !assert.NoError(t, err) {
		return
	}
	assert.True(t, actual.Status.Finished)
	assert.Nil(t, apiMeta.FindStatusCondition(actual.Status.Conditions, k8upv1.ConditionReady.String()))

	progressing := apiMeta.FindStatusCondition(actual.Status.Conditions, k8upv1.ConditionProgressing.String())
	if assert.NotNil(t, progressing) {
		assert.Equal(t, metav1.ConditionFalse, progressing.Status)
		assert.Equal(t, k8upv1.ReasonFinished.String(), progressing.Reason)
		assert.Equal(t, "the Job 'default/backup-job' ended", progressing.Message)
	}
}

func newFakeBackupClient(t *testing.T) (client.Client, *k8upv1.Backup) {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := k8upv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	backup := &k8upv1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "backup",
			Namespace: "default",
		},
	}
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&k8upv1.Backup{}).
		WithObjects(backup).
		Build()

	return fakeClient, backup
}
