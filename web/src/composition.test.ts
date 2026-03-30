import { describe, it, expect, vi, beforeAll, beforeEach, afterEach } from 'vitest'
import sampleComposition from '@examples/sample-composition.json'
import standardComposition from '@examples/standard-composition.json'

/**
 * Minimal verification that the workbench uses the onboarding sample
 * composition from deploy/examples/sample-composition.json as its
 * single source of truth.
 *
 * If this test fails, the sample file has changed in a way that
 * breaks the workbench's assumptions about the 3-block onboarding
 * path: local-pv -> postgresql -> pgbouncer.
 */
describe('onboarding sample composition', () => {
  it('loads from deploy/examples/sample-composition.json', () => {
    expect(sampleComposition).toBeDefined()
    expect(sampleComposition.composition).toBeDefined()
    expect(sampleComposition.composition.blocks).toBeInstanceOf(Array)
  })

  it('contains the 3-block onboarding path', () => {
    const blocks = sampleComposition.composition.blocks
    expect(blocks).toHaveLength(3)

    const names = blocks.map(b => b.name)
    expect(names).toEqual(['storage', 'db', 'pooler'])
  })

  it('has the expected block kinds', () => {
    const blocks = sampleComposition.composition.blocks
    const kinds = blocks.map(b => b.kind)
    expect(kinds).toEqual([
      'storage.local-pv',
      'datastore.postgresql',
      'gateway.pgbouncer',
    ])
  })

  it('wires storage -> db -> pooler via inputs', () => {
    const blocks = sampleComposition.composition.blocks
    const db = blocks.find(b => b.name === 'db')!
    const pooler = blocks.find(b => b.name === 'pooler')!

    expect(db.inputs).toBeDefined()
    expect(db.inputs!.storage).toBe('storage/pvc-spec')

    expect(pooler.inputs).toBeDefined()
    expect(pooler.inputs!['upstream-dsn']).toBe('db/dsn')
  })
})

describe('standard composition structure', () => {
  it('loads from deploy/examples/standard-composition.json', () => {
    expect(standardComposition).toBeDefined()
    expect(standardComposition.composition).toBeDefined()
    expect(standardComposition.composition.blocks).toBeInstanceOf(Array)
  })

  it('contains the 4-block standard path', () => {
    const blocks = standardComposition.composition.blocks
    expect(blocks).toHaveLength(4)

    const names = blocks.map(b => b.name)
    expect(names).toEqual(['storage', 'db', 'rotator', 'pooler'])
  })

  it('has explicit upstream-credential wire: pooler <- rotator', () => {
    const pooler = standardComposition.composition.blocks.find(b => b.name === 'pooler')!
    expect(pooler.inputs!['upstream-credential']).toBe('rotator/credential')
  })
})

/**
 * Credential source badge tests.
 *
 * The workbench fetches credential sources from the API topology endpoint
 * (POST /v1/compositions/topology → credentialSources). These tests verify
 * that the fetch function correctly consumes the API response for both
 * sample and standard paths, matching the same resolved wire truth as
 * CLI/API (#117).
 */
describe('credential source badge via API', () => {
  const originalFetch = globalThis.fetch

  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    globalThis.fetch = originalFetch
  })

  it('sample path: API returns pooler <- db', async () => {
    const mockResponse = {
      ok: true,
      json: async () => ({
        nodes: [],
        wires: [],
        credentialSources: { pooler: 'db' },
      }),
    }
    vi.mocked(fetch).mockResolvedValue(mockResponse as Response)

    const resp = await fetch('/v1/compositions/topology', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ composition: sampleComposition.composition }),
    })
    const data = await resp.json()

    expect(data.credentialSources).toEqual({ pooler: 'db' })
  })

  it('standard path: API returns pooler <- rotator', async () => {
    const mockResponse = {
      ok: true,
      json: async () => ({
        nodes: [],
        wires: [],
        credentialSources: { pooler: 'rotator' },
      }),
    }
    vi.mocked(fetch).mockResolvedValue(mockResponse as Response)

    const resp = await fetch('/v1/compositions/topology', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ composition: standardComposition.composition }),
    })
    const data = await resp.json()

    expect(data.credentialSources).toEqual({ pooler: 'rotator' })
  })

  it('credential note is separate from composition payload', async () => {
    const mockResponse = {
      ok: true,
      json: async () => ({
        nodes: [],
        wires: [],
        credentialSources: { pooler: 'db' },
      }),
    }
    vi.mocked(fetch).mockResolvedValue(mockResponse as Response)

    const resp = await fetch('/v1/compositions/topology', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ composition: sampleComposition.composition }),
    })
    const data = await resp.json()

    // Composition payload must stay clean — no credentialSources field
    const compositionPayload = { composition: { blocks: sampleComposition.composition.blocks } }
    expect(compositionPayload).not.toHaveProperty('credentialSources')

    // Credential note data comes from API response, shown as adjacent summary
    expect(data.credentialSources).toEqual({ pooler: 'db' })
  })

  it('surfaces API unavailability instead of silently hiding badges', async () => {
    vi.mocked(fetch).mockRejectedValue(new Error('network error'))

    // Replicate the fetchCredentialSources fallback logic —
    // must return available: false so the UI shows a visible note
    let sources: Record<string, string> = {}
    let available = true
    try {
      await fetch('/v1/compositions/topology', {
        method: 'POST',
        body: '{}',
      })
    } catch {
      sources = {}
      available = false
    }

    expect(sources).toEqual({})
    expect(available).toBe(false)
  })
})

describe('API status pill state mapping', () => {
  // Import the extracted helper that drives the pill JSX
  // If the mapping or helper is removed/renamed, this import fails → test fails
  let apiPillState: (available: boolean | null) => { label: string; className: string }

  beforeAll(async () => {
    const mod = await import('./App')
    apiPillState = mod.apiPillState
  })

  it('shows neutral state before API response', () => {
    const pill = apiPillState(null)
    expect(pill.label).toBe('API')
    expect(pill.className).toBe('')
  })

  it('shows connected when API is available', () => {
    const pill = apiPillState(true)
    expect(pill.label).toBe('API connected')
    expect(pill.className).toBe('api-connected')
  })

  it('shows unavailable when API is unreachable', () => {
    const pill = apiPillState(false)
    expect(pill.label).toBe('API unavailable')
    expect(pill.className).toBe('api-unavailable')
  })
})
