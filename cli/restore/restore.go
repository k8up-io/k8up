package restore

var (
	Cfg = &RestoreConfig{}
)

type RestoreConfig struct {
	// spec.restoreMethod.folder.claimName
	ClaimName  string
	Kubeconfig string
	Namespace  string
	// metadata.name
	RestoreName string
	// spec.podSecurityContext.runAsUser
	RunAsUser int64
	// one of restore methods s3 || pvc
	RestoreMethod string
	// spec.snapshot
	Snapshot string
	// spec.backend.repoPasswordSecretRef.name
	SecretRef string
	// spec.backend.repoPasswordSecretRef.key
	SecretRefKey string

	// spec.backend.s3.endpoint
	S3Endpoint string
	// spec.backend.s3.bucket
	S3Bucket string
	// spec.backend.s3.accessKeyIDSecretRef.name && spec.backend.s3.secretAccessKeySecretRef.name
	S3SecretRef string
	// spec.backend.s3.accessKeyIDSecretRef.key
	S3SecretRefUsernameKey string
	// spec.backend.s3.secretAccessKeySecretRef.key
	S3SecretRefPasswordKey string

	// spec.restoreMethod.s3.endpoint
	RestoreToS3Endpoint string
	// spec.restoreMethod.s3.bucket
	RestoreToS3Bucket string
	// spec.restoreMethod.s3.accessKeyIDSecretRef.name && spec.restoreMethod.s3.secretAccessKeySecretRef.name
	RestoreToS3Secret string
	// spec.restoreMethod.s3.accessKeyIDSecretRef.name
	RestoreToS3SecretUsernameKey string
	// spec.restoreMethod.s3.secretAccessKeySecretRef.name
	RestoreToS3SecretPasswordKey string
}
