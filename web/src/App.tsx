import { useState, useMemo } from 'react'
import sampleComposition from '@examples/sample-composition.json'
import './App.css'

interface BlockRef {
  kind: string
  name: string
  parameters?: Record<string, string>
  inputs?: Record<string, string>
}

const initialBlocks: BlockRef[] = sampleComposition.composition.blocks.map(b => ({
  ...b,
  parameters: b.parameters ? { ...b.parameters } : undefined,
  inputs: b.inputs ? { ...b.inputs } : undefined,
}))

function categoryOf(kind: string): string {
  return kind.split('.')[0]
}

function dotClass(kind: string): string {
  return `sidebar-dot ${categoryOf(kind)}`
}

function getWires(blocks: BlockRef[]): { from: string; to: string; port: string }[] {
  const wires: { from: string; to: string; port: string }[] = []
  const nameSet = new Set(blocks.map(b => b.name))
  for (const b of blocks) {
    if (!b.inputs) continue
    for (const [port, ref] of Object.entries(b.inputs)) {
      const [fromBlock, fromPort] = ref.split('/')
      if (!nameSet.has(fromBlock)) continue
      wires.push({ from: `${fromBlock}/${fromPort}`, to: `${b.name}/${port}`, port })
    }
  }
  return wires
}

function topoSort(blocks: BlockRef[]): BlockRef[] {
  const byName = new Map(blocks.map(b => [b.name, b]))
  const visited = new Set<string>()
  const result: BlockRef[] = []

  function visit(name: string) {
    if (visited.has(name)) return
    visited.add(name)
    const b = byName.get(name)
    if (!b) return
    if (b.inputs) {
      for (const ref of Object.values(b.inputs)) {
        visit(ref.split('/')[0])
      }
    }
    result.push(b)
  }

  for (const b of blocks) visit(b.name)
  return result
}

// Minimal YAML serializer for composition output
function toYaml(obj: unknown, indent = 0): string {
  const pad = '  '.repeat(indent)
  if (obj === null || obj === undefined) return `${pad}~`
  if (typeof obj === 'string') return obj.includes(':') || obj.includes('#') || obj.includes("'")
    ? `"${obj}"` : obj
  if (typeof obj === 'number' || typeof obj === 'boolean') return String(obj)
  if (Array.isArray(obj)) {
    if (obj.length === 0) return '[]'
    return obj.map(item => {
      if (typeof item === 'object' && item !== null) {
        const entries = Object.entries(item)
        const first = entries[0]
        const rest = entries.slice(1)
        let line = `${pad}- ${first[0]}: ${toYaml(first[1], indent + 2)}`
        for (const [k, v] of rest) {
          if (typeof v === 'object' && v !== null) {
            line += `\n${pad}  ${k}:\n${toYaml(v, indent + 3)}`
          } else {
            line += `\n${pad}  ${k}: ${toYaml(v, indent + 2)}`
          }
        }
        return line
      }
      return `${pad}- ${toYaml(item, indent + 1)}`
    }).join('\n')
  }
  if (typeof obj === 'object') {
    const entries = Object.entries(obj)
    if (entries.length === 0) return '{}'
    return entries.map(([k, v]) => {
      if (typeof v === 'object' && v !== null) {
        return `${pad}${k}:\n${toYaml(v, indent + 1)}`
      }
      return `${pad}${k}: ${toYaml(v, indent + 1)}`
    }).join('\n')
  }
  return String(obj)
}

// Registry of known block categories for sidebar display
const categories = [
  { name: 'Storage', blocks: [{ kind: 'storage.local-pv', label: 'Local PV' }, { kind: 'storage.ebs', label: 'EBS' }] },
  { name: 'Datastore', blocks: [{ kind: 'datastore.postgresql', label: 'PostgreSQL' }, { kind: 'datastore.mysql', label: 'MySQL' }, { kind: 'datastore.redis', label: 'Redis' }] },
  { name: 'Security', blocks: [{ kind: 'security.password-rotation', label: 'Password Rotation' }, { kind: 'security.mtls', label: 'mTLS' }] },
  { name: 'Gateway', blocks: [{ kind: 'gateway.pgbouncer', label: 'PgBouncer' }, { kind: 'gateway.proxysql', label: 'ProxySQL' }] },
  { name: 'Observability', blocks: [{ kind: 'observability.metrics-exporter', label: 'Metrics Exporter' }] },
  { name: 'Integration', blocks: [{ kind: 'integration.s3-backup', label: 'S3 Backup' }] },
]

type OutputFormat = 'json' | 'yaml'

function App() {
  const [blocks, setBlocks] = useState<BlockRef[]>(initialBlocks)
  const [deletedNames, setDeletedNames] = useState<Set<string>>(new Set())
  const [selectedName, setSelectedName] = useState<string | null>(null)
  const [outputFormat, setOutputFormat] = useState<OutputFormat>('json')

  // Single resolved composition: filter deleted blocks, strip dangling inputs
  const currentBlocks = useMemo(() => {
    const live = blocks.filter(b => !deletedNames.has(b.name))
    const nameSet = new Set(live.map(b => b.name))
    return live.map(b => {
      if (!b.inputs) return b
      const cleaned: Record<string, string> = {}
      for (const [port, ref] of Object.entries(b.inputs)) {
        if (nameSet.has(ref.split('/')[0])) cleaned[port] = ref
      }
      return Object.keys(cleaned).length > 0
        ? { ...b, inputs: cleaned }
        : { ...b, inputs: undefined }
    })
  }, [blocks, deletedNames])

  const sorted = useMemo(() => topoSort(currentBlocks), [currentBlocks])
  const wires = useMemo(() => getWires(currentBlocks), [currentBlocks])
  const activeKinds = useMemo(() => new Set(currentBlocks.map(b => b.kind)), [currentBlocks])

  const compositionData = useMemo(() => ({ composition: { blocks: currentBlocks } }), [currentBlocks])
  const jsonOutput = useMemo(() => JSON.stringify(compositionData, null, 2), [compositionData])
  const yamlOutput = useMemo(() => toYaml(compositionData), [compositionData])

  const selectedBlock = blocks.find(b => b.name === selectedName) ?? null
  const isSelectedDeleted = selectedName !== null && deletedNames.has(selectedName)

  function updateParam(blockName: string, key: string, value: string) {
    setBlocks(prev => prev.map(b => {
      if (b.name !== blockName || !b.parameters) return b
      return { ...b, parameters: { ...b.parameters, [key]: value } }
    }))
  }

  function toggleDelete(blockName: string) {
    setDeletedNames(prev => {
      const next = new Set(prev)
      if (next.has(blockName)) {
        next.delete(blockName)
      } else {
        next.add(blockName)
      }
      return next
    })
  }

  function handleCardClick(name: string) {
    setSelectedName(prev => prev === name ? null : name)
  }

  return (
    <div className="app">
      {/* Header */}
      <header className="header">
        <div className="header-logo">otto<span>plus</span></div>
        <div className="header-sep" />
        <div className="header-label">Workbench</div>
      </header>

      {/* Left: Block catalog */}
      <aside className="sidebar">
        {categories.map(cat => (
          <div className="sidebar-section" key={cat.name}>
            <div className="sidebar-title">{cat.name}</div>
            {cat.blocks.map(b => (
              <div className={`sidebar-item ${activeKinds.has(b.kind) ? 'active' : ''}`} key={b.kind}>
                <div className={dotClass(b.kind)} />
                <div>
                  <div>{b.label}</div>
                  <div className="sidebar-kind">{b.kind}</div>
                </div>
              </div>
            ))}
          </div>
        ))}

        <div className="sidebar-section">
          <div className="sidebar-title">All Blocks</div>
          {blocks.map(b => {
            const isDeleted = deletedNames.has(b.name)
            return (
              <div
                className={`sidebar-block-item ${isDeleted ? 'deleted' : ''} ${selectedName === b.name ? 'selected' : ''}`}
                key={b.name}
                onClick={() => handleCardClick(b.name)}
              >
                <div className={dotClass(b.kind)} />
                <span className="sidebar-block-name">{b.name}</span>
                <button
                  className={`sidebar-block-action ${isDeleted ? 'restore' : 'delete'}`}
                  onClick={(e) => { e.stopPropagation(); toggleDelete(b.name) }}
                  title={isDeleted ? 'Restore block' : 'Remove block'}
                >
                  {isDeleted ? '+' : '\u00d7'}
                </button>
              </div>
            )
          })}
        </div>
      </aside>

      {/* Center: Canvas */}
      <main className="canvas" onClick={() => setSelectedName(null)}>
        <div className="canvas-title">Composition Pipeline</div>
        {currentBlocks.length === 0 ? (
          <div className="canvas-empty">No blocks in composition. Restore blocks from the sidebar.</div>
        ) : (
          <div className="pipeline">
            {sorted.map((b, i) => {
              const incomingWire = i > 0 && b.inputs
                ? Object.entries(b.inputs).find(([, ref]) => ref.split('/')[0] === sorted[i - 1].name)
                : null

              return (
                <div key={b.name} style={{ display: 'flex', alignItems: 'center' }}>
                  {i > 0 && (
                    <div className="wire">
                      <div className="wire-label">{incomingWire ? incomingWire[1] : ''}</div>
                      <div className="wire-line" />
                    </div>
                  )}
                  <div
                    className={`block-card ${selectedName === b.name ? 'selected' : ''}`}
                    onClick={(e) => { e.stopPropagation(); handleCardClick(b.name) }}
                  >
                    <div className="block-card-name">{b.name}</div>
                    <div className="block-card-kind">{b.kind}</div>
                    {b.parameters && (
                      <div className="block-card-params">
                        {Object.entries(b.parameters).map(([k, v]) => (
                          <span className="param-tag" key={k}>{k}={v}</span>
                        ))}
                      </div>
                    )}
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </main>

      {/* Right: Block details */}
      <aside className="params">
        <div className="params-title">Block Details</div>
        {selectedBlock ? (
          <div className={`params-block ${isSelectedDeleted ? 'params-block-deleted' : ''}`}>
            <div className="params-block-header">
              <div className="params-block-name">{selectedBlock.name}</div>
              <button
                className={`params-block-toggle ${isSelectedDeleted ? 'restore' : 'delete'}`}
                onClick={() => toggleDelete(selectedBlock.name)}
              >
                {isSelectedDeleted ? 'Restore' : 'Remove'}
              </button>
            </div>
            <div className="params-block-kind">{selectedBlock.kind}</div>
            {isSelectedDeleted && (
              <div className="params-deleted-badge">Removed from composition</div>
            )}
            {selectedBlock.parameters && !isSelectedDeleted && (
              <>
                <div className="params-inputs-title">Parameters</div>
                {Object.entries(selectedBlock.parameters).map(([k, v]) => (
                  <div className="params-row" key={k}>
                    <span className="params-key">{k}</span>
                    <input
                      className="params-input"
                      value={v}
                      onChange={(e) => updateParam(selectedBlock.name, k, e.target.value)}
                    />
                  </div>
                ))}
              </>
            )}
            {selectedBlock.inputs && !isSelectedDeleted && (
              <>
                <div className="params-inputs-title">Inputs</div>
                {Object.entries(selectedBlock.inputs).map(([k, v]) => (
                  <div className="params-row" key={k}>
                    <span className="params-key">{k}</span>
                    <span className="params-val">{v}</span>
                  </div>
                ))}
              </>
            )}
          </div>
        ) : (
          <div className="params-empty">Select a block to view details</div>
        )}
      </aside>

      {/* Bottom: Results & explanation */}
      <section className="results">
        <div className="results-panels">
          {/* Generated Output */}
          <div className="results-card results-output">
            <div className="results-card-header">
              <div className="results-card-title">Generated Output</div>
              <div className="results-tabs">
                <button
                  className={`results-tab ${outputFormat === 'json' ? 'active' : ''}`}
                  onClick={() => setOutputFormat('json')}
                >
                  JSON
                </button>
                <button
                  className={`results-tab ${outputFormat === 'yaml' ? 'active' : ''}`}
                  onClick={() => setOutputFormat('yaml')}
                >
                  YAML
                </button>
              </div>
            </div>
            <pre className="results-pre">{outputFormat === 'json' ? jsonOutput : yamlOutput}</pre>
          </div>

          {/* Validation */}
          <div className="results-card results-validation">
            <div className="results-card-header">
              <div className="results-card-title">Validation</div>
            </div>
            <div className="results-card-body">
              <div className="validate-row">
                <span className={`validate-icon ${currentBlocks.length > 0 ? 'validate-ok' : 'validate-warn'}`}>
                  {currentBlocks.length > 0 ? '\u2713' : '!'}
                </span>
                <span>
                  {currentBlocks.length > 0
                    ? <>Composition valid &mdash; {currentBlocks.length} blocks, {wires.length} wires</>
                    : <>Composition empty &mdash; no blocks</>
                  }
                </span>
              </div>
              {deletedNames.size > 0 && (
                <div className="validate-row">
                  <span className="validate-icon validate-info">&#8227;</span>
                  <span>{deletedNames.size} block{deletedNames.size > 1 ? 's' : ''} removed (restorable)</span>
                </div>
              )}
              <div className="results-source">
                Source: deploy/examples/sample-composition.json
              </div>
            </div>
          </div>

          {/* Topology & Wires */}
          <div className="results-card results-topology">
            <div className="results-card-header">
              <div className="results-card-title">Topology &amp; Wires</div>
            </div>
            <div className="results-card-body">
              {currentBlocks.length > 0 ? (
                <>
                  <div className="topo-order">
                    {sorted.map((b, i) => (
                      <div key={b.name} style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
                        {i > 0 && <span className="topo-arrow">&rarr;</span>}
                        <div className="topo-step">
                          <span className="topo-num">{i + 1}</span>
                          <span>{b.name}</span>
                        </div>
                      </div>
                    ))}
                  </div>
                  {wires.length > 0 && (
                    <div className="wires-list">
                      {wires.map((w, i) => (
                        <div className="wire-row" key={i}>
                          <span>{w.from} &rarr; {w.to}</span>
                        </div>
                      ))}
                    </div>
                  )}
                </>
              ) : (
                <div className="results-empty">No blocks to display topology</div>
              )}
            </div>
          </div>
        </div>
      </section>
    </div>
  )
}

export default App
