import sampleComposition from '@examples/sample-composition.json'
import './App.css'

const compositionJson = JSON.stringify(sampleComposition, null, 2)

interface BlockRef {
  kind: string
  name: string
  parameters?: Record<string, string>
  inputs?: Record<string, string>
}

const blocks: BlockRef[] = sampleComposition.composition.blocks

function categoryOf(kind: string): string {
  return kind.split('.')[0]
}

function dotClass(kind: string): string {
  return `sidebar-dot ${categoryOf(kind)}`
}

function getWires(blocks: BlockRef[]): { from: string; to: string; port: string }[] {
  const wires: { from: string; to: string; port: string }[] = []
  for (const b of blocks) {
    if (!b.inputs) continue
    for (const [port, ref] of Object.entries(b.inputs)) {
      const [fromBlock, fromPort] = ref.split('/')
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

const sorted = topoSort(blocks)
const wires = getWires(blocks)

// Registry of known block categories for sidebar display
const categories = [
  { name: 'Storage', blocks: [{ kind: 'storage.local-pv', label: 'Local PV' }, { kind: 'storage.ebs', label: 'EBS' }] },
  { name: 'Datastore', blocks: [{ kind: 'datastore.postgresql', label: 'PostgreSQL' }, { kind: 'datastore.mysql', label: 'MySQL' }, { kind: 'datastore.redis', label: 'Redis' }] },
  { name: 'Security', blocks: [{ kind: 'security.password-rotation', label: 'Password Rotation' }, { kind: 'security.mtls', label: 'mTLS' }] },
  { name: 'Gateway', blocks: [{ kind: 'gateway.pgbouncer', label: 'PgBouncer' }, { kind: 'gateway.proxysql', label: 'ProxySQL' }] },
  { name: 'Observability', blocks: [{ kind: 'observability.metrics-exporter', label: 'Metrics Exporter' }] },
  { name: 'Integration', blocks: [{ kind: 'integration.s3-backup', label: 'S3 Backup' }] },
]

const activeKinds = new Set(blocks.map(b => b.kind))

function App() {
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
      </aside>

      {/* Center: Canvas */}
      <main className="canvas">
        <div className="canvas-title">Composition Pipeline</div>
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
                <div className="block-card">
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
      </main>

      {/* Right: Parameters & generated output */}
      <aside className="params">
        <div className="params-title">Block Details</div>
        {blocks.map(b => (
          <div className="params-block" key={b.name}>
            <div className="params-block-name">{b.name}</div>
            {b.parameters && Object.entries(b.parameters).map(([k, v]) => (
              <div className="params-row" key={k}>
                <span className="params-key">{k}</span>
                <span className="params-val">{v}</span>
              </div>
            ))}
            {b.inputs && (
              <>
                <div className="params-inputs-title">Inputs</div>
                {Object.entries(b.inputs).map(([k, v]) => (
                  <div className="params-row" key={k}>
                    <span className="params-key">{k}</span>
                    <span className="params-val">{v}</span>
                  </div>
                ))}
              </>
            )}
          </div>
        ))}

        <div className="params-divider" />
        <div className="params-title">Generated Output</div>
        <pre className="params-output">{compositionJson}</pre>
      </aside>

      {/* Bottom: Validation & topology */}
      <section className="validate">
        <div className="validate-title">Validation &amp; Topology</div>
        <div className="validate-row">
          <span className="validate-icon validate-ok">&#10003;</span>
          <span>Composition valid &mdash; {blocks.length} blocks, {wires.length} wires</span>
        </div>
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
        <div className="source-label">
          Source: deploy/examples/sample-composition.json
        </div>
      </section>
    </div>
  )
}

export default App
