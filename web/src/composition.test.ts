import { describe, it, expect } from 'vitest'
import sampleComposition from '@examples/sample-composition.json'

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
