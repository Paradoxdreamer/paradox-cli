package cmd

// This file is the single place that pulls platform services into the CLI.
// Each service registers itself via registry.Register in its init().
//
// Rule: when you add a new service (deploy, …), add a
// blank import here. Do not scatter imports across other cmd files.

import (
	_ "github.com/paradox-cloud/paradox/internal/auth"    // Paradox Auth
	_ "github.com/paradox-cloud/paradox/internal/flags"   // Feature Flags
	_ "github.com/paradox-cloud/paradox/internal/queue"   // Paradox Queue
	_ "github.com/paradox-cloud/paradox/internal/storage" // Object Storage

	"github.com/paradox-cloud/paradox/internal/registry"
)

func init() {
	registry.AttachCommands(rootCmd)
}
