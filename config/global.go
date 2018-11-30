package config

import "github.com/spf13/viper"

// Global contains configuration that is the same for all services
type Global struct {
	Image                          string
	GlobalAccessKeyID              string
	GlobalSecretAccessKey          string
	GlobalRepoPassword             string
	GlobalS3Endpoint               string
	GlobalS3Bucket                 string
	GlobalStatsURL                 string
	GlobalRestoreS3Endpoint        string
	GlobalRestoreS3Bucket          string
	GlobalRestoreS3AccessKeyID     string
	GlobalRestoreS3SecretAccessKey string
	GlobalArchiveS3Endpoint        string
	GlobalArchiveS3Bucket          string
	GlobalArchiveS3AccessKeyID     string
	GlobalArchiveS3SecretAccessKey string
	Label                          string
	Identifier                     string
	RestartPolicy                  string
	GlobalPromURL                  string
	GlobalKeepJobs                 int
}

// New returns an instance of Global with the fields set to
// the approriate env variables.
func New() Global {
	initDefaults()
	return Global{
		GlobalAccessKeyID:              viper.GetString("GlobalAccessKeyID"),
		GlobalSecretAccessKey:          viper.GetString("GlobalSecretAccessKey"),
		GlobalRepoPassword:             viper.GetString("GlobalRepoPassword"),
		GlobalS3Endpoint:               viper.GetString("GlobalS3Endpoint"),
		GlobalS3Bucket:                 viper.GetString("GlobalS3Bucket"),
		GlobalStatsURL:                 viper.GetString("GlobalStatsURL"),
		GlobalRestoreS3Bucket:          viper.GetString("GlobalRestoreS3Bucket"),
		GlobalRestoreS3Endpoint:        viper.GetString("GlobalRestoreS3Endpoint"),
		GlobalRestoreS3AccessKeyID:     viper.GetString("GlobalRestoreS3AccessKeyID"),
		GlobalRestoreS3SecretAccessKey: viper.GetString("GlobalRestoreS3SecretAccessKey"),
		GlobalArchiveS3Bucket:          viper.GetString("GlobalArchiveS3Bucket"),
		GlobalArchiveS3Endpoint:        viper.GetString("GlobalArchiveS3Endpoint"),
		GlobalArchiveS3AccessKeyID:     viper.GetString("GlobalArchiveS3AccessKeyID"),
		GlobalArchiveS3SecretAccessKey: viper.GetString("GlobalArchiveS3SecretAccessKey"),
		Image:                          viper.GetString("image"),
		Label:                          viper.GetString("label"),
		Identifier:                     viper.GetString("identifier"),
		RestartPolicy:                  viper.GetString("restartPolicy"),
		GlobalPromURL:                  viper.GetString("PromURL"),
		GlobalKeepJobs:                 viper.GetInt("GlobalKeepJobs"),
	}
}

func initDefaults() {
	viper.SetDefault("image", "172.30.1.1:5000/myproject/restic")
	viper.SetDefault("label", "baasresource")
	viper.SetDefault("identifier", "baasid")
	viper.SetDefault("restartPolicy", "Never")
	viper.SetDefault("PromURL", "http://127.0.0.1/")
	viper.SetDefault("GlobalKeepJobs", 10)
}
