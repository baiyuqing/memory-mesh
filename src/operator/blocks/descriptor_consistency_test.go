package blocks_test

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/baiyuqing/ottoplus/src/core/block"

	// Import all block implementations.
	"github.com/baiyuqing/ottoplus/src/operator/blocks/datastore/mysql"
	"github.com/baiyuqing/ottoplus/src/operator/blocks/datastore/postgresql"
	"github.com/baiyuqing/ottoplus/src/operator/blocks/datastore/redis"
	"github.com/baiyuqing/ottoplus/src/operator/blocks/gateway/pgbouncer"
	"github.com/baiyuqing/ottoplus/src/operator/blocks/gateway/proxysql"
	"github.com/baiyuqing/ottoplus/src/operator/blocks/integration/s3-backup"
	"github.com/baiyuqing/ottoplus/src/operator/blocks/integration/slack-notifier"
	"github.com/baiyuqing/ottoplus/src/operator/blocks/integration/stripe"
	"github.com/baiyuqing/ottoplus/src/operator/blocks/networking/ingress"
	"github.com/baiyuqing/ottoplus/src/operator/blocks/observability/health-dashboard"
	"github.com/baiyuqing/ottoplus/src/operator/blocks/observability/log-aggregator"
	"github.com/baiyuqing/ottoplus/src/operator/blocks/observability/metrics-exporter"
	"github.com/baiyuqing/ottoplus/src/operator/blocks/security/mtls"
	"github.com/baiyuqing/ottoplus/src/operator/blocks/security/password-rotation"
	localpv "github.com/baiyuqing/ottoplus/src/operator/blocks/storage/local-pv"
	"github.com/baiyuqing/ottoplus/src/operator/blocks/storage/ebs"

	"gopkg.in/yaml.v3"
)

// blockMDDescriptor mirrors the BLOCK.md YAML frontmatter structure.
type blockMDDescriptor struct {
	Kind        string              `yaml:"kind"`
	Category    string              `yaml:"category"`
	Version     string              `yaml:"version"`
	Description string              `yaml:"description"`
	Ports       []block.Port        `yaml:"ports"`
	Parameters  []block.ParameterSpec `yaml:"parameters"`
	Requires    []string            `yaml:"requires"`
	Provides    []string            `yaml:"provides"`
}

// allBlocks returns every block implementation paired with its BLOCK.md path.
func allBlocks() []struct {
	Block   block.Block
	MDPath  string
} {
	base := findBlocksDir()
	return []struct {
		Block  block.Block
		MDPath string
	}{
		{&postgresql.Block{}, filepath.Join(base, "datastore/postgresql/BLOCK.md")},
		{&mysql.Block{}, filepath.Join(base, "datastore/mysql/BLOCK.md")},
		{&redis.Block{}, filepath.Join(base, "datastore/redis/BLOCK.md")},
		{&pgbouncer.Block{}, filepath.Join(base, "gateway/pgbouncer/BLOCK.md")},
		{&proxysql.Block{}, filepath.Join(base, "gateway/proxysql/BLOCK.md")},
		{&localpv.Block{}, filepath.Join(base, "storage/local-pv/BLOCK.md")},
		{&ebs.Block{}, filepath.Join(base, "storage/ebs/BLOCK.md")},
		{&s3backup.Block{}, filepath.Join(base, "integration/s3-backup/BLOCK.md")},
		{&slacknotifier.Block{}, filepath.Join(base, "integration/slack-notifier/BLOCK.md")},
		{&stripe.Block{}, filepath.Join(base, "integration/stripe/BLOCK.md")},
		{&healthdashboard.Block{}, filepath.Join(base, "observability/health-dashboard/BLOCK.md")},
		{&logaggregator.Block{}, filepath.Join(base, "observability/log-aggregator/BLOCK.md")},
		{&metricsexporter.Block{}, filepath.Join(base, "observability/metrics-exporter/BLOCK.md")},
		{&mtls.Block{}, filepath.Join(base, "security/mtls/BLOCK.md")},
		{&passwordrotation.Block{}, filepath.Join(base, "security/password-rotation/BLOCK.md")},
		{&ingress.Block{}, filepath.Join(base, "networking/ingress/BLOCK.md")},
	}
}

// findBlocksDir locates the src/operator/blocks directory relative to the
// test binary's working directory.
func findBlocksDir() string {
	// Walk up from cwd until we find go.mod.
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "src", "operator", "blocks")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			panic("could not find go.mod in any parent directory")
		}
		dir = parent
	}
}

// parseFrontmatter extracts YAML frontmatter from a BLOCK.md file.
func parseFrontmatter(path string) (blockMDDescriptor, error) {
	f, err := os.Open(path)
	if err != nil {
		return blockMDDescriptor{}, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var lines []string
	inFrontmatter := false

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			if inFrontmatter {
				break // end of frontmatter
			}
			inFrontmatter = true
			continue
		}
		if inFrontmatter {
			lines = append(lines, line)
		}
	}

	var md blockMDDescriptor
	if err := yaml.Unmarshal([]byte(strings.Join(lines, "\n")), &md); err != nil {
		return blockMDDescriptor{}, fmt.Errorf("parse frontmatter in %s: %w", path, err)
	}
	return md, nil
}

func TestBlockMD_MatchesDescriptor(t *testing.T) {
	for _, entry := range allBlocks() {
		desc := entry.Block.Descriptor()
		t.Run(desc.Kind, func(t *testing.T) {
			md, err := parseFrontmatter(entry.MDPath)
			if err != nil {
				t.Fatalf("failed to parse BLOCK.md: %v", err)
			}

			// kind
			if md.Kind != desc.Kind {
				t.Errorf("kind: BLOCK.md=%q, Descriptor=%q", md.Kind, desc.Kind)
			}

			// category
			if md.Category != string(desc.Category) {
				t.Errorf("category: BLOCK.md=%q, Descriptor=%q", md.Category, desc.Category)
			}

			// version
			if md.Version != desc.Version {
				t.Errorf("version: BLOCK.md=%q, Descriptor=%q", md.Version, desc.Version)
			}

			// description
			if md.Description != desc.Description {
				t.Errorf("description: BLOCK.md=%q, Descriptor=%q", md.Description, desc.Description)
			}

			// ports
			if len(md.Ports) != len(desc.Ports) {
				t.Errorf("port count: BLOCK.md=%d, Descriptor=%d", len(md.Ports), len(desc.Ports))
			} else {
				mdPorts := make(map[string]block.Port)
				for _, p := range md.Ports {
					mdPorts[p.Name] = p
				}
				for _, dp := range desc.Ports {
					mp, ok := mdPorts[dp.Name]
					if !ok {
						t.Errorf("port %q in Descriptor but not in BLOCK.md", dp.Name)
						continue
					}
					if mp.PortType != dp.PortType {
						t.Errorf("port %q portType: BLOCK.md=%q, Descriptor=%q", dp.Name, mp.PortType, dp.PortType)
					}
					if mp.Direction != dp.Direction {
						t.Errorf("port %q direction: BLOCK.md=%q, Descriptor=%q", dp.Name, mp.Direction, dp.Direction)
					}
					if mp.Required != dp.Required {
						t.Errorf("port %q required: BLOCK.md=%v, Descriptor=%v", dp.Name, mp.Required, dp.Required)
					}
				}
			}

			// parameters
			if len(md.Parameters) != len(desc.Parameters) {
				t.Errorf("parameter count: BLOCK.md=%d, Descriptor=%d", len(md.Parameters), len(desc.Parameters))
			} else {
				mdParams := make(map[string]block.ParameterSpec)
				for _, p := range md.Parameters {
					mdParams[p.Name] = p
				}
				for _, dp := range desc.Parameters {
					mp, ok := mdParams[dp.Name]
					if !ok {
						t.Errorf("parameter %q in Descriptor but not in BLOCK.md", dp.Name)
						continue
					}
					if mp.Type != dp.Type {
						t.Errorf("param %q type: BLOCK.md=%q, Descriptor=%q", dp.Name, mp.Type, dp.Type)
					}
					if mp.Default != dp.Default {
						t.Errorf("param %q default: BLOCK.md=%q, Descriptor=%q", dp.Name, mp.Default, dp.Default)
					}
					if mp.Required != dp.Required {
						t.Errorf("param %q required: BLOCK.md=%v, Descriptor=%v", dp.Name, mp.Required, dp.Required)
					}
				}
			}

			// requires
			if len(md.Requires) != len(desc.Requires) {
				t.Errorf("requires count: BLOCK.md=%d, Descriptor=%d", len(md.Requires), len(desc.Requires))
			} else {
				for i := range desc.Requires {
					if i < len(md.Requires) && md.Requires[i] != desc.Requires[i] {
						t.Errorf("requires[%d]: BLOCK.md=%q, Descriptor=%q", i, md.Requires[i], desc.Requires[i])
					}
				}
			}

			// provides
			if len(md.Provides) != len(desc.Provides) {
				t.Errorf("provides count: BLOCK.md=%d, Descriptor=%d", len(md.Provides), len(desc.Provides))
			} else {
				for i := range desc.Provides {
					if i < len(md.Provides) && md.Provides[i] != desc.Provides[i] {
						t.Errorf("provides[%d]: BLOCK.md=%q, Descriptor=%q", i, md.Provides[i], desc.Provides[i])
					}
				}
			}
		})
	}
}
