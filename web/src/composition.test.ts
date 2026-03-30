import { describe, it, expect } from 'vitest'
import sampleComposition from '@examples/sample-composition.json'
import standardComposition from '@examples/standard-composition.json'

// Mirrors Go block.CredentialSources: extracts consumer→source for
// each wire whose destination port is "upstream-credential".
function getCredentialSources(
  blocks: Array<{ name: string; inputs?: Record<string, string> }>,
): Map<string, string> {
  const sources = new Map<string, string>()
  const nameSet = new Set(blocks.map(b => b.name))
  for (const b of blocks) {
    if (!b.inputs) continue
    for (const [port, ref] of Object.entries(b.inputs)) {
      if (port !== 'upstream-credential') continue
      const fromBlock = ref.split('/')[0]
      if (nameSet.has(fromBlock)) {
        sources.set(b.name, fromBlock)
      }
    }
  }
  return sources
}

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

  it('has no explicit credential source (auto-wired by backend)', () => {
    const sources = getCredentialSources(sampleComposition.composition.blocks)
    expect(sources.size).toBe(0)
  })
})

describe('standard composition credential source', () => {
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

  it('derives credential source: pooler <- rotator', () => {
    const sources = getCredentialSources(standardComposition.composition.blocks)
    expect(sources.size).toBe(1)
    expect(sources.get('pooler')).toBe('rotator')
  })
})
