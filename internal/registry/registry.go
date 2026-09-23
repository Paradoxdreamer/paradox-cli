package registry

import (
	"fmt"
	"sort"
	"sync"

	"github.com/spf13/cobra"
)

// Service is the contract every Paradox platform component implements
// to participate in the CLI and (later) the control plane.
//
// Keep this interface small. Add methods only when a second real
// service needs them — never on speculation.
type Service struct {
	// Name is the stable identifier (e.g. "auth", "queue", "deploy").
	Name string

	// Description is a one-line summary shown in help / doctor.
	Description string

	// Commands are optional cobra subcommands this service contributes
	// under the root (or under a group). May be nil.
	Commands []*cobra.Command

	// DoctorChecks are optional health/diagnostic checks.
	// Each returns (ok, detail).
	DoctorChecks []DoctorCheck

	// ConfigSection is the key this service owns inside paradox.yaml
	// (e.g. "auth"). Empty means it uses only global config.
	ConfigSection string
}

// DoctorCheck is a single diagnostic contributed by a service.
type DoctorCheck struct {
	Name string
	Fn   func() (ok bool, detail string)
}

var (
	mu       sync.RWMutex
	services = map[string]*Service{}
)

// Register adds a service. Panics on duplicate name — registration
// happens at init time and duplicates are programming errors.
func Register(s *Service) {
	if s == nil || s.Name == "" {
		panic("registry: service name is required")
	}
	mu.Lock()
	defer mu.Unlock()
	if _, exists := services[s.Name]; exists {
		panic(fmt.Sprintf("registry: service %q already registered", s.Name))
	}
	services[s.Name] = s
}

// Get returns a service by name, or nil.
func Get(name string) *Service {
	mu.RLock()
	defer mu.RUnlock()
	return services[name]
}

// All returns all registered services sorted by name.
func All() []*Service {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]*Service, 0, len(services))
	for _, s := range services {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

// Names returns sorted service names.
func Names() []string {
	all := All()
	names := make([]string, len(all))
	for i, s := range all {
		names[i] = s.Name
	}
	return names
}

// AttachCommands adds every registered service's commands to the given parent.
// Call once from root init after all services have registered.
func AttachCommands(parent *cobra.Command) {
	for _, s := range All() {
		for _, c := range s.Commands {
			parent.AddCommand(c)
		}
	}
}

// DoctorChecks returns a flat list of all service-contributed checks.
func DoctorChecks() []DoctorCheck {
	var out []DoctorCheck
	for _, s := range All() {
		out = append(out, s.DoctorChecks...)
	}
	return out
}
