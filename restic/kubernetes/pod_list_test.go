package kubernetes

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func annotatedPod(name string, uid types.UID, annotations map[string]string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   "ns",
			UID:         uid,
			Annotations: annotations,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "main", Image: "postgres"}},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
}

func TestListPods_FileName(t *testing.T) {
	k8cli := fake.NewClientBuilder().WithObjects(
		annotatedPod("db-a", "a", map[string]string{
			"k8up.io/backupcommand":  "pg_dump",
			"k8up.io/file-extension": ".sql",
			"k8up.io/file-name":      "db-a",
		}),
		annotatedPod("db-b", "b", map[string]string{
			"k8up.io/backupcommand":  "pg_dump",
			"k8up.io/file-extension": ".sql",
		}),
	).Build()

	lister := NewPodLister(context.Background(), k8cli, "k8up.io/backupcommand", "k8up.io/file-extension", "k8up.io/file-name", "k8up.io/backupcommand-container", "ns", nil, false, logr.Discard())
	pods, err := lister.ListPods()
	require.NoError(t, err)
	require.Len(t, pods, 2)

	byName := map[string]BackupPod{}
	for _, p := range pods {
		byName[p.PodName] = p
	}
	assert.Equal(t, "db-a", byName["db-a"].FileName)
	assert.Equal(t, ".sql", byName["db-a"].FileExtension)
	assert.Empty(t, byName["db-b"].FileName)
}

func TestBackupPod_StdinFilename(t *testing.T) {
	tests := map[string]struct {
		fileName string
		want     string
		wantErr  bool
	}{
		"Default":       {want: "/ns-main"},
		"Blank":         {fileName: "  ", want: "/ns-main"},
		"Custom":        {fileName: "orders-db", want: "/orders-db"},
		"Trimmed":       {fileName: " orders-db ", want: "/orders-db"},
		"Slash":         {fileName: "dumps/orders", wantErr: true},
		"LeadingSlash":  {fileName: "/orders", wantErr: true},
		"Dot":           {fileName: ".", wantErr: true},
		"ParentDirName": {fileName: "..", wantErr: true},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			pod := BackupPod{PodName: "db-a", ContainerName: "main", FileName: tt.fileName}
			got, err := pod.StdinFilename("ns")
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
