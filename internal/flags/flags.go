package flags

import (
	"fmt"
	"os"
	"strings"

	"github.com/paradox-cloud/paradox/internal/logging"
	"github.com/paradox-cloud/paradox/internal/registry"
	"github.com/spf13/cobra"
)

func init() {
	registry.Register(&registry.Service{
		Name:          "flags",
		Description:   "Feature flags — rollouts, targeting, kill switches",
		ConfigSection: "flags",
		Commands:      []*cobra.Command{flagsCmd},
		DoctorChecks: []registry.DoctorCheck{
			{Name: "Flags store", Fn: checkFlagsStore},
		},
	})

	flagsCmd.AddCommand(flagsInitCmd)
	flagsCmd.AddCommand(flagsListCmd)
	flagsCmd.AddCommand(flagsGetCmd)
	flagsCmd.AddCommand(flagsSetCmd)
	flagsCmd.AddCommand(flagsEnableCmd)
	flagsCmd.AddCommand(flagsDisableCmd)
	flagsCmd.AddCommand(flagsDeleteCmd)
	flagsCmd.AddCommand(flagsEvalCmd)
}

var flagsCmd = &cobra.Command{
	Use:   "flags",
	Short: "Paradox Feature Flags",
	Long: `Feature flags with percentage rollouts, user targeting, environments, and kill switches.

  paradox flags init
  paradox flags set <key> [--pct 10] [--env development,staging]
  paradox flags enable <key>
  paradox flags disable <key>
  paradox flags eval <key> --user u --email e --env development
  paradox flags list
  paradox flags get <key>
  paradox flags delete <key>
`,
}

var flagsInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the flags store",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := flagsRoot()
		if err != nil {
			return err
		}
		if _, err := NewStore(root); err != nil {
			return err
		}
		fmt.Printf("✓ Flags store ready at %s\n", root)
		logging.Info("flags store initialized", "path", root)
		return nil
	},
}

var flagsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all flags",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := openStore()
		if err != nil {
			return err
		}
		list, err := store.List()
		if err != nil {
			return err
		}
		if len(list) == 0 {
			fmt.Println("(no flags)")
			return nil
		}
		fmt.Printf("%-24s %-8s %4s  %s\n", "KEY", "ENABLED", "PCT", "ENVIRONMENTS")
		for _, f := range list {
			envs := strings.Join(f.Environments, ",")
			if envs == "" {
				envs = "*"
			}
			fmt.Printf("%-24s %-8v %3d%%  %s\n", f.Key, f.Enabled, f.Percentage, envs)
		}
		return nil
	},
}

var flagsGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Show a flag",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := openStore()
		if err != nil {
			return err
		}
		f, err := store.Get(args[0])
		if err != nil {
			return err
		}
		printFlag(f)
		return nil
	},
}

var (
	setPct     int
	setDesc    string
	setEnvs    string
	setAllow   string
	setDeny    string
	setEnabled bool
)

var flagsSetCmd = &cobra.Command{
	Use:   "set <key>",
	Short: "Create or update a flag",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := openStore()
		if err != nil {
			return err
		}
		key := args[0]
		f, err := store.Get(key)
		if err != nil {
			f = &Flag{Key: key, Enabled: true, Percentage: 0}
		}
		if cmd.Flags().Changed("pct") {
			f.Percentage = setPct
		}
		if cmd.Flags().Changed("desc") {
			f.Description = setDesc
		}
		if cmd.Flags().Changed("env") {
			f.Environments = splitCSV(setEnvs)
		}
		if cmd.Flags().Changed("allow") {
			f.AllowUsers = splitCSV(setAllow)
		}
		if cmd.Flags().Changed("deny") {
			f.DenyUsers = splitCSV(setDeny)
		}
		if cmd.Flags().Changed("enabled") {
			f.Enabled = setEnabled
		}
		if err := store.Set(f); err != nil {
			return err
		}
		fmt.Printf("✓ Flag %q saved\n", key)
		printFlag(f)
		logging.Info("flag saved", "key", key, "enabled", f.Enabled, "pct", f.Percentage)
		return nil
	},
}

var flagsEnableCmd = &cobra.Command{
	Use:   "enable <key>",
	Short: "Turn the kill switch on",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return setKillSwitch(args[0], true)
	},
}

var flagsDisableCmd = &cobra.Command{
	Use:   "disable <key>",
	Short: "Turn the kill switch off (instant)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return setKillSwitch(args[0], false)
	},
}

var flagsDeleteCmd = &cobra.Command{
	Use:   "delete <key>",
	Short: "Delete a flag",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := openStore()
		if err != nil {
			return err
		}
		if err := store.Delete(args[0]); err != nil {
			return err
		}
		fmt.Printf("✓ Deleted flag %q\n", args[0])
		return nil
	},
}

var (
	evalUser  string
	evalEmail string
	evalEnv   string
)

var flagsEvalCmd = &cobra.Command{
	Use:   "eval <key>",
	Short: "Evaluate a flag for a user/environment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := openStore()
		if err != nil {
			return err
		}
		f, err := store.Get(args[0])
		if err != nil {
			return err
		}
		ctx := Context{
			UserID:      evalUser,
			Email:       evalEmail,
			Environment: evalEnv,
		}
		if ctx.Environment == "" {
			ctx.Environment = "development"
		}
		ev := f.Evaluate(ctx)
		fmt.Printf("flag:    %s\n", ev.Key)
		fmt.Printf("enabled: %v\n", ev.Enabled)
		fmt.Printf("reason:  %s\n", ev.Reason)
		return nil
	},
}

func init() {
	flagsSetCmd.Flags().IntVar(&setPct, "pct", 0, "percentage rollout 0–100")
	flagsSetCmd.Flags().StringVar(&setDesc, "desc", "", "description")
	flagsSetCmd.Flags().StringVar(&setEnvs, "env", "", "comma-separated environments")
	flagsSetCmd.Flags().StringVar(&setAllow, "allow", "", "comma-separated allow user ids/emails")
	flagsSetCmd.Flags().StringVar(&setDeny, "deny", "", "comma-separated deny user ids/emails")
	flagsSetCmd.Flags().BoolVar(&setEnabled, "enabled", true, "kill switch")

	flagsEvalCmd.Flags().StringVar(&evalUser, "user", "", "user id")
	flagsEvalCmd.Flags().StringVar(&evalEmail, "email", "", "user email")
	flagsEvalCmd.Flags().StringVar(&evalEnv, "env", "development", "environment")
}

func setKillSwitch(key string, enabled bool) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	f, err := store.Get(key)
	if err != nil {
		return err
	}
	f.Enabled = enabled
	if err := store.Set(f); err != nil {
		return err
	}
	state := "enabled"
	if !enabled {
		state = "disabled"
	}
	fmt.Printf("✓ Flag %q %s\n", key, state)
	logging.Info("flag kill switch", "key", key, "enabled", enabled)
	return nil
}

func printFlag(f *Flag) {
	fmt.Printf("  key:          %s\n", f.Key)
	fmt.Printf("  enabled:      %v\n", f.Enabled)
	fmt.Printf("  percentage:   %d%%\n", f.Percentage)
	fmt.Printf("  environments: %s\n", orStar(f.Environments))
	fmt.Printf("  allow_users:  %s\n", orNone(f.AllowUsers))
	fmt.Printf("  deny_users:   %s\n", orNone(f.DenyUsers))
	if f.Description != "" {
		fmt.Printf("  description:  %s\n", f.Description)
	}
}

func orStar(ss []string) string {
	if len(ss) == 0 {
		return "*"
	}
	return strings.Join(ss, ", ")
}

func orNone(ss []string) string {
	if len(ss) == 0 {
		return "(none)"
	}
	return strings.Join(ss, ", ")
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func openStore() (*Store, error) {
	root, err := flagsRoot()
	if err != nil {
		return nil, err
	}
	return NewStore(root)
}

func checkFlagsStore() (ok bool, detail string) {
	root, err := flagsRoot()
	if err != nil {
		return false, err.Error()
	}
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return false, "not initialized (run `paradox flags init`)"
		}
		return false, err.Error()
	}
	if !info.IsDir() {
		return false, root + " is not a directory"
	}
	return true, root
}
