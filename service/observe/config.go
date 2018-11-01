package observe

import (
	"git.vshn.net/vshn/baas/service"
)

type config struct {
	service.GlobalConfig
}

func newConfig() config {
	tmp := config{
		GlobalConfig: service.NewGlobalConfig(),
	}
	return tmp
}
