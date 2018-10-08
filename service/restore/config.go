package restore

import (
	"git.vshn.net/vshn/baas/service"
	"github.com/spf13/viper"
)

type config struct {
	service.GlobalConfig
	image string
}

func newConfig() config {
	setDefaults()
	tmp := config{
		image:        viper.GetString("image"),
		GlobalConfig: service.NewGlobalConfig(),
	}
	return tmp
}

func setDefaults() {
}
