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

// Registry of known block categories for sidebar display
const categories = [
  { name: 'Storage', blocks: [{ kind: 'storage.local-pv', label: 'Local PV' }, { kind: 'storage.ebs', label: 'EBS' }] },
  { name: 'Datastore', blocks: [{ kind: 'datastore.postgresql', label: 'PostgreSQL' }, { kind: 'datastore.mysql', label: 'MySQL' }, { kind: 'datastore.redis', label: 'Redis' }] },
  { name: 'Security', blocks: [{ kind: 'security.password-rotation', label: 'Password Rotation' }, { kind: 'security.mtls', label: 'mTLS' }] },
  { name: 'Gateway', blocks: [{ kind: 'gateway.pgbouncer', label: 'PgBouncer' }, { kind: 'gateway.proxysql', label: 'ProxySQL' }] },
  { name: 'Observability', blocks: [{ kind: 'observability.metrics-exporter', label: 'Metrics Exporter' }] },
  { name: 'Integration', blocks: [{ kind: 'integration.s3-backup', label: 'S3 Backup' }] },
]

function App() {
  const [blocks, setBlocks] = useState<BlockRef[]>(initialBlocks)
  const [deletedNames, setDeletedNames] = useState<Set<string>>(new Set())
  const [selectedName, setSelectedName] = useState<string | null>(null)

  const liveBlocks = useMemo(() => blocks.filter(b => !deletedNames.has(b.name)), [blocks, deletedNames])

  // Strip dangling input references (inputs pointing to deleted blocks)
  const liveNameSet = useMemo(() => new Set(liveBlocks.map(b => b.name)), [liveBlocks])
  const resolvedBlocks = useMemo(() => liveBlocks.map(b => {
    if (!b.inputs) return b
    const cleaned: Record<string, string> = {}
    for (const [port, ref] of Object.entries(b.inputs)) {
      const fromBlock = ref.split('/')[0]
      if (liveNameSet.has(fromBlock)) cleaned[port] = ref
    }
    return Object.keys(cleaned).length > 0
      ? { ...b, inputs: cleaned }
      : { ...b, inputs: undefined }
  }), [liveBlocks, liveNameSet])

  // Detect broken references for validation
  const brokenRefs = useMemo(() => {
    const broken: { block: string; port: string; ref: string }[] = []
    for (const b of liveBlocks) {
      if (!b.inputs) continue
      for (const [port, ref] of Object.entries(b.inputs)) {
        const fromBlock = ref.split('/')[0]
        if (!liveNameSet.has(fromBlock)) {
          broken.push({ block: b.name, port, ref })
        }
      }
    }
    return broken
  }, [liveBlocks, liveNameSet])

  const sorted = useMemo(() => topoSort(resolvedBlocks), [resolvedBlocks])
  const wires = useMemo(() => getWires(resolvedBlocks), [resolvedBlocks])
  const activeKinds = useMemo(() => new Set(liveBlocks.map(b => b.kind)), [liveBlocks])
  const isValid = liveBlocks.length > 0 && brokenRefs.length === 0

  const compositionOutput = useMemo(() => JSON.stringify(
    { composition: { blocks: resolvedBlocks } },
    null,
    2,
  ), [resolvedBlocks])

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
        {liveBlocks.length === 0 ? (
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

      {/* Right: Parameters & generated output */}
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

        <div className="params-divider" />
        <div className="params-title">Generated Output</div>
        <pre className="params-output">{compositionOutput}</pre>
      </aside>

      {/* Bottom: Validation & topology */}
      <section className="validate">
        <div className="validate-title">Validation &amp; Topology</div>
        <div className="validate-row">
          <span className={`validate-icon ${isValid ? 'validate-ok' : 'validate-warn'}`}>
            {isValid ? '\u2713' : '!'}
          </span>
          <span>
            {liveBlocks.length === 0
              ? <>Composition empty &mdash; no blocks</>
              : isValid
                ? <>Composition valid &mdash; {liveBlocks.length} blocks, {wires.length} wires</>
                : <>Composition invalid &mdash; {liveBlocks.length} blocks, {brokenRefs.length} broken reference{brokenRefs.length > 1 ? 's' : ''}</>
            }
          </span>
        </div>
        {deletedNames.size > 0 && (
          <div className="validate-row">
            <span className="validate-icon validate-warn">!</span>
            <span>{deletedNames.size} block{deletedNames.size > 1 ? 's' : ''} removed</span>
          </div>
        )}
        {brokenRefs.map((br, i) => (
          <div className="validate-row validate-error" key={i} style={{ paddingLeft: 22 }}>
            <span>{br.block}.{br.port} references missing block "{br.ref.split('/')[0]}"</span>
          </div>
        ))}
        {liveBlocks.length > 0 && (
          <>
            <div className="validate-row">
              <span className="validate-icon validate-info">&#8227;</span>
              <span>Topological order:</span>
            </div>
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
            <div className="validate-row" style={{ marginTop: 8 }}>
              <span className="validate-icon validate-info">&#8227;</span>
              <span>Wires:</span>
            </div>
            {wires.map((w, i) => (
              <div className="validate-row" key={i} style={{ paddingLeft: 22 }}>
                <span>{w.from} &rarr; {w.to}</span>
              </div>
            ))}
          </>
        )}
        <div className="source-label">
          Source: deploy/examples/sample-composition.json
        </div>
      </section>
    </div>
  )
}

export default App
