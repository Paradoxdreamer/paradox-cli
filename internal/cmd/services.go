package cmd

// This file is the single place that pulls platform services into the CLI.
// Each service registers itself via registry.Register in its init().
//
// Rule: when you add a new service (queue, deploy, storage, …), add a
// blank import here. Do not scatter imports across other cmd files.

import (
	_ "github.com/paradox-cloud/paradox/internal/auth" // Paradox Auth

	"github.com/paradox-cloud/paradox/internal/registry"
)

func init() {
	// Attach all registered service commands to the root.
	// Runs after every service's init() because of Go's init order
	// within the same package and imported packages.
	registry.AttachCommands(rootCmd)
}
