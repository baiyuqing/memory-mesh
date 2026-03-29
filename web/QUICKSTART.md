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

## Sample file location

The default composition is loaded from:

```
deploy/examples/sample-composition.json
```

This is the single source of truth. The workbench reads it directly via a Vite alias -- there is no second copy.
