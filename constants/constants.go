// Constants provides constant values, either derived from the environmen variables
// or directly hardcoded. This should be replaced by templating CRDs in the future for more
// flexibility.

package constants

import (
	"os"
	"strconv"
)

const (
	RestoreS3EndpointEnvName        = "RESTORE_S3ENDPOINT"
	RestoreS3AccessKeyIDEnvName     = "RESTORE_ACCESSKEYID"
	RestoreS3SecretAccessKeyEnvName = "RESTORE_SECRETACCESSKEY"
	ResticRepositoryEnvName         = "RESTIC_REPOSITORY"
	ResticPasswordEnvName           = "RESTIC_PASSWORD"
	AwsAccessKeyIDEnvName           = "AWS_ACCESS_KEY_ID"
	AwsSecretAccessKeyEnvName       = "AWS_SECRET_ACCESS_KEY"
)

// TODO: this will be replaced with a CRD in the future.

var (
	mountPath                      = getEnvVar("BACKUP_DATAPATH", "/data")
	backupAnnotation               = getEnvVar("BACKUP_ANNOTATION", "k8up.syn.tools/backup")
	backupCommandAnnotation        = getEnvVar("BACKUP_BACKUPCOMMANDANNOTATION", "k8up.syn.tools/backupcommand")
	fileExtensionAnnotation        = getEnvVar("BACKUP_FILEEXTENSIONANNOTATION", "k8up.syn.tools/file-extension")
	serviceAccount                 = getEnvVar("BACKUP_PODEXECACCOUNTNAME", "pod-executor")
	backupCheckSchedule            = getEnvVar("BACKUP_CHECKSCHEDULE", "0 0 * * 0")
	globalAccessKey                = getEnvVar("BACKUP_GLOBALACCESSKEYID", "")
	globalKeepJobs                 = getEnvVar("BACKUP_GLOBALKEEPJOBS", "6")
	globalRepoPassword             = getEnvVar("BACKUP_GLOBALREPOPASSWORD", "")
	globalRestoreS3AccessKey       = getEnvVar("BACKUP_GLOBALRESTORES3ACCESKEYID", "")
	globalRestoreS3Bucket          = getEnvVar("BACKUP_GLOBALRESTORES3BUCKET", "")
	globalRestoreS3Endpoint        = getEnvVar("BACKUP_GLOBALRESTORES3ENDPOINT", "")
	globalRestoreS3SecretAccessKey = getEnvVar("BACKUP_GLOBALRESTORES3SECRETACCESSKEY", "")
	globalS3Bucket                 = getEnvVar("BACKUP_GLOBALS3BUCKET", "")
	globalS3Endpoint               = getEnvVar("BACKUP_GLOBALS3ENDPOINT", "")
	globalSecretAccessKey          = getEnvVar("BACKUP_GLOBALSECRETACCESSKEY", "")
	globalStatsURL                 = getEnvVar("BACKUP_GLOBALSTATSURL", "")
	backupImage                    = getEnvVar("BACKUP_IMAGE", "172.30.1.1:5000/myproject/restic")
	podExecRoleName                = getEnvVar("BACKUP_PODEXECROLENAME", "pod-executor")
	podFilter                      = getEnvVar("BACKUP_PODFILTER", "backupPod=true")
	promURL                        = getEnvVar("BACKUP_PROMURL", "")
	restartPolicy                  = getEnvVar("BACKUP_RESTARTPOLICY", "OnFailure")
)

func GetRestartPolicy() string {
	return restartPolicy
}

func GetPromURL() string {
	return promURL
}

func GetPodFilter() string {
	return podFilter
}

func GetPodExecRoleName() string {
	return podExecRoleName
}

func GetBackupImage() string {
	return backupImage
}

func GetGlobalStatsURL() string {
	return globalStatsURL
}

func GetGlobalSecretAccessKey() string {
	return globalSecretAccessKey
}

func GetGlobalS3Endpoint() string {
	return globalS3Endpoint
}

func GetGlobalS3Bucket() string {
	return globalS3Bucket
}

func GetGlobalRestoreS3SecretAccessKey() string {
	return globalRestoreS3SecretAccessKey
}

func GetGlobalRestoreS3Endpoint() string {
	return globalRestoreS3Endpoint
}

func GetGlobalRestoreS3Bucket() string {
	return globalRestoreS3Bucket
}

func GetGlobalRestoreS3AccessKey() string {
	return globalRestoreS3AccessKey
}

func GetGlobalRepoPassword() string {
	return globalRepoPassword
}

func GetGlobalKeepJobs() int {
	if i, err := strconv.Atoi(globalKeepJobs); err != nil {
		return 6
	} else {
		return i
	}
}

func GetGlobalAccessKey() string {
	return globalAccessKey
}

func GetBackupCheckSchedule() string {
	return backupCheckSchedule
}

func GetServiceAccount() string {
	return serviceAccount
}

func GetFileExtensionAnnotation() string {
	return fileExtensionAnnotation
}

func GetBackupCommandAnnotation() string {
	return backupCommandAnnotation
}

func GetMountPath() string {
	return mountPath
}

func GetBackupAnnotation() string {
	return backupAnnotation
}

func getEnvVar(name, defaultValue string) string {
	if str, ok := os.LookupEnv(name); ok {
		return str
	}
	return defaultValue
}
