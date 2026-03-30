import { useState, useMemo, useEffect, useCallback, useRef } from 'react'
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

// Derives the API status pill display state from the availability flag.
// Exported so tests can verify the exact label/class/hint/docsUrl mapping.
export function apiPillState(available: boolean | null): { label: string; className: string; hint: string | null; docsUrl: string | null; target: string | null; connectedNote: string | null } {
  if (available === true) return { label: 'API connected', className: 'api-connected', hint: null, docsUrl: '/QUICKSTART.md', target: 'localhost:8080', connectedNote: 'credential sources live' }
  if (available === false) return { label: 'API unavailable', className: 'api-unavailable', hint: 'make workbench', docsUrl: '/QUICKSTART.md', target: 'localhost:8080', connectedNote: null }
  return { label: 'API', className: '', hint: null, docsUrl: null, target: null, connectedNote: null }
}

// Copies a command string to the clipboard and calls onCopied when done.
// Exported so tests can verify the clipboard integration without rendering.
export function copyToClipboard(
  command: string,
  onCopied: () => void,
): void {
  navigator.clipboard.writeText(command).then(onCopied, () => {})
}

export function formatHealthTime(time: Date): string {
  const h = String(time.getHours()).padStart(2, '0')
  const m = String(time.getMinutes()).padStart(2, '0')
  const s = String(time.getSeconds()).padStart(2, '0')
  return `${h}:${m}:${s}`
}

// Renders the API status pill with optional CTA + copy button + retry.
// Exported so tests can render and click the pill in isolation.
export function ApiPill({ available, onRetry, onHealthCheck }: { available: boolean | null; onRetry?: () => void; onHealthCheck?: () => Promise<boolean> }) {
  const [copied, setCopied] = useState(false)
  const [targetCopied, setTargetCopied] = useState(false)
  const [healthStatus, setHealthStatus] = useState<'idle' | 'checking' | 'ok' | 'fail'>('idle')
  const [healthResult, setHealthResult] = useState<'ok' | 'fail' | null>(null)
  const [healthTime, setHealthTime] = useState<Date | null>(null)
  const [healthTarget, setHealthTarget] = useState<string | null>(null)
  const [recovered, setRecovered] = useState(false)
  const [targetChanged, setTargetChanged] = useState(false)
  const targetChangedTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const prevAvailable = useRef(available)
  const pill = apiPillState(available)

  useEffect(() => {
    if (prevAvailable.current === false && available === true) {
      setRecovered(true)
      const timer = setTimeout(() => setRecovered(false), 3000)
      return () => clearTimeout(timer)
    }
    prevAvailable.current = available
  }, [available])

  useEffect(() => {
    if (healthTarget && pill.target !== healthTarget) {
      setHealthResult(null)
      setHealthTime(null)
      setHealthTarget(null)
      setHealthStatus('idle')
      setTargetChanged(true)
      if (targetChangedTimer.current) clearTimeout(targetChangedTimer.current)
      targetChangedTimer.current = setTimeout(() => setTargetChanged(false), 3000)
    }
  }, [pill.target, healthTarget])

  const handleCopy = useCallback(() => {
    if (!pill.hint) return
    copyToClipboard(pill.hint, () => {
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    })
  }, [pill.hint])

  const handleHealthCheck = useCallback(() => {
    if (!onHealthCheck) return
    setHealthStatus('checking')
    onHealthCheck().then(ok => {
      const result = ok ? 'ok' : 'fail'
      setHealthStatus(result)
      setHealthResult(result)
      setHealthTime(new Date())
      setHealthTarget(pill.target)
      setTimeout(() => setHealthStatus('idle'), 1500)
    }, () => {
      setHealthStatus('fail')
      setHealthResult('fail')
      setHealthTime(new Date())
      setHealthTarget(pill.target)
      setTimeout(() => setHealthStatus('idle'), 1500)
    })
  }, [onHealthCheck, pill.target])

  const handleTargetCopy = useCallback(() => {
    if (!pill.target) return
    copyToClipboard(pill.target, () => {
      setTargetCopied(true)
      setTimeout(() => setTargetCopied(false), 1500)
    })
  }, [pill.target])

  return (
    <div className={`header-api-pill ${pill.className}`}>
      <span className="header-api-dot" />
      <span className="header-api-label">{pill.label}</span>
      {recovered && (
        <span className="header-api-recovered">credential sources ready</span>
      )}
      {pill.target && (
        <span className="header-api-target">{pill.target}</span>
      )}
      {available === true && pill.target && (
        <button
          className="header-api-target-copy"
          onClick={handleTargetCopy}
          title="Copy target"
        >
          {targetCopied ? 'copied' : 'copy'}
        </button>
      )}
      {available === true && pill.docsUrl && (
        <a
          className="header-api-target-docs"
          href={pill.docsUrl}
          target="_blank"
          rel="noopener noreferrer"
          title="API docs"
        >
          docs
        </a>
      )}
      {available === true && onHealthCheck && (
        <button
          className="header-api-target-health"
          onClick={handleHealthCheck}
          title="Check API health"
          disabled={healthStatus === 'checking'}
        >
          {healthStatus === 'idle' ? 'ping' : healthStatus}
        </button>
      )}
      {available === true && healthResult && (
        <span className={`header-api-health-result header-api-health-result-${healthResult} header-api-health-emphasis`}>
          {healthTarget && <span className="header-api-health-target">{healthTarget}</span>}
          {healthResult === 'ok' ? 'reachable' : 'unreachable'}
        </span>
      )}
      {available === true && healthTime && healthResult && (
        <>
          <span className="header-api-health-time">{formatHealthTime(healthTime)}</span>
          <button
            className="header-api-health-clear"
            onClick={() => { setHealthResult(null); setHealthTime(null); setHealthTarget(null); setHealthStatus('idle') }}
            title="Clear health result"
          >
            clear
          </button>
        </>
      )}
      {available === true && targetChanged && (
        <span className="header-api-target-changed">health record cleared after target changed</span>
      )}
      {pill.connectedNote && !recovered && (
        <span className="header-api-connected-note">{pill.connectedNote}</span>
      )}
      {pill.hint && (
        <span className="header-api-hint">
          run <code>{pill.hint}</code>
          <button
            className="header-api-copy"
            onClick={handleCopy}
            title="Copy command"
          >
            {copied ? 'copied' : 'copy'}
          </button>
          {onRetry && (
            <button
              className="header-api-retry"
              onClick={onRetry}
              title="Retry connection"
            >
              retry
            </button>
          )}
          {pill.docsUrl && (
            <a
              className="header-api-docs"
              href={pill.docsUrl}
              target="_blank"
              rel="noopener noreferrer"
              title="Setup instructions"
            >
              docs
            </a>
          )}
        </span>
      )}
    </div>
  )
}

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

// Fetches credential sources from the API topology endpoint.
// The API compiles the composition (normalize, auto-wire, validate,
// topo-sort) and returns credentialSources as a per-consumer map,
// so the workbench consumes the same resolved truth as CLI/API.
// Returns { sources, available } so the UI can distinguish "no sources"
// from "API unreachable".
async function fetchCredentialSources(
  blocks: BlockRef[],
): Promise<{ sources: Record<string, string>; available: boolean }> {
  try {
    const resp = await fetch('/v1/compositions/topology', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ composition: { blocks } }),
    })
    if (!resp.ok) return { sources: {}, available: false }
    const data = await resp.json()
    return { sources: data.credentialSources ?? {}, available: true }
  } catch {
    return { sources: {}, available: false }
  }
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

// Per-block-kind field metadata for the 3 onboarding blocks
interface FieldMeta {
  key: string
  label: string
  type: 'text' | 'number' | 'select'
  group: string
  required?: boolean
  defaultValue?: string
  description?: string
  options?: string[]
}

const blockFieldMeta: Record<string, { title: string; description: string; fields: FieldMeta[] }> = {
  'storage.local-pv': {
    title: 'Local Persistent Volume',
    description: 'Provisions a local persistent volume on the node for data storage.',
    fields: [
      { key: 'size', label: 'Volume Size', type: 'text', group: 'Storage', required: true, defaultValue: '5Gi', description: 'Capacity of the persistent volume (e.g. 1Gi, 10Gi)' },
    ],
  },
  'datastore.postgresql': {
    title: 'PostgreSQL Database',
    description: 'Deploys a PostgreSQL cluster with configurable version and replicas.',
    fields: [
      { key: 'version', label: 'Version', type: 'select', group: 'Engine', required: true, defaultValue: '16', description: 'PostgreSQL major version', options: ['14', '15', '16', '17'] },
      { key: 'replicas', label: 'Replicas', type: 'number', group: 'Scaling', required: false, defaultValue: '3', description: 'Number of database replicas for high availability' },
    ],
  },
  'gateway.pgbouncer': {
    title: 'PgBouncer Connection Pooler',
    description: 'Lightweight connection pooler that sits between the application and PostgreSQL.',
    fields: [],
  },
}

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
  const [credentialSources, setCredentialSources] = useState<Record<string, string>>({})
  const [apiAvailable, setApiAvailable] = useState<boolean | null>(null)
  const activeKinds = useMemo(() => new Set(currentBlocks.map(b => b.kind)), [currentBlocks])

  const retryApi = useCallback(() => {
    if (currentBlocks.length === 0) return
    fetchCredentialSources(currentBlocks).then(({ sources, available }) => {
      setCredentialSources(sources)
      setApiAvailable(available)
    })
  }, [currentBlocks])

  const checkApiHealth = useCallback(async (): Promise<boolean> => {
    const { available } = await fetchCredentialSources(currentBlocks)
    setApiAvailable(available)
    return available
  }, [currentBlocks])

  // Fetch credential sources from the API topology endpoint whenever
  // the composition changes. This consumes the same compiled wire truth
  // as CLI/API (#117) instead of reimplementing auto-wire logic locally.
  useEffect(() => {
    if (currentBlocks.length === 0) {
      setCredentialSources({})
      setApiAvailable(null)
      return
    }
    fetchCredentialSources(currentBlocks).then(({ sources, available }) => {
      setCredentialSources(sources)
      setApiAvailable(available)
    })
  }, [currentBlocks])

  const compositionData = useMemo(() => ({ composition: { blocks: currentBlocks } }), [currentBlocks])
  const jsonOutput = useMemo(() => JSON.stringify(compositionData, null, 2), [compositionData])
  const yamlOutput = useMemo(() => toYaml(compositionData), [compositionData])

  const selectedBlock = blocks.find(b => b.name === selectedName) ?? null
  const resolvedSelectedBlock = currentBlocks.find(b => b.name === selectedName) ?? null
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
        <div className="header-source">
          <span className="header-source-dot" />
          <span className="header-source-path">deploy/examples/sample-composition.json</span>
          <div className="header-sep" />
          <span className="header-block-count">{currentBlocks.length} blocks &middot; {wires.length} wires</span>
        </div>
        <ApiPill available={apiAvailable} onRetry={retryApi} onHealthCheck={checkApiHealth} />
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
        {blocks.every(b => deletedNames.has(b.name)) ? (
          <div className="canvas-empty">No blocks in composition. Restore blocks from the sidebar.</div>
        ) : (
          <div className="pipeline">
            {topoSort(blocks).map((b, i, arr) => {
              const isDeleted = deletedNames.has(b.name)
              const cat = categoryOf(b.kind)
              const prevBlock = arr[i - 1]
              const incomingWire = i > 0 && b.inputs && prevBlock
                ? Object.entries(b.inputs).find(([, ref]) => ref.split('/')[0] === prevBlock.name)
                : null
              const wireActive = incomingWire && !isDeleted && !deletedNames.has(prevBlock?.name ?? '')

              return (
                <div key={b.name} style={{ display: 'flex', alignItems: 'center' }}>
                  {i > 0 && (
                    <div className={`wire ${wireActive ? 'wire-active' : 'wire-inactive'}`}>
                      <div className="wire-label">
                        {incomingWire ? (
                          <>
                            <span className="wire-port-from">{incomingWire[1].split('/')[1]}</span>
                            <span className="wire-arrow-label">&rarr;</span>
                            <span className="wire-port-to">{incomingWire[0]}</span>
                          </>
                        ) : ''}
                      </div>
                      <div className="wire-line" />
                    </div>
                  )}
                  <div
                    className={`block-card ${selectedName === b.name ? 'selected' : ''} ${isDeleted ? 'block-card-deleted' : ''} block-card-${cat}`}
                    onClick={(e) => { e.stopPropagation(); handleCardClick(b.name) }}
                  >
                    <div className="block-card-header">
                      <span className={`block-card-badge ${cat}`}>{cat}</span>
                      {isDeleted && (
                        <button
                          className="block-card-restore"
                          onClick={(e) => { e.stopPropagation(); toggleDelete(b.name) }}
                          title="Restore block"
                        >
                          Restore
                        </button>
                      )}
                    </div>
                    <div className="block-card-name">{b.name}</div>
                    <div className="block-card-kind">{b.kind}</div>
                    {isDeleted && <div className="block-card-removed-chip">Removed</div>}
                    {!isDeleted && b.parameters && (
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
        {selectedBlock ? (() => {
          const meta = blockFieldMeta[selectedBlock.kind]
          const groups = meta
            ? Array.from(new Set(meta.fields.map(f => f.group)))
            : []

          return (
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
              {meta && <div className="params-block-desc">{meta.description}</div>}
              {isSelectedDeleted && (
                <div className="params-deleted-badge">Removed from composition</div>
              )}
              {!isSelectedDeleted && meta && meta.fields.length > 0 && (
                <>
                  {groups.map(group => (
                    <div className="params-group" key={group}>
                      <div className="params-group-title">{group}</div>
                      {meta.fields.filter(f => f.group === group).map(field => {
                        const val = selectedBlock.parameters?.[field.key] ?? ''
                        return (
                          <div className="params-field-wrap" key={field.key}>
                            <div className="params-field">
                              <label className="params-field-label">
                                {field.label}
                                {field.required && <span className="params-required">*</span>}
                              </label>
                              {field.type === 'select' ? (
                                <select
                                  className="params-select"
                                  value={val}
                                  onChange={(e) => updateParam(selectedBlock.name, field.key, e.target.value)}
                                >
                                  {field.options?.map(opt => (
                                    <option key={opt} value={opt}>{opt}</option>
                                  ))}
                                </select>
                              ) : field.type === 'number' ? (
                                <input
                                  className="params-input"
                                  type="number"
                                  min="1"
                                  value={val}
                                  placeholder={field.defaultValue}
                                  onChange={(e) => updateParam(selectedBlock.name, field.key, e.target.value)}
                                />
                              ) : (
                                <input
                                  className="params-input"
                                  value={val}
                                  placeholder={field.defaultValue}
                                  onChange={(e) => updateParam(selectedBlock.name, field.key, e.target.value)}
                                />
                              )}
                            </div>
                            {field.description && (
                              <div className="params-field-desc">{field.description}{field.defaultValue && <> &middot; default: <code>{field.defaultValue}</code></>}</div>
                            )}
                          </div>
                        )
                      })}
                    </div>
                  ))}
                </>
              )}
              {!isSelectedDeleted && meta && meta.fields.length === 0 && (
                <div className="params-no-params">
                  No configurable parameters. This block is configured automatically via its inputs.
                </div>
              )}
              {!isSelectedDeleted && !meta && selectedBlock.parameters && (
                <>
                  <div className="params-group-title">Parameters</div>
                  {Object.entries(selectedBlock.parameters).map(([k, v]) => (
                    <div className="params-field" key={k}>
                      <label className="params-field-label">{k}</label>
                      <input
                        className="params-input"
                        value={v}
                        onChange={(e) => updateParam(selectedBlock.name, k, e.target.value)}
                      />
                    </div>
                  ))}
                </>
              )}
              {resolvedSelectedBlock?.inputs && !isSelectedDeleted && (
                <div className="params-group">
                  <div className="params-group-title">Inputs</div>
                  {Object.entries(resolvedSelectedBlock.inputs).map(([k, v]) => (
                    <div className="params-field" key={k}>
                      <label className="params-field-label">{k}</label>
                      <span className="params-val">{v}</span>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )
        })() : (
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
            {Object.keys(credentialSources).length > 0 && (
              <div className="output-credential-note">
                {Object.entries(credentialSources)
                  .sort(([a], [b]) => a.localeCompare(b))
                  .map(([consumer, source]) => (
                    <span key={consumer} className="output-credential-item">
                      credential: {consumer} &larr; {source}
                    </span>
                  ))}
              </div>
            )}
            {apiAvailable === false && (
              <div className="output-credential-note output-credential-unavailable">
                <span className="output-credential-item">credential sources unavailable — start API server</span>
              </div>
            )}
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
                  {apiAvailable === false && (
                    <div className="credential-sources">
                      <div className="credential-source-row">
                        <span className="credential-source-badge credential-source-unavailable">credential</span>
                        <span className="credential-source-text">unavailable — start API server for credential source badges</span>
                      </div>
                    </div>
                  )}
                  {Object.keys(credentialSources).length > 0 && (
                    <div className="credential-sources">
                      {Object.entries(credentialSources)
                        .sort(([a], [b]) => a.localeCompare(b))
                        .map(([consumer, source]) => (
                          <div className="credential-source-row" key={consumer}>
                            <span className="credential-source-badge">credential</span>
                            <span className="credential-source-text">{consumer} &larr; {source}</span>
                          </div>
                        ))}
                    </div>
                  )}
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
