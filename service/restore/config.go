package restore

import (
	configPackage "github.com/vshn/k8up/config"
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
