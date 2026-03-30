# Workbench Quickstart

The ottoplus workbench is a local browser UI for viewing and editing block compositions.

## Start the workbench (recommended)

From the repo root:

```bash
make workbench
```

This starts both the API server (`:8080`) and the workbench (`:5173`) together. All credential-source surfaces are available by default. Open [http://localhost:5173](http://localhost:5173) in your browser.

### Browser only (no API)

```bash
cd web && npm install && npm start
```

The workbench loads but credential-source badges show as unavailable without the API.

## What you see

The page loads the **onboarding sample composition** from `deploy/examples/sample-composition.json`. This is the 3-block path:

```
local-pv  ->  postgresql  ->  pgbouncer
(storage)     (db)            (pooler)
```

The four-panel layout shows:

| Area | What it shows |
|------|--------------|
| Left sidebar | Block catalog + composition block list with delete/restore |
| Center canvas | Pipeline visualization with blocks and wires |
| Right panel | Selected block details, parameters (editable), inputs |
| Bottom results | Generated Output (JSON/YAML), Validation, Topology & Wires |

## Try editing

1. **Select a block** -- click any block card on the canvas or in the sidebar
2. **Edit a parameter** -- change `size`, `version`, or `replicas` in the right panel
3. **Delete a block** -- click the x button next to a block in the sidebar
4. **Restore a block** -- click the + button to bring it back
5. **Switch output format** -- toggle between JSON and YAML in the bottom-left panel

All changes are reflected immediately across all panels. No page refresh needed.

## CLI alternative

If you prefer the terminal over the browser, you can inspect the same sample composition with the CLI:

```bash
# List all registered blocks
go run ./cmd/ottoplus blocks list

# Validate the onboarding sample
go run ./cmd/ottoplus compose validate --file deploy/examples/sample-composition.json

# Show auto-wired connections
go run ./cmd/ottoplus compose auto-wire --file deploy/examples/sample-composition.json

# Show topological order and wires
go run ./cmd/ottoplus compose topology --file deploy/examples/sample-composition.json
```

All commands support `--format json` for machine-readable output (default is human-readable table):

```bash
go run ./cmd/ottoplus blocks list --format json
go run ./cmd/ottoplus compose validate --file deploy/examples/sample-composition.json --format json
go run ./cmd/ottoplus compose auto-wire --file deploy/examples/sample-composition.json --format json
go run ./cmd/ottoplus compose topology --file deploy/examples/sample-composition.json --format json
```

The CLI accepts any composition JSON via `--file` — it is not limited to the onboarding sample. Run any command with `--help` for usage details.

## Credential sources

The workbench topology panel shows which block provides the upstream credential for each consumer. This is derived from the API (`POST /v1/compositions/topology`) and reflects the same compiled wire truth as the CLI and API.

| Path | Credential source | Why |
|------|-------------------|-----|
| Sample (3-block) | `pooler <- db` | No explicit `upstream-credential` wire — the compiler auto-wires it from `db`'s `credential` output |
| Standard (4-block) | `pooler <- rotator` | The pooler's `upstream-credential` input is explicitly wired to `rotator/credential` |

To see credential badges in the workbench, use `make workbench` (recommended) which starts both API and frontend together. If you started with the browser-only path, the header pill shows "API unavailable" and credential badges are not displayed.

## Sample file location

The default composition is loaded from:

```
deploy/examples/sample-composition.json
```

This is the single source of truth. The workbench reads it directly via a Vite alias -- there is no second copy.
