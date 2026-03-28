// Package block defines the fundamental composable unit of the ottoplus
// platform. Blocks are self-describing, independently testable
// building blocks that can be wired together to form infrastructure stacks.
package block

import "context"

// PortDirection indicates whether a port is an input or an output.
type PortDirection string

const (
	PortInput  PortDirection = "input"
	PortOutput PortDirection = "output"
)

// Port represents a typed connection point on a block. Blocks are wired
// together by matching an output port of one block to an input port of
// another when the PortType values are equal.
type Port struct {
	Name      string        `json:"name"      yaml:"name"`
	PortType  string        `json:"portType"  yaml:"portType"`
	Direction PortDirection `json:"direction" yaml:"direction"`
	Required  bool          `json:"required"  yaml:"required"`
}

// Category classifies blocks into functional groups.
type Category string

const (
	CategoryDatastore     Category = "datastore"
	CategoryCompute       Category = "compute"
	CategoryStorage       Category = "storage"
	CategoryObservability Category = "observability"
	CategorySecurity      Category = "security"
	CategoryNetworking    Category = "networking"
)

// ParameterSpec describes one configurable parameter of a block.
type ParameterSpec struct {
	Name        string `json:"name"        yaml:"name"`
	Type        string `json:"type"        yaml:"type"`
	Default     string `json:"default"     yaml:"default"`
	Description string `json:"description" yaml:"description"`
	Required    bool   `json:"required"    yaml:"required"`
}

// Descriptor is the machine-readable manifest for a block. Every block
// must return a Descriptor. Descriptors carry no runtime state; they are
// pure metadata that AI agents and composition validators inspect.
type Descriptor struct {
	Kind        string          `json:"kind"        yaml:"kind"`
	Category    Category        `json:"category"    yaml:"category"`
	Version     string          `json:"version"     yaml:"version"`
	Description string          `json:"description" yaml:"description"`
	Ports       []Port          `json:"ports"       yaml:"ports"`
	Parameters  []ParameterSpec `json:"parameters"  yaml:"parameters"`
	Requires    []string        `json:"requires"    yaml:"requires"`
	Provides    []string        `json:"provides"    yaml:"provides"`
}

// Block is the domain-level interface. It is infrastructure-agnostic —
// it knows its own shape (Descriptor) and can validate its own config.
// It does NOT know how to create Kubernetes resources; that is the job
// of BlockRuntime in the operator layer.
type Block interface {
	Descriptor() Descriptor
	ValidateParameters(ctx context.Context, parameters map[string]string) error
}

// Phase represents the lifecycle phase of a block instance.
type Phase string

const (
	PhasePending      Phase = "Pending"
	PhaseProvisioning Phase = "Provisioning"
	PhaseReady        Phase = "Ready"
	PhaseDegraded     Phase = "Degraded"
	PhaseFailed       Phase = "Failed"
	PhaseDeleting     Phase = "Deleting"
)

// BlockStatus represents the reconciled status of a single block
// within a composition.
type BlockStatus struct {
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Phase   Phase  `json:"phase"`
	Message string `json:"message"`
}
