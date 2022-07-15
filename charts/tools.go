//go:build tools

// Package tools is a place to put any tooling dependencies as imports.
// Go modules will be forced to download and install them.
package tools

import (
	// helm-docs
	_ "github.com/norwoodj/helm-docs/cmd/helm-docs"
)
