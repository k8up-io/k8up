package v1alpha1

import (
	backup "github.com/vshn/k8up/apis/backup"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	version = "v1alpha1"
)

// Backup constants
const (
	BackupKind   = "Backup"
	BackupName   = "backup"
	BackupPlural = "backups"

	RestoreKind   = "Restore"
	RestoreName   = "restore"
	RestorePlural = "restores"

	ArchiveKind   = "Archive"
	ArchiveName   = "archive"
	ArchivePlural = "archives"

	ScheduleKind   = "Schedule"
	ScheduleName   = "schedule"
	SchedulePlural = "schedules"

	CheckKind   = "Check"
	CheckName   = "check"
	CheckPlural = "checks"

	PruneKind   = "Prune"
	PruneName   = "prune"
	PrunePlural = "prunes"

	PreBackupPodKind   = "PreBackupPod"
	PreBackupPodName   = "prebackuppod"
	PreBackupPodPlural = "prebackuppods"

	NamespaceScope = apiextensionsv1beta1.NamespaceScoped
	ClusterScope   = apiextensionsv1beta1.ClusterScoped
)

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = schema.GroupVersion{Group: backup.GroupName, Version: version}

// Kind takes an unqualified kind and returns back a Group qualified GroupKind
func Kind(kind string) schema.GroupKind {
	return VersionKind(kind).GroupKind()
}

// VersionKind takes an unqualified kind and returns back a Group qualified GroupVersionKind
func VersionKind(kind string) schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(kind)
}

// Resource takes an unqualified resource and returns a Group qualified GroupResource
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

var (
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme   = SchemeBuilder.AddToScheme
)

// Adds the list of known types to Scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&Backup{},
		&BackupList{},
		&Restore{},
		&RestoreList{},
		&Archive{},
		&ArchiveList{},
		&Schedule{},
		&ScheduleList{},
		&Check{},
		&CheckList{},
		&Prune{},
		&PruneList{},
		&PreBackupPod{},
		&PreBackupPodList{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
