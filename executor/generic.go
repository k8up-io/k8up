package executor

import (
	"github.com/go-logr/logr"
	"github.com/vshn/k8up/job"
)

type generic struct {
	job.Config
}

func (g *generic) Logger() logr.Logger {
	return g.Log
}

func (g *generic) Exclusive() bool {
	return false
}

func (g *generic) GetName() string {
	return g.Name
}
