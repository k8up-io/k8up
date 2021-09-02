package cmd

import (
	"github.com/go-logr/logr"
	"github.com/urfave/cli/v2"
)

const loggerMetadataKeyName = "logger"

// AppLogger retrieves the application-wide logger instance from the cli.Context's Metadata.
// This function will return nil if SetAppLogger was not called before this function is called.
func AppLogger(c *cli.Context) logr.Logger {
	return c.App.Metadata[loggerMetadataKeyName].(logr.Logger)
}

// SetAppLogger stores the application-wide logger instance to the cli.Context's Metadata,
// so that it can later be retrieved by AppLogger.
func SetAppLogger(c *cli.Context, logger logr.Logger) {
	c.App.Metadata[loggerMetadataKeyName] = logger
}
