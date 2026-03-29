// Package cli implements the ottoplus command-line interface.
// It provides subcommands for listing blocks and validating compositions.
package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/baiyuqing/ottoplus/src/core/block"
	"github.com/baiyuqing/ottoplus/src/operator/blocks/datastore/postgresql"
	"github.com/baiyuqing/ottoplus/src/operator/blocks/gateway/pgbouncer"
	passwordrotation "github.com/baiyuqing/ottoplus/src/operator/blocks/security/password-rotation"
	localpv "github.com/baiyuqing/ottoplus/src/operator/blocks/storage/local-pv"
)

const usage = `Usage: ottoplus <command> [options]

Commands:
  blocks list                        List all registered blocks
  compose validate --file <path>     Validate a composition file
  compose auto-wire --file <path>    Auto-wire a composition and show wires
  compose topology --file <path>     Show topological block order and wires
`

// Run dispatches to the appropriate subcommand based on args.
func Run(args []string) error {
	return run(args, os.Stdout)
}

func run(args []string, w io.Writer) error {
	if len(args) == 0 {
		fmt.Fprint(w, usage)
		return nil
	}

	switch args[0] {
	case "blocks":
		return runBlocks(args[1:], w)
	case "compose":
		return runCompose(args[1:], w)
	default:
		fmt.Fprint(w, usage)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runBlocks(args []string, w io.Writer) error {
	if len(args) == 0 || args[0] != "list" {
		return fmt.Errorf("usage: ottoplus blocks list")
	}
	registry := newRegistry()
	return blocksList(registry, w)
}

func runCompose(args []string, w io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: ottoplus compose <validate|auto-wire|topology> --file <path>")
	}

	sub := args[0]
	fs := flag.NewFlagSet("compose "+sub, flag.ContinueOnError)
	filePath := fs.String("file", "", "Path to composition JSON file")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *filePath == "" {
		return fmt.Errorf("--file is required")
	}

	registry := newRegistry()

	switch sub {
	case "validate":
		return composeValidate(registry, *filePath, w)
	case "auto-wire":
		return composeAutoWire(registry, *filePath, w)
	case "topology":
		return composeTopology(registry, *filePath, w)
	default:
		return fmt.Errorf("unknown compose subcommand %q", sub)
	}
}

func newRegistry() *block.Registry {
	registry := block.NewRegistry()
	for _, b := range []block.Block{
		&localpv.Block{},
		&postgresql.Block{},
		&pgbouncer.Block{},
		&passwordrotation.Block{},
	} {
		_ = registry.Register(b)
	}
	return registry
}

// compositionFile mirrors the JSON composition file format.
type compositionFile struct {
	Composition struct {
		Blocks []block.BlockRef `json:"blocks"`
	} `json:"composition"`
}

// blockDisplayNames maps block kinds to their canonical human-readable names.
var blockDisplayNames = map[string]string{
	"storage.local-pv":            "Local PV",
	"datastore.postgresql":        "PostgreSQL",
	"gateway.pgbouncer":           "PgBouncer",
	"security.password-rotation":  "Password Rotation",
}

// displayName returns the canonical human-readable name for a block kind.
func displayName(kind string) string {
	if name, ok := blockDisplayNames[kind]; ok {
		return name
	}
	// Fallback for unknown kinds: title-case the suffix.
	parts := strings.SplitN(kind, ".", 2)
	name := kind
	if len(parts) == 2 {
		name = parts[1]
	}
	words := strings.Split(name, "-")
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

func blocksList(registry *block.Registry, w io.Writer) error {
	descriptors := registry.List()
	sort.Slice(descriptors, func(i, j int) bool {
		if descriptors[i].Category != descriptors[j].Category {
			return descriptors[i].Category < descriptors[j].Category
		}
		return descriptors[i].Kind < descriptors[j].Kind
	})

	fmt.Fprintf(w, "%-14s  %-22s  %-26s  %s\n", "CATEGORY", "NAME", "KIND", "DESCRIPTION")
	for _, d := range descriptors {
		desc := d.Description
		if len(desc) > 40 {
			desc = desc[:37] + "..."
		}
		fmt.Fprintf(w, "%-14s  %-22s  %-26s  %s\n", d.Category, displayName(d.Kind), d.Kind, desc)
	}
	return nil
}

func composeValidate(registry *block.Registry, filePath string, w io.Writer) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("cannot read file: %w", err)
	}

	var doc compositionFile
	if err := json.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	comp := block.Composition{Blocks: doc.Composition.Blocks}

	if errs := comp.NormalizeInputs(); len(errs) > 0 {
		return reportErrors(w, filePath, errs)
	}

	if errs := comp.Validate(registry); len(errs) > 0 {
		return reportErrors(w, filePath, errs)
	}

	fmt.Fprintf(w, "ok  %s (%d blocks)\n", filePath, len(comp.Blocks))
	return nil
}

func composeAutoWire(registry *block.Registry, filePath string, w io.Writer) error {
	comp, err := loadComposition(filePath)
	if err != nil {
		return err
	}

	if errs := comp.NormalizeInputs(); len(errs) > 0 {
		return reportErrors(w, filePath, errs)
	}

	if errs := comp.AutoWire(registry); len(errs) > 0 {
		return reportErrors(w, filePath, errs)
	}

	fmt.Fprintf(w, "ok  %s (%d blocks, %d wires)\n\n", filePath, len(comp.Blocks), len(comp.Wires))
	if len(comp.Wires) == 0 {
		fmt.Fprintln(w, "No wires.")
		return nil
	}

	fmt.Fprintf(w, "%-20s  %-16s  %-20s  %-16s\n", "FROM BLOCK", "PORT", "TO BLOCK", "PORT")
	for _, wire := range comp.Wires {
		fmt.Fprintf(w, "%-20s  %-16s  %-20s  %-16s\n",
			wire.FromBlock, wire.FromPort, wire.ToBlock, wire.ToPort)
	}
	return nil
}

func composeTopology(registry *block.Registry, filePath string, w io.Writer) error {
	comp, err := loadComposition(filePath)
	if err != nil {
		return err
	}

	if errs := comp.NormalizeInputs(); len(errs) > 0 {
		return reportErrors(w, filePath, errs)
	}

	if errs := comp.AutoWire(registry); len(errs) > 0 {
		return reportErrors(w, filePath, errs)
	}

	sorted, err := comp.TopologicalSort()
	if err != nil {
		return fmt.Errorf("topology: %w", err)
	}

	fmt.Fprintf(w, "ok  %s (%d blocks)\n\n", filePath, len(sorted))
	fmt.Fprintln(w, "Topological order:")
	for i, ref := range sorted {
		fmt.Fprintf(w, "  %d. %s (%s)\n", i+1, ref.Name, ref.Kind)
	}

	if len(comp.Wires) > 0 {
		fmt.Fprintf(w, "\nWires (%d):\n", len(comp.Wires))
		for _, wire := range comp.Wires {
			fmt.Fprintf(w, "  %s/%s -> %s/%s\n",
				wire.FromBlock, wire.FromPort, wire.ToBlock, wire.ToPort)
		}
	}
	return nil
}

func loadComposition(filePath string) (*block.Composition, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot read file: %w", err)
	}

	var doc compositionFile
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	comp := &block.Composition{Blocks: doc.Composition.Blocks}
	return comp, nil
}

func reportErrors(w io.Writer, filePath string, errs []error) error {
	msgs := make([]string, len(errs))
	for i, e := range errs {
		msgs[i] = e.Error()
	}
	fmt.Fprintf(w, "FAIL  %s\n", filePath)
	for _, msg := range msgs {
		fmt.Fprintf(w, "  - %s\n", msg)
	}
	return fmt.Errorf("validation failed: %s", strings.Join(msgs, "; "))
}
