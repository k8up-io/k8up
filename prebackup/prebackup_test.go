package prebackup

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	k8upv1alpha1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/job"
)

func TestPreBackup_getPodTemplates(t *testing.T) {
	ns := "test-namespace"
	backupObj := &k8upv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-backup",
			Namespace: ns,
		},
	}
	tests := map[string]struct {
		initFakeObjects        []runtime.Object
		expectPodTemplateNames []string
	}{
		"GivenPrebackupObjectInDifferentNamespaces_WhenListingPrebackupTemplates_ThenIgnoreTemplatesFromOtherNamespaces": {
			initFakeObjects: []runtime.Object{
				backupObj,
				createPrebackupPod(ns, "backup-pod"),
				createPrebackupPod("foreign-namespace", "foreign-pod"),
			},
			expectPodTemplateNames: []string{"backup-pod"},
		},
	}
	scheme := runtime.NewScheme()
	require.NoError(t, k8upv1alpha1.SchemeBuilder.AddToScheme(scheme))

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			fakeClient := fake.NewFakeClientWithScheme(scheme, tt.initFakeObjects...)
			p := &PreBackup{
				Config: job.Config{
					Client: fakeClient,
					Obj:    backupObj,
				},
			}
			p.Config.Client = fakeClient
			list, err := p.getPodTemplates()
			assert.NoError(t, err)
			assert.Len(t, list.Items, len(tt.expectPodTemplateNames))
			for _, template := range list.Items {
				assert.Contains(t, tt.expectPodTemplateNames, template.Name)
			}
		})
	}
}

func createPrebackupPod(ns, name string) *k8upv1alpha1.PreBackupPod {
	return &k8upv1alpha1.PreBackupPod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
	}
}
