package cfg

import (
	"fmt"
	"strings"
	"time"
)

const (
	// RestoreTypeS3 indicates that the restore shall be done to an S3 endpoint.
	RestoreTypeS3 = "s3"

	// RestoreTypeFolder indicates that the restore shall be done to a folder,
	// usually a RWX PVC mounted to the Pod of the restore process.
	RestoreTypeFolder = "folder"
)

var (
	// Config contains the values of the user-provided configuration of the operator module,
	// combined with the default values as defined in operator.Command.
	Config = &Configuration{}
)

// Configuration holds a strongly-typed tree of the configuration
type Configuration struct {
	DoCheck   bool
	DoPrune   bool
	DoRestore bool
	DoArchive bool

	BackupCommandAnnotation       string
	BackupFileExtensionAnnotation string
	BackupContainerAnnotation     string
	BackupDir                     string

	SkipPreBackup bool

	PromURL    string
	WebhookURL string

	Hostname   string
	KubeConfig string

	ResticBin        string
	ResticRepository string
	ResticOptions    string

	RestoreDir         string
	RestoreS3Endpoint  string
	RestoreS3AccessKey string
	RestoreS3SecretKey string
	RestoreSnap        string
	RestoreType        string
	RestoreFilter      string
	VerifyRestore      bool
	RestoreTrimPath    bool

	PruneKeepLast    int
	PruneKeepHourly  int
	PruneKeepDaily   int
	PruneKeepWeekly  int
	PruneKeepMonthly int
	PruneKeepYearly  int
	PruneKeepTags    bool

	PruneKeepWithin        string
	PruneKeepWithinHourly  string
	PruneKeepWithinDaily   string
	PruneKeepWithinWeekly  string
	PruneKeepWithinMonthly string
	PruneKeepWithinYearly  string

	Tags []string

	TargetPods []string

	SleepDuration time.Duration
}

// Validate ensures a consistent configuration and returns an error should that not be the case
func (c *Configuration) Validate() error {
	if err := c.validateRestore(); err != nil {
		return err
	}
	if err := c.validatePrune(); err != nil {
		return err
	}

	return nil
}

func (c *Configuration) validatePrune() error {
	if !c.DoPrune {
		return nil
	}

	keepN := map[string]int{
		"keepLast":    c.PruneKeepLast,
		"keepHourly":  c.PruneKeepHourly,
		"keepDaily":   c.PruneKeepDaily,
		"keepWeekly":  c.PruneKeepWeekly,
		"keepMonthly": c.PruneKeepMonthly,
		"keepYearly":  c.PruneKeepYearly,
	}
	for arg, val := range keepN {
		if val < 0 {
			return fmt.Errorf("the value of the argument %s must not be negative, but was set to '%d'", arg, val)
		}
	}

	keepWithin := map[string]string{
		"keepWithin":        c.PruneKeepWithin,
		"keepWithinHourly":  c.PruneKeepWithinHourly,
		"keepWithinDaily":   c.PruneKeepWithinDaily,
		"keepWithinWeekly":  c.PruneKeepWithinWeekly,
		"keepWithinMonthly": c.PruneKeepWithinMonthly,
		"keepWithinYearly":  c.PruneKeepWithinYearly,
	}
	for arg, val := range keepWithin {
		if val == "" {
			continue
		}
		d, err := time.ParseDuration(val)
		if err != nil {
			return fmt.Errorf("the duration '%s' of the argument %s is not valid: %w", val, arg, err)
		}
		if d <= 0 {
			return fmt.Errorf("the duration '%s' of the argument %s must not be negative", val, arg)
		}
	}
	return nil
}

func (c *Configuration) validateRestore() error {
	if !c.DoRestore {
		return nil
	}

	c.RestoreType = strings.ToLower(c.RestoreType)
	switch c.RestoreType {
	case RestoreTypeS3:
		switch {
		case "" == c.RestoreS3Endpoint:
			return fmt.Errorf("if the restore type is set to '%s', then the restore s3 endpoint must be defined", RestoreTypeS3)
		case "" == c.RestoreS3AccessKey:
			return fmt.Errorf("if the restore type is set to '%s', then the restore s3 access key must be defined", RestoreTypeS3)
		case "" == c.RestoreS3SecretKey:
			return fmt.Errorf("if the restore type is set to '%s', then the restore s3 secret key must be defined", RestoreTypeS3)
		}

	case RestoreTypeFolder:
		if "" == c.RestoreDir {
			return fmt.Errorf("if the restore type is set to '%s', then the restore directory must be defined", RestoreTypeFolder)
		}

	default:
		return fmt.Errorf("the restore type '%s' is unknown", c.RestoreType)
	}
	return nil
}
