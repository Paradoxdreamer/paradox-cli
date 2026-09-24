package deploy

import (
	"fmt"
	"os"
	"time"

	"github.com/paradox-cloud/paradox/internal/logging"
	"github.com/paradox-cloud/paradox/internal/registry"
	"github.com/spf13/cobra"
)

func init() {
	registry.Register(&registry.Service{
		Name:          "deploy",
		Description:   "Local deploy — run, health check, rollback",
		ConfigSection: "deploy",
		Commands:      []*cobra.Command{deployCmd},
		DoctorChecks: []registry.DoctorCheck{
			{Name: "Deploy store", Fn: checkDeployStore},
		},
	})

	deployCmd.AddCommand(deployInitCmd)
	deployCmd.AddCommand(deployRunCmd)
	deployCmd.AddCommand(deployStatusCmd)
	deployCmd.AddCommand(deployStopCmd)
	deployCmd.AddCommand(deployListCmd)
	deployCmd.AddCommand(deployRollbackCmd)
	deployCmd.AddCommand(deployLogsCmd)
}

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Paradox Deploy — run apps with health checks & rollback",
	Long: `Local deploy (mini Heroku-style).

  paradox deploy init
  paradox deploy run --name APP --cmd "python3 -m http.server 8787" --health http://127.0.0.1:8787/
  paradox deploy status
  paradox deploy stop --name APP
  paradox deploy rollback --name APP
  paradox deploy logs --name APP
  paradox deploy list

Releases and PIDs live under ~/.paradox/deploy/
`,
}

var deployInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize deploy store",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := deployRoot()
		if err != nil {
			return err
		}
		if _, err := NewStore(root); err != nil {
			return err
		}
		fmt.Printf("✓ Deploy store ready at %s\n", root)
		logging.Info("deploy store initialized", "path", root)
		return nil
	},
}

var (
	runName   string
	runCmd    string
	runHealth string
	runDir    string
	runWait   string
)

var deployRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Start a new release for an app",
	RunE: func(cmd *cobra.Command, args []string) error {
		if runName == "" || runCmd == "" {
			return fmt.Errorf("--name and --cmd are required")
		}
		store, err := openStore()
		if err != nil {
			return err
		}
		wait := 10 * time.Second
		if runWait != "" {
			d, err := time.ParseDuration(runWait)
			if err != nil {
				return fmt.Errorf("invalid --wait: %w", err)
			}
			wait = d
		}
		rel, err := store.Start(runName, runCmd, runDir, runHealth, wait)
		if err != nil {
			return err
		}
		fmt.Printf("✓ Deployed %s\n", runName)
		fmt.Printf("  release:  %s\n", rel.ID)
		fmt.Printf("  pid:      %d\n", rel.PID)
		fmt.Printf("  cmd:      %s\n", rel.Command)
		if rel.HealthURL != "" {
			fmt.Printf("  health:   %s (%s)\n", rel.HealthURL, rel.HealthStatus)
		}
		logging.Info("deploy run", "app", runName, "release", rel.ID, "pid", rel.PID)
		return nil
	},
}

var statusName string

var deployStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show running apps / current releases",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := openStore()
		if err != nil {
			return err
		}
		apps, err := store.ListApps()
		if err != nil {
			return err
		}
		if statusName != "" {
			apps = filterApp(apps, statusName)
		}
		if len(apps) == 0 {
			fmt.Println("(no apps deployed)")
			return nil
		}
		fmt.Printf("%-16s %-12s %6s  %-10s %s\n", "APP", "RELEASE", "PID", "HEALTH", "COMMAND")
		for _, a := range apps {
			cur := a.Current
			if cur == nil {
				fmt.Printf("%-16s %-12s %6s  %-10s %s\n", a.Name, "-", "-", "-", "(stopped)")
				continue
			}
			health := cur.HealthStatus
			if cur.Status == "stopped" || !processAlive(cur.PID) {
				health = "stopped"
			} else if health == "" {
				health = "running"
			}
			fmt.Printf("%-16s %-12s %6d  %-10s %s\n", a.Name, shortID(cur.ID), cur.PID, health, truncate(cur.Command, 40))
		}
		return nil
	},
}

var stopName string

var deployStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the current release of an app",
	RunE: func(cmd *cobra.Command, args []string) error {
		if stopName == "" {
			return fmt.Errorf("--name is required")
		}
		store, err := openStore()
		if err != nil {
			return err
		}
		if err := store.Stop(stopName); err != nil {
			return err
		}
		fmt.Printf("✓ Stopped %s\n", stopName)
		return nil
	},
}

var deployListCmd = &cobra.Command{
	Use:   "list",
	Short: "List release history",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := openStore()
		if err != nil {
			return err
		}
		name := statusName
		if name == "" && len(args) > 0 {
			name = args[0]
		}
		rels, err := store.ListReleases(name)
		if err != nil {
			return err
		}
		if len(rels) == 0 {
			fmt.Println("(no releases)")
			return nil
		}
		fmt.Printf("%-16s %-18s %-8s %6s  %s\n", "APP", "RELEASE", "STATUS", "PID", "CREATED")
		for _, r := range rels {
			fmt.Printf("%-16s %-18s %-8s %6d  %s\n", r.App, shortID(r.ID), r.Status, r.PID, r.CreatedAt.Format(time.RFC3339))
		}
		return nil
	},
}

var rollbackName string

var deployRollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Roll back to the previous successful release",
	RunE: func(cmd *cobra.Command, args []string) error {
		if rollbackName == "" {
			return fmt.Errorf("--name is required")
		}
		store, err := openStore()
		if err != nil {
			return err
		}
		rel, err := store.Rollback(rollbackName)
		if err != nil {
			return err
		}
		fmt.Printf("✓ Rolled back %s to %s (pid %d)\n", rollbackName, shortID(rel.ID), rel.PID)
		return nil
	},
}

var logsName string
var logsLines int

var deployLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Show recent process output for an app",
	RunE: func(cmd *cobra.Command, args []string) error {
		if logsName == "" {
			return fmt.Errorf("--name is required")
		}
		store, err := openStore()
		if err != nil {
			return err
		}
		text, err := store.Logs(logsName, logsLines)
		if err != nil {
			return err
		}
		fmt.Print(text)
		return nil
	},
}

func init() {
	deployRunCmd.Flags().StringVar(&runName, "name", "", "app name")
	deployRunCmd.Flags().StringVar(&runCmd, "cmd", "", "command to run (shell)")
	deployRunCmd.Flags().StringVar(&runHealth, "health", "", "optional health URL (HTTP GET)")
	deployRunCmd.Flags().StringVar(&runDir, "dir", "", "working directory (default: current)")
	deployRunCmd.Flags().StringVar(&runWait, "wait", "10s", "max time to wait for health")
	_ = deployRunCmd.MarkFlagRequired("name")
	_ = deployRunCmd.MarkFlagRequired("cmd")

	deployStatusCmd.Flags().StringVar(&statusName, "name", "", "filter by app name")
	deployStopCmd.Flags().StringVar(&stopName, "name", "", "app name")
	_ = deployStopCmd.MarkFlagRequired("name")
	deployListCmd.Flags().StringVar(&statusName, "name", "", "filter by app name")
	deployRollbackCmd.Flags().StringVar(&rollbackName, "name", "", "app name")
	_ = deployRollbackCmd.MarkFlagRequired("name")
	deployLogsCmd.Flags().StringVar(&logsName, "name", "", "app name")
	deployLogsCmd.Flags().IntVar(&logsLines, "lines", 50, "lines to show")
	_ = deployLogsCmd.MarkFlagRequired("name")
}

func openStore() (*Store, error) {
	root, err := deployRoot()
	if err != nil {
		return nil, err
	}
	return NewStore(root)
}

func checkDeployStore() (ok bool, detail string) {
	root, err := deployRoot()
	if err != nil {
		return false, err.Error()
	}
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return false, "not initialized (run `paradox deploy init`)"
		}
		return false, err.Error()
	}
	if !info.IsDir() {
		return false, root + " is not a directory"
	}
	return true, root
}

func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func filterApp(apps []AppState, name string) []AppState {
	var out []AppState
	for _, a := range apps {
		if a.Name == name {
			out = append(out, a)
		}
	}
	return out
}
