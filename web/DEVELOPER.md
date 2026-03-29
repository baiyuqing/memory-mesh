# Workbench Developer Guide

## Architecture

The workbench is a Vite + React + TypeScript single-page app in `web/`.

### Sample composition (single source of truth)

The onboarding 3-block composition lives at:

```
deploy/examples/sample-composition.json
```

The workbench imports this file directly via a Vite resolve alias:

```
@examples  ->  ../deploy/examples
```

Configured in `web/vite.config.ts` (resolve.alias + server.fs.allow). There is no copy of this data in the frontend -- the page reads the repo file at build time.

### What happens if you modify the sample file

| Change | Effect |
|--------|--------|
| Edit block parameters | Workbench default values update on next `npm start` / build |
| Add/remove a block | Workbench canvas and results reflect the change |
| Rename the file | Build breaks -- update `web/src/examples.d.ts` and import in `App.tsx` |

### Files

| File | Role |
|------|------|
| `deploy/examples/sample-composition.json` | Single source: 3-block onboarding composition |
| `web/src/App.tsx` | Main component: imports sample, renders all 4 panels |
| `web/src/App.css` | Layout and styling |
| `web/src/examples.d.ts` | TypeScript declaration for the `@examples` alias |
| `web/vite.config.ts` | Vite config with alias and fs.allow |
| `web/src/composition.test.ts` | Frontend test: verifies sample structure |

### Block field metadata

`web/src/App.tsx` contains `blockFieldMeta` -- per-block-kind field definitions for the 3 onboarding blocks. This drives the right panel: labels, input types, required markers, descriptions, and defaults. If you add a new block kind, add a corresponding entry here.

## Tests

### Frontend test

```bash
cd web
npm test
```

Runs `vitest` against `web/src/composition.test.ts`. This test imports `deploy/examples/sample-composition.json` through the same Vite alias the app uses and verifies:

- The file loads and has the expected structure
- Contains exactly the 3 onboarding blocks (storage, db, pooler)
- Block kinds match (storage.local-pv, datastore.postgresql, gateway.pgbouncer)
- Input wiring is correct (storage -> db -> pooler)

This is the only test that directly anchors the onboarding sample (`sample-composition.json`).

### Go tests

```bash
make ci-local
```

The Go-side tests in `deploy/examples/`, `src/core/compiler/`, `src/api/`, and `src/operator/reconciler/` exercise the **standard** compositions (`standard-composition.json` / `LoadStandardCompositionJSON`), not this onboarding sample. They serve as project-level regression checks but do not directly validate the 3-block onboarding path.

## Build

```bash
cd web
npm run build    # TypeScript check + Vite production build -> web/dist/
```
