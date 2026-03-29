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

const rootUsage = `Usage: ottoplus <command> [options]

Commands:
  blocks     List registered blocks
  compose    Validate, auto-wire, or inspect compositions

Run 'ottoplus <command> --help' for details on a specific command.
`

const blocksUsage = `Usage: ottoplus blocks <subcommand>

Subcommands:
  list    List all registered blocks (category, name, kind, description)

Flags:
  --format <table|json>    Output format (default: table)
`

const composeUsage = `Usage: ottoplus compose <subcommand> --file <path>

Subcommands:
  validate    Validate a composition file against the block registry
  auto-wire   Auto-wire a composition and show the resulting wire table
  topology    Show topological block order and wires

Flags:
  --file <path>    Path to a composition JSON file (required)
`

// Run dispatches to the appropriate subcommand based on args.
func Run(args []string) error {
	return run(args, os.Stdout)
}

func run(args []string, w io.Writer) error {
	if len(args) == 0 {
		fmt.Fprint(w, rootUsage)
		return nil
	}

	switch args[0] {
	case "--help", "-h", "help":
		fmt.Fprint(w, rootUsage)
		return nil
	case "blocks":
		return runBlocks(args[1:], w)
	case "compose":
		return runCompose(args[1:], w)
	default:
		fmt.Fprint(w, rootUsage)
		return fmt.Errorf("unknown command %q — available commands: blocks, compose", args[0])
	}
}

func runBlocks(args []string, w io.Writer) error {
	if len(args) == 0 {
		fmt.Fprint(w, blocksUsage)
		return fmt.Errorf("missing subcommand — run 'ottoplus blocks --help'")
	}

	switch args[0] {
	case "--help", "-h", "help":
		fmt.Fprint(w, blocksUsage)
		return nil
	case "list":
		format := "table"
		for i, a := range args[1:] {
			if a == "--help" || a == "-h" || a == "-help" {
				fmt.Fprint(w, "Usage: ottoplus blocks list [--format <table|json>]\n\nFlags:\n  --format <table|json>    Output format (default: table)\n")
				return nil
			}
			if a == "--format" && i+1 < len(args[1:]) {
				format = args[1:][i+1]
			}
		}
		if format != "table" && format != "json" {
			return fmt.Errorf("unsupported format %q — available: table, json", format)
		}
		registry := newRegistry()
		if format == "json" {
			return blocksListJSON(registry, w)
		}
		return blocksList(registry, w)
	default:
		fmt.Fprint(w, blocksUsage)
		return fmt.Errorf("unknown blocks subcommand %q — available: list", args[0])
	}
}

func runCompose(args []string, w io.Writer) error {
	if len(args) == 0 {
		fmt.Fprint(w, composeUsage)
		return fmt.Errorf("missing subcommand — run 'ottoplus compose --help'")
	}

	sub := args[0]

	switch sub {
	case "--help", "-h", "help":
		fmt.Fprint(w, composeUsage)
		return nil
	case "validate", "auto-wire", "topology":
		// continue below
	default:
		fmt.Fprint(w, composeUsage)
		return fmt.Errorf("unknown compose subcommand %q — available: validate, auto-wire, topology", sub)
	}

	// Intercept --help/-h before flag.Parse to avoid flag.ErrHelp error path.
	for _, a := range args[1:] {
		if a == "--help" || a == "-h" || a == "-help" {
			fmt.Fprintf(w, "Usage: ottoplus compose %s --file <path>\n\n", sub)
			fmt.Fprintf(w, "Flags:\n  --file <path>    Path to a composition JSON file (required)\n")
			return nil
		}
	}

	fs := flag.NewFlagSet("ottoplus compose "+sub, flag.ContinueOnError)
	fs.SetOutput(w)
	filePath := fs.String("file", "", "Path to composition JSON file (required)")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *filePath == "" {
		fmt.Fprintf(w, "Error: --file is required\n\n")
		fmt.Fprintf(w, "Usage: ottoplus compose %s --file <path>\n", sub)
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
	"storage.local-pv":           "Local PV",
	"datastore.postgresql":       "PostgreSQL",
	"gateway.pgbouncer":          "PgBouncer",
	"security.password-rotation": "Password Rotation",
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

type blockEntry struct {
	Category    string `json:"category"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Description string `json:"description"`
}

func blocksListJSON(registry *block.Registry, w io.Writer) error {
	descriptors := registry.List()
	sort.Slice(descriptors, func(i, j int) bool {
		if descriptors[i].Category != descriptors[j].Category {
			return descriptors[i].Category < descriptors[j].Category
		}
		return descriptors[i].Kind < descriptors[j].Kind
	})

	entries := make([]blockEntry, len(descriptors))
	for i, d := range descriptors {
		entries[i] = blockEntry{
			Category:    string(d.Category),
			Name:        displayName(d.Kind),
			Kind:        d.Kind,
			Description: d.Description,
		}
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(entries)
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
