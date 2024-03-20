package cfg

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	RestoreS3EndpointEnvName        = "RESTORE_S3ENDPOINT"
	RestoreS3AccessKeyIDEnvName     = "RESTORE_ACCESSKEYID"
	RestoreS3SecretAccessKeyEnvName = "RESTORE_SECRETACCESSKEY"

	ResticRepositoryEnvName = "RESTIC_REPOSITORY"
	ResticPasswordEnvName   = "RESTIC_PASSWORD"
	ResticOptionsEnvName    = "RESTIC_OPTIONS"

	AwsAccessKeyIDEnvName     = "AWS_ACCESS_KEY_ID"
	AwsSecretAccessKeyEnvName = "AWS_SECRET_ACCESS_KEY"

	AzureAccountEnvName    = "AZURE_ACCOUNT_NAME"
	AzureAccountKeyEnvName = "AZURE_ACCOUNT_KEY"

	GcsProjectIDEnvName   = "GOOGLE_PROJECT_ID"
	GcsAccessTokenEnvName = "GOOGLE_ACCESS_TOKEN"

	B2AccountIDEnvName  = "B2_ACCOUNT_ID"
	B2AccountKeyEnvName = "B2_ACCOUNT_KEY"

	RestUserEnvName     = "USER"
	RestPasswordEnvName = "PASSWORD"
)

var (
	// Config contains the values of the user-provided configuration of the operator module,
	// combined with the default values as defined in operator.Command.
	Config = &Configuration{}
)

// Configuration holds a strongly-typed tree of the configuration
type Configuration struct {
	MountPath                        string
	BackupAnnotation                 string
	BackupContainerAnnotation        string
	BackupCommandAnnotation          string
	FileExtensionAnnotation          string
	ServiceAccount                   string
	BackupCheckSchedule              string
	GlobalAccessKey                  string
	GlobalKeepJobs                   int
	GlobalFailedJobsHistoryLimit     int
	GlobalSuccessfulJobsHistoryLimit int
	GlobalRepoPassword               string
	GlobalRestoreS3AccessKey         string
	GlobalRestoreS3Bucket            string
	GlobalRestoreS3Endpoint          string
	GlobalRestoreS3SecretAccessKey   string
	GlobalS3Bucket                   string
	GlobalS3Endpoint                 string
	GlobalSecretAccessKey            string
	GlobalStatsURL                   string
	GlobalConcurrentArchiveJobsLimit int
	GlobalConcurrentBackupJobsLimit  int
	GlobalConcurrentCheckJobsLimit   int
	GlobalConcurrentPruneJobsLimit   int
	GlobalConcurrentRestoreJobsLimit int
	GlobalCPUResourceRequest         string
	GlobalCPUResourceLimit           string
	GlobalMemoryResourceRequest      string
	GlobalMemoryResourceLimit        string
	BackupImage                      string
	BackupCommandRestic              []string
	MetricsBindAddress               string
	PodExecRoleName                  string
	PodFilter                        string
	PromURL                          string
	RestartPolicy                    string
	SkipWithoutAnnotation            bool

	// Enabling this will ensure there is only one active controller manager.
	EnableLeaderElection bool
	OperatorNamespace    string

	// Allows to pass options to restic, see https://restic.readthedocs.io/en/stable/manual_rest.html?highlight=--option#usage-help
	// Format: `key=value,key2=value2`
	ResticOptions string
}

func (c Configuration) GetGlobalDefaultResources() (res corev1.ResourceRequirements) {
	if r, err := resource.ParseQuantity(c.GlobalMemoryResourceRequest); err == nil && c.GlobalMemoryResourceRequest != "" {
		if res.Requests == nil {
			res.Requests = make(corev1.ResourceList)
		}
		res.Requests[corev1.ResourceMemory] = r
	}
	if r, err := resource.ParseQuantity(c.GlobalCPUResourceRequest); err == nil && c.GlobalCPUResourceRequest != "" {
		if res.Requests == nil {
			res.Requests = make(corev1.ResourceList)
		}
		res.Requests[corev1.ResourceCPU] = r
	}
	if r, err := resource.ParseQuantity(c.GlobalMemoryResourceLimit); err == nil && c.GlobalMemoryResourceLimit != "" {
		if res.Limits == nil {
			res.Limits = make(corev1.ResourceList)
		}
		res.Limits[corev1.ResourceMemory] = r
	}
	if r, err := resource.ParseQuantity(c.GlobalCPUResourceLimit); err == nil && c.GlobalCPUResourceLimit != "" {
		if res.Limits == nil {
			res.Limits = make(corev1.ResourceList)
		}
		res.Limits[corev1.ResourceCPU] = r
	}
	return res
}

// GetGlobalRepository is a shortcut for building an S3 string "s3:<endpoint>/<bucket>"
func (c Configuration) GetGlobalRepository() string {
	return fmt.Sprintf("s3:%s/%s", c.GlobalS3Endpoint, c.GlobalS3Bucket)
}

// GetGlobalFailedJobsHistoryLimit returns the global failed jobs history limit.
// Returns global KeepJobs if unspecified.
func (c Configuration) GetGlobalFailedJobsHistoryLimit() int {
	if c.GlobalKeepJobs < 0 {
		return maxInt(0, c.GlobalFailedJobsHistoryLimit)
	}

	if c.GlobalFailedJobsHistoryLimit < 0 {
		return maxInt(0, c.GlobalKeepJobs)
	}
	return maxInt(0, c.GlobalFailedJobsHistoryLimit)
}

// GetGlobalSuccessfulJobsHistoryLimit returns the global successful jobs history limit.
// Returns global KeepJobs if unspecified.
func (c Configuration) GetGlobalSuccessfulJobsHistoryLimit() int {
	if c.GlobalKeepJobs < 0 {
		return maxInt(0, c.GlobalSuccessfulJobsHistoryLimit)
	}

	if c.GlobalSuccessfulJobsHistoryLimit < 0 {
		return maxInt(0, c.GlobalKeepJobs)
	}
	return maxInt(0, c.GlobalSuccessfulJobsHistoryLimit)
}

func maxInt(x, y int) int {
	if x > y {
		return x
	}
	return y
}
