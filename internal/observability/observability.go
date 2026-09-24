package observability

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/paradox-cloud/paradox/internal/logging"
	"github.com/paradox-cloud/paradox/internal/registry"
	"github.com/spf13/cobra"
)

func init() {
	registry.Register(&registry.Service{
		Name: "observability", Description: "Logs, metrics, events — local observability",
		ConfigSection: "observability", Commands: []*cobra.Command{obsCmd},
		DoctorChecks: []registry.DoctorCheck{{Name: "Observability store", Fn: checkObs}},
	})
	obsCmd.AddCommand(obsInitCmd, obsTrackCmd, obsErrorCmd, obsMetricCmd, obsTailCmd, obsMetricsCmd, obsStatusCmd)
}

var obsCmd = &cobra.Command{
	Use: "obs", Short: "Paradox Observability — events, metrics, errors",
	Long: `Tiny local observability layer.

  paradox obs track payment.completed --attr amount=42
  paradox obs error "payment failed" --attr code=card_declined
  paradox obs metric api_latency 183
  paradox obs tail -n 20
  paradox obs metrics
  paradox obs status
`,
}

var obsInitCmd = &cobra.Command{
	Use: "init", Short: "Initialize observability store",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := obsRoot()
		if err != nil {
			return err
		}
		if _, err := NewStore(root); err != nil {
			return err
		}
		fmt.Printf("✓ Observability store ready at %s\n", root)
		logging.Info("observability initialized", "path", root)
		return nil
	},
}

var trackAttrs []string
var obsTrackCmd = &cobra.Command{
	Use: "track <name>", Short: "Record a track event", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := Track(args[0], parseAttrs(trackAttrs)); err != nil {
			return err
		}
		fmt.Printf("✓ tracked %s\n", args[0])
		return nil
	},
}

var errorAttrs []string
var obsErrorCmd = &cobra.Command{
	Use: "error <message>", Short: "Record an error event", Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := Error(strings.Join(args, " "), parseAttrs(errorAttrs)); err != nil {
			return err
		}
		fmt.Println("✓ error recorded")
		return nil
	},
}

var obsMetricCmd = &cobra.Command{
	Use: "metric <name> <value>", Short: "Record a metric sample", Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		v, err := strconv.ParseFloat(args[1], 64)
		if err != nil {
			return fmt.Errorf("value must be a number: %w", err)
		}
		if err := Metric(args[0], v); err != nil {
			return err
		}
		fmt.Printf("✓ metric %s=%g\n", args[0], v)
		return nil
	},
}

var tailN int
var tailType string
var obsTailCmd = &cobra.Command{
	Use: "tail", Short: "Show recent events",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		events, err := s.TailEvents(tailN, tailType)
		if err != nil {
			return err
		}
		if len(events) == 0 {
			fmt.Println("(no events)")
			return nil
		}
		for _, e := range events {
			ts := e.Time.Format(time.RFC3339)
			switch e.Type {
			case "track":
				fmt.Printf("%s  TRACK  %s  %v\n", ts, e.Name, e.Attrs)
			case "error":
				fmt.Printf("%s  ERROR  %s  %v\n", ts, e.Message, e.Attrs)
			default:
				fmt.Printf("%s  %-5s  %s\n", ts, strings.ToUpper(e.Type), e.Message)
			}
		}
		return nil
	},
}

var obsMetricsCmd = &cobra.Command{
	Use: "metrics", Short: "Show metric summaries",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		sums, err := s.SummarizeMetrics()
		if err != nil {
			return err
		}
		if len(sums) == 0 {
			fmt.Println("(no metrics)")
			return nil
		}
		fmt.Printf("%-24s %6s %10s %10s %10s %10s\n", "NAME", "COUNT", "MIN", "MAX", "AVG", "LAST")
		for _, m := range sums {
			avg := 0.0
			if m.Count > 0 {
				avg = m.Sum / float64(m.Count)
			}
			fmt.Printf("%-24s %6d %10.2f %10.2f %10.2f %10.2f\n", m.Name, m.Count, m.Min, m.Max, avg, m.Last)
		}
		return nil
	},
}

var obsStatusCmd = &cobra.Command{
	Use: "status", Short: "Dashboard-style summary",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		ev, met, errs, err := s.Counts()
		if err != nil {
			return err
		}
		root, _ := obsRoot()
		fmt.Println("Paradox Observability")
		fmt.Println("=====================")
		fmt.Printf("  Store:    %s\n", root)
		fmt.Printf("  Events:   %d\n", ev)
		fmt.Printf("  Errors:   %d\n", errs)
		fmt.Printf("  Metrics:  %d samples\n", met)
		fmt.Println()
		fmt.Println("  Recent errors:")
		errors, _ := s.TailEvents(5, "error")
		if len(errors) == 0 {
			fmt.Println("    (none)")
		}
		for _, e := range errors {
			fmt.Printf("    %s  %s\n", e.Time.Format("15:04:05"), e.Message)
		}
		return nil
	},
}

func init() {
	obsTrackCmd.Flags().StringArrayVar(&trackAttrs, "attr", nil, "key=value")
	obsErrorCmd.Flags().StringArrayVar(&errorAttrs, "attr", nil, "key=value")
	obsTailCmd.Flags().IntVarP(&tailN, "n", "n", 20, "number of events")
	obsTailCmd.Flags().StringVar(&tailType, "type", "", "filter: track|error|log")
}

func parseAttrs(pairs []string) map[string]any {
	if len(pairs) == 0 {
		return nil
	}
	out := map[string]any{}
	for _, p := range pairs {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) != 2 {
			out[p] = true
			continue
		}
		if n, err := strconv.ParseFloat(kv[1], 64); err == nil {
			out[kv[0]] = n
			continue
		}
		out[kv[0]] = kv[1]
	}
	return out
}

func openStore() (*Store, error) {
	root, err := obsRoot()
	if err != nil {
		return nil, err
	}
	return NewStore(root)
}

func checkObs() (bool, string) {
	root, err := obsRoot()
	if err != nil {
		return false, err.Error()
	}
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return false, "not initialized (run `paradox obs init`)"
		}
		return false, err.Error()
	}
	if !info.IsDir() {
		return false, "not a directory"
	}
	return true, root
}
