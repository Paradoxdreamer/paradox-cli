package cmd

// Single place that pulls platform services into the CLI.

import (
	_ "github.com/paradox-cloud/paradox/internal/auth"    // Auth
	_ "github.com/paradox-cloud/paradox/internal/deploy"  // Deploy
	_ "github.com/paradox-cloud/paradox/internal/flags"   // Feature Flags
	_ "github.com/paradox-cloud/paradox/internal/gateway" // API Gateway
	_ "github.com/paradox-cloud/paradox/internal/queue"   // Queue
	_ "github.com/paradox-cloud/paradox/internal/storage" // Storage

	"github.com/paradox-cloud/paradox/internal/registry"
)

func init() {
	registry.AttachCommands(rootCmd)
}
