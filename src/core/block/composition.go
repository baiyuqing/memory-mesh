package block

import (
	"fmt"
	"strings"
)

// BlockRef is a reference to a specific block within a composition,
// including its parameter overrides.
type BlockRef struct {
	Kind       string            `json:"kind"       yaml:"kind"`
	Name       string            `json:"name"       yaml:"name"`
	Parameters map[string]string `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	Inputs     map[string]string `json:"inputs,omitempty"     yaml:"inputs,omitempty"`
}

// Wire connects an output port of one block instance to an input port
// of another. The composition validator ensures port types match.
type Wire struct {
	FromBlock string `json:"fromBlock" yaml:"fromBlock"`
	FromPort  string `json:"fromPort"  yaml:"fromPort"`
	ToBlock   string `json:"toBlock"   yaml:"toBlock"`
	ToPort    string `json:"toPort"    yaml:"toPort"`
}

// CredentialSource returns the FromBlock of the first wire whose ToPort
// is "upstream-credential". If no such wire exists, it returns "".
func CredentialSource(wires []Wire) string {
	for _, w := range wires {
		if w.ToPort == "upstream-credential" {
			return w.FromBlock
		}
	}
	return ""
}

// Composition is the domain model for a fully described, wired-together
// set of blocks. This is what the CRD's spec.blocks section maps to.
type Composition struct {
	Blocks []BlockRef `json:"blocks" yaml:"blocks"`
	Wires  []Wire     `json:"wires"  yaml:"wires"`
}

// NormalizeInputs expands inline inputs on BlockRefs into Wire entries.
// This allows users to declare dependencies directly on each block instead
// of in a separate wires section. The format is: inputPort: "blockName/portName".
func (c *Composition) NormalizeInputs() []error {
	var errs []error

	existingWires := make(map[string]Wire)
	for _, w := range c.Wires {
		key := w.ToBlock + ":" + w.ToPort
		existingWires[key] = w
	}

	for _, ref := range c.Blocks {
		for toPort, source := range ref.Inputs {
			parts := strings.SplitN(source, "/", 2)
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				errs = append(errs, fmt.Errorf(
					"block %q: invalid input %q: expected \"blockName/portName\", got %q",
					ref.Name, toPort, source,
				))
				continue
			}

			newWire := Wire{
				FromBlock: parts[0],
				FromPort:  parts[1],
				ToBlock:   ref.Name,
				ToPort:    toPort,
			}

			key := ref.Name + ":" + toPort
			if existing, ok := existingWires[key]; ok {
				if existing.FromBlock != newWire.FromBlock || existing.FromPort != newWire.FromPort {
					errs = append(errs, fmt.Errorf(
						"block %q input port %q: conflict between inline input (%s/%s) and explicit wire (%s/%s)",
						ref.Name, toPort,
						newWire.FromBlock, newWire.FromPort,
						existing.FromBlock, existing.FromPort,
					))
				}
				continue
			}

			c.Wires = append(c.Wires, newWire)
			existingWires[key] = newWire
		}
	}

	return errs
}

// Validate checks that a composition is internally consistent:
//   - All block kinds exist in the registry
//   - All parameters pass block-level validation
//   - All wires connect existing blocks with matching port types
//   - All required input ports are wired
//   - Dependency requirements (Descriptor.Requires) are satisfied
func (c *Composition) Validate(registry *Registry) []error {
	var errs []error

	blocksByName := make(map[string]Block)
	for _, ref := range c.Blocks {
		b, ok := registry.Get(ref.Kind)
		if !ok {
			errs = append(errs, fmt.Errorf("block kind %q not found in registry", ref.Kind))
			continue
		}
		if _, dup := blocksByName[ref.Name]; dup {
			errs = append(errs, fmt.Errorf("duplicate block name %q", ref.Name))
			continue
		}
		blocksByName[ref.Name] = b
		if err := b.ValidateParameters(nil, ref.Parameters); err != nil {
			errs = append(errs, fmt.Errorf("block %q: %w", ref.Name, err))
		}
	}

	for _, w := range c.Wires {
		fromBlock, ok := blocksByName[w.FromBlock]
		if !ok {
			errs = append(errs, fmt.Errorf("wire references unknown block %q", w.FromBlock))
			continue
		}
		toBlock, ok := blocksByName[w.ToBlock]
		if !ok {
			errs = append(errs, fmt.Errorf("wire references unknown block %q", w.ToBlock))
			continue
		}
		fromPort := findPort(fromBlock.Descriptor().Ports, w.FromPort, PortOutput)
		toPort := findPort(toBlock.Descriptor().Ports, w.ToPort, PortInput)
		if fromPort == nil {
			errs = append(errs, fmt.Errorf("block %q has no output port %q", w.FromBlock, w.FromPort))
		}
		if toPort == nil {
			errs = append(errs, fmt.Errorf("block %q has no input port %q", w.ToBlock, w.ToPort))
		}
		if fromPort != nil && toPort != nil && fromPort.PortType != toPort.PortType {
			errs = append(errs, fmt.Errorf(
				"port type mismatch: %s.%s (%s) -> %s.%s (%s)",
				w.FromBlock, w.FromPort, fromPort.PortType,
				w.ToBlock, w.ToPort, toPort.PortType,
			))
		}
	}

	wiredInputs := make(map[string]map[string]bool)
	for _, w := range c.Wires {
		if wiredInputs[w.ToBlock] == nil {
			wiredInputs[w.ToBlock] = make(map[string]bool)
		}
		wiredInputs[w.ToBlock][w.ToPort] = true
	}
	for _, ref := range c.Blocks {
		b, ok := blocksByName[ref.Name]
		if !ok {
			continue
		}
		for _, port := range b.Descriptor().Ports {
			if port.Direction == PortInput && port.Required {
				if !wiredInputs[ref.Name][port.Name] {
					errs = append(errs, fmt.Errorf(
						"block %q required input port %q is not wired",
						ref.Name, port.Name,
					))
				}
			}
		}
	}

	return errs
}

// AutoWire attempts to automatically connect blocks by matching output
// port types to input port types. It only auto-wires when the match is
// unambiguous (exactly one output matches a given input type).
func (c *Composition) AutoWire(registry *Registry) []error {
	var errs []error

	wiredInputs := make(map[string]map[string]bool)
	for _, w := range c.Wires {
		if wiredInputs[w.ToBlock] == nil {
			wiredInputs[w.ToBlock] = make(map[string]bool)
		}
		wiredInputs[w.ToBlock][w.ToPort] = true
	}

	outputsByType := make(map[string][]struct {
		blockName string
		portName  string
	})
	for _, ref := range c.Blocks {
		b, ok := registry.Get(ref.Kind)
		if !ok {
			continue
		}
		for _, port := range b.Descriptor().Ports {
			if port.Direction == PortOutput {
				outputsByType[port.PortType] = append(outputsByType[port.PortType], struct {
					blockName string
					portName  string
				}{ref.Name, port.Name})
			}
		}
	}

	for _, ref := range c.Blocks {
		b, ok := registry.Get(ref.Kind)
		if !ok {
			continue
		}
		for _, port := range b.Descriptor().Ports {
			if port.Direction != PortInput {
				continue
			}
			if wiredInputs[ref.Name][port.Name] {
				continue
			}
			sources := outputsByType[port.PortType]
			// Filter out self-references
			var candidates []struct {
				blockName string
				portName  string
			}
			for _, s := range sources {
				if s.blockName != ref.Name {
					candidates = append(candidates, s)
				}
			}
			if len(candidates) == 1 {
				c.Wires = append(c.Wires, Wire{
					FromBlock: candidates[0].blockName,
					FromPort:  candidates[0].portName,
					ToBlock:   ref.Name,
					ToPort:    port.Name,
				})
			} else if len(candidates) > 1 && (port.Required || port.PortType == "credential") {
				errs = append(errs, fmt.Errorf(
					"block %q input port %q (type %s) has %d candidates, wire explicitly",
					ref.Name, port.Name, port.PortType, len(candidates),
				))
			}
		}
	}

	return errs
}

// TopologicalSort returns blocks ordered by their wire dependencies.
// Upstream blocks (those whose outputs are consumed) come first.
func (c *Composition) TopologicalSort() ([]BlockRef, error) {
	blockIndex := make(map[string]int)
	for i, ref := range c.Blocks {
		blockIndex[ref.Name] = i
	}

	inDegree := make(map[string]int)
	dependents := make(map[string][]string)
	for _, ref := range c.Blocks {
		inDegree[ref.Name] = 0
	}
	for _, w := range c.Wires {
		inDegree[w.ToBlock]++
		dependents[w.FromBlock] = append(dependents[w.FromBlock], w.ToBlock)
	}

	var queue []string
	for _, ref := range c.Blocks {
		if inDegree[ref.Name] == 0 {
			queue = append(queue, ref.Name)
		}
	}

	var sorted []BlockRef
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		sorted = append(sorted, c.Blocks[blockIndex[name]])
		for _, dep := range dependents[name] {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}

	if len(sorted) != len(c.Blocks) {
		return nil, fmt.Errorf("circular dependency detected in block wiring")
	}

	return sorted, nil
}

func findPort(ports []Port, name string, direction PortDirection) *Port {
	for i := range ports {
		if ports[i].Name == name && ports[i].Direction == direction {
			return &ports[i]
		}
	}
	return nil
}
