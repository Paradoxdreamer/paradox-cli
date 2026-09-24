package agent

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/paradox-cloud/paradox/internal/logging"
	"github.com/paradox-cloud/paradox/internal/registry"
	"github.com/spf13/cobra"
)

func init() {
	registry.Register(&registry.Service{
		Name: "agent", Description: "Agent Runtime — ParadoxGPT foundation",
		ConfigSection: "agent", Commands: []*cobra.Command{agentCmd},
		DoctorChecks: []registry.DoctorCheck{{Name: "Agent store", Fn: checkAgent}},
	})
	agentCmd.AddCommand(agentInitCmd, agentCreateCmd, agentListCmd, agentRunCmd, agentToolsCmd)
}

var agentCmd = &cobra.Command{
	Use: "agent", Short: "Paradox Agent Runtime (ParadoxGPT engine)",
	Long: `Agent Runtime is the engine under ParadoxGPT.

  paradox agent create assistant --prompt "You are helpful."
  paradox agent run assistant "use tool echo_time"
  paradox agent tools

Default model is echo (local). Real LLM adapters plug in later.
`,
}

var agentInitCmd = &cobra.Command{
	Use: "init", Short: "Initialize agent store",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := agentRoot()
		if err != nil {
			return err
		}
		if _, err := NewRuntime(root); err != nil {
			return err
		}
		fmt.Printf("✓ Agent runtime ready at %s\n", root)
		logging.Info("agent runtime initialized", "path", root)
		return nil
	},
}

var createPrompt, createModel, createTools, createDesc string

var agentCreateCmd = &cobra.Command{
	Use: "create <name>", Short: "Create or update an agent", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := agentRoot()
		if err != nil {
			return err
		}
		rt, err := NewRuntime(root)
		if err != nil {
			return err
		}
		var tools []string
		for _, t := range strings.Split(createTools, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tools = append(tools, t)
			}
		}
		a := &Agent{Name: args[0], Description: createDesc, SystemPrompt: createPrompt, Model: createModel, Tools: tools, MaxSteps: 5}
		if a.SystemPrompt == "" {
			a.SystemPrompt = "You are a helpful Paradox agent."
		}
		if a.Model == "" {
			a.Model = "echo"
		}
		if err := rt.Store.Put(a); err != nil {
			return err
		}
		fmt.Printf("✓ Agent %q saved (model=%s)\n", a.Name, a.Model)
		return nil
	},
}

var agentListCmd = &cobra.Command{
	Use: "list", Short: "List agents",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := agentRoot()
		if err != nil {
			return err
		}
		rt, err := NewRuntime(root)
		if err != nil {
			return err
		}
		list, err := rt.Store.List()
		if err != nil {
			return err
		}
		if len(list) == 0 {
			fmt.Println("(no agents)")
			return nil
		}
		fmt.Printf("%-16s %-8s %s\n", "NAME", "MODEL", "DESCRIPTION")
		for _, a := range list {
			desc := a.Description
			if desc == "" {
				desc = a.SystemPrompt
			}
			if len(desc) > 50 {
				desc = desc[:47] + "..."
			}
			fmt.Printf("%-16s %-8s %s\n", a.Name, a.Model, desc)
		}
		return nil
	},
}

var agentRunCmd = &cobra.Command{
	Use: "run <name> <message>", Short: "Run an agent", Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := agentRoot()
		if err != nil {
			return err
		}
		rt, err := NewRuntime(root)
		if err != nil {
			return err
		}
		res, err := rt.Run(context.Background(), args[0], strings.Join(args[1:], " "))
		if err != nil {
			return err
		}
		fmt.Println("── Agent response ──")
		fmt.Println(res.Output)
		fmt.Println()
		fmt.Printf("steps=%d tool_calls=%d model=%s duration=%s\n", res.Steps, res.ToolCalls, res.Model, res.Duration)
		return nil
	},
}

var agentToolsCmd = &cobra.Command{
	Use: "tools", Short: "List available tools",
	RunE: func(cmd *cobra.Command, args []string) error {
		r := NewToolRegistry()
		fmt.Printf("%-16s %s\n", "NAME", "DESCRIPTION")
		for _, s := range r.Specs() {
			fmt.Printf("%-16s %s\n", s.Name, s.Description)
		}
		return nil
	},
}

func init() {
	agentCreateCmd.Flags().StringVar(&createPrompt, "prompt", "", "system prompt")
	agentCreateCmd.Flags().StringVar(&createModel, "model", "echo", "model adapter")
	agentCreateCmd.Flags().StringVar(&createTools, "tools", "", "comma-separated allow-list")
	agentCreateCmd.Flags().StringVar(&createDesc, "desc", "", "description")
}

func checkAgent() (bool, string) {
	root, err := agentRoot()
	if err != nil {
		return false, err.Error()
	}
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return false, "not initialized (run `paradox agent init`)"
		}
		return false, err.Error()
	}
	if !info.IsDir() {
		return false, "not a directory"
	}
	return true, root
}
