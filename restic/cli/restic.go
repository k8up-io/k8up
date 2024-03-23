package cli

import (
	"context"
	"github.com/k8up-io/k8up/v2/operator/utils"
	"path"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"

	"github.com/k8up-io/k8up/v2/restic/cfg"
	"github.com/k8up-io/k8up/v2/restic/dto"
)

type ArrayOpts []string

func (a *ArrayOpts) String() string {
	return strings.Join(*a, ", ")
}

func (a *ArrayOpts) BuildArgs() []string {
	argList := make([]string, 0)
	for _, elem := range *a {
		argList = append(argList, "--tag", elem)
	}
	return argList
}

func (a *ArrayOpts) Set(value string) error {
	*a = append(*a, value)
	return nil
}

type Restic struct {
	resticPath string
	logger     logr.Logger
	snapshots  []dto.Snapshot
	ctx        context.Context
	bucket     string

	// globalFlags are applied to all invocations of restic
	globalFlags  Flags
	statsHandler StatsHandler

	caCert     string
	clientCert clientCert
}

type clientCert struct {
	cert string
	key  string
	pem  string
}

// New returns a new Restic reference
func New(ctx context.Context, logger logr.Logger, statsHandler StatsHandler) *Restic {
	globalFlags := Flags{}

	options := strings.Split(cfg.Config.ResticOptions, ",")
	if len(options) > 0 {
		logger.Info("using the following restic options", "options", options)
		globalFlags.AddFlag("--option", options...)
	}

	var caCert string
	if cfg.Config.CACert != "" {
		caCert = cfg.Config.CACert
		globalFlags.AddFlag("--cacert", cfg.Config.CACert)
	}
	var cc clientCert
	if cfg.Config.ClientCert != "" && cfg.Config.ClientKey != "" {
		var pemFileName strings.Builder
		pemFileName.WriteString("restic.repo.")
		pemFileName.WriteString(utils.RandomStringGenerator(10))
		pemFileName.WriteString(".pem")

		cc.cert = cfg.Config.ClientCert
		cc.key = cfg.Config.ClientKey
		cc.pem = filepath.Join(cfg.Config.VarDir, pemFileName.String())
		globalFlags.AddFlag("--tls-client-cert", cc.pem)
	}

	return &Restic{
		logger:       logger,
		resticPath:   cfg.Config.ResticBin,
		ctx:          ctx,
		bucket:       path.Base(cfg.Config.ResticRepository),
		caCert:       caCert,
		clientCert:   cc,
		globalFlags:  globalFlags,
		statsHandler: statsHandler,
	}
}
