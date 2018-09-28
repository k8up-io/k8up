package baas

import "github.com/spf13/viper"

type config struct {
	annotation              string
	defaultCheckSchedule    string
	podFilter               string
	backupCommandAnnotation string
	image                   string
	dataPath                string
	jobName                 string
	podName                 string
	restartPolicy           string
	globalPromURL           string
	podExecRoleName         string
	podExecAccountName      string
	globalAccessKeyID       string
	globalSecretAccessKey   string
	globalRepoPassword      string
	globalS3Endpoint        string
	globalS3Bucket          string
	globalStatsURL          string
}

func newConfig() config {
	setDefaults()
	tmp := config{
		annotation:              viper.GetString("annotation"),
		defaultCheckSchedule:    viper.GetString("checkSchedule"),
		podFilter:               viper.GetString("podFilter"),
		backupCommandAnnotation: viper.GetString("backupCommandAnnotation"),
		image:                   viper.GetString("image"),
		dataPath:                viper.GetString("dataPath"),
		jobName:                 viper.GetString("jobName"),
		podName:                 viper.GetString("podName"),
		restartPolicy:           viper.GetString("restartPolicy"),
		globalPromURL:           viper.GetString("PromURL"),
		podExecRoleName:         viper.GetString("PodExecRoleName"),
		podExecAccountName:      viper.GetString("PodExecAccountName"),
		globalAccessKeyID:       viper.GetString("GlobalAccessKeyID"),
		globalSecretAccessKey:   viper.GetString("GlobalSecretAccessKey"),
		globalRepoPassword:      viper.GetString("GlobalRepoPassword"),
		globalS3Endpoint:        viper.GetString("GlobalS3Endpoint"),
		globalS3Bucket:          viper.GetString("GlobalS3Bucket"),
		globalStatsURL:          viper.GetString("GlobalStatsURL"),
	}
	return tmp
}

func setDefaults() {
	viper.SetDefault("annotation", "appuio.ch/backup")
	viper.SetDefault("checkSchedule", "0 0 * * 0")
	viper.SetDefault("podFilter", "backupPod=true")
	viper.SetDefault("backupCommandAnnotation", "appuio.ch/backupcommand")
	viper.SetDefault("GlobalKeepJobs", 10)
	viper.SetDefault("image", "172.30.1.1:5000/myproject/restic")
	viper.SetDefault("dataPath", "/data")
	viper.SetDefault("jobName", "backupjob")
	viper.SetDefault("podName", "backupjob-pod")
	viper.SetDefault("restartPolicy", "OnFailure")
	viper.SetDefault("PromURL", "http://127.0.0.1/")
	viper.SetDefault("PodExecRoleName", "pod-executor")
	viper.SetDefault("PodExecAccountName", "pod-executor")
}
