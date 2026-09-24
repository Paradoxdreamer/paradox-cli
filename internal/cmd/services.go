package cmd

import (
	_ "github.com/paradox-cloud/paradox/internal/auth"
	_ "github.com/paradox-cloud/paradox/internal/deploy"
	_ "github.com/paradox-cloud/paradox/internal/flags"
	_ "github.com/paradox-cloud/paradox/internal/gateway"
	_ "github.com/paradox-cloud/paradox/internal/observability"
	_ "github.com/paradox-cloud/paradox/internal/queue"
	_ "github.com/paradox-cloud/paradox/internal/storage"

	"github.com/paradox-cloud/paradox/internal/registry"
)

func init() {
	registry.AttachCommands(rootCmd)
}
