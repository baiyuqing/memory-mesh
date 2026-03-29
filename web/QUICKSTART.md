# Workbench Quickstart

The ottoplus workbench is a local browser UI for viewing and editing block compositions.

## Start the workbench

```bash
cd web
npm install
npm start
```

Open [http://localhost:5173](http://localhost:5173) in your browser.

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

The CLI accepts any composition JSON via `--file` — it is not limited to the onboarding sample. Run any command with `--help` for usage details.

## Sample file location

The default composition is loaded from:

```
deploy/examples/sample-composition.json
```

This is the single source of truth. The workbench reads it directly via a Vite alias -- there is no second copy.
