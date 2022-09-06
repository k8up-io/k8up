//go:build tools

// Place any runtime dependencies as imports in this file.
// Go modules will be forced to download and install them.
package tools

import (
	_ "github.com/elastic/crd-ref-docs"
	_ "sigs.k8s.io/kind"
)
