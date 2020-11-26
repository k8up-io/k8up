package cfg

// Configuration holds a strongly-typed tree of the configuration
type Configuration struct {
	MountPath                      string `koanf:"datapath"`
	BackupAnnotation               string `koanf:"annotation"`
	BackupCommandAnnotation        string `koanf:"backupcommandannotation"`
	FileExtensionAnnotation        string `koanf:"fileextensionannotation"`
	ServiceAccount                 string `koanf:"podexecaccountname"`
	BackupCheckSchedule            string `koanf:"checkschedule"`
	GlobalAccessKey                string `koanf:"globalaccesskeyid"`
	GlobalKeepJobs                 int    `koanf:"globalkeepjobs"`
	GlobalRepoPassword             string `koanf:"globalrepopassword"`
	GlobalRestoreS3AccessKey       string `koanf:"globalrestores3accesskeyid"`
	GlobalRestoreS3Bucket          string `koanf:"globalrestores3bucket"`
	GlobalRestoreS3Endpoint        string `koanf:"globalrestores3endpoint"`
	GlobalRestoreS3SecretAccessKey string `koanf:"globalrestores3secretaccesskeyid"`
	GlobalS3Bucket                 string `koanf:"globals3bucket"`
	GlobalS3Endpoint               string `koanf:"globals3endpoint"`
	GlobalSecretAccessKey          string `koanf:"globalsecretaccesskeyid"`
	GlobalStatsURL                 string `koanf:"globalstatsurl"`
	BackupImage                    string `koanf:"image"`
	MetricsBindAddress             string `koanf:"metrics-bindaddress"`
	PodExecRoleName                string `koanf:"podexecrolename"`
	PodFilter                      string `koanf:"podfilter"`
	PromURL                        string `koanf:"promurl"`
	RestartPolicy                  string `koanf:"restartpolicy"`
}

// NewDefaultConfig retrieves the config with sane defaults
func NewDefaultConfig() *Configuration {
	return &Configuration{
		MountPath:               "/data",
		BackupAnnotation:        "k8up.syn.tools/backup",
		BackupCommandAnnotation: "k8up.syn.tools/backupcommand",
		FileExtensionAnnotation: "k8up.syn.tools/file-extension",
		ServiceAccount:          "pod-executor",
		BackupCheckSchedule:     "0 0 * * 0",
		GlobalKeepJobs:          6,
		BackupImage:             "172.30.1.1:5000/myproject/restic",
		PodExecRoleName:         "pod-executor",
		RestartPolicy:           "OnFailure",
		MetricsBindAddress:      ":8080",
	}
}

var (
	Config = NewDefaultConfig()
)
