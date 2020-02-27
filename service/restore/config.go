package restore

import (
	"github.com/spf13/viper"
	configPackage "github.com/vshn/k8up/config"
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
