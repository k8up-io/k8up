package restore

import (
	configPackage "git.vshn.net/vshn/baas/config"
	"github.com/spf13/viper"
)

type config struct {
	configPackage.Global
	image string
}

func newConfig() config {
	setDefaults()
	tmp := config{
		image:  viper.GetString("image"),
		Global: configPackage.New(),
	}
	return tmp
}

func setDefaults() {
}
