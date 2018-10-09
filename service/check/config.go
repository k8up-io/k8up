package check

import "git.vshn.net/vshn/baas/service"

type config struct {
	service.GlobalConfig
}

func newConfig() config {
	return config{
		GlobalConfig: service.NewGlobalConfig(),
	}
}
