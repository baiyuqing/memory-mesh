// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, act, cleanup } from '@testing-library/react'
import { ApiPill } from './App'

describe('ApiPill rendered copy action', () => {
  let writeText: ReturnType<typeof vi.fn>

  beforeEach(() => {
    writeText = vi.fn().mockResolvedValue(undefined)
    Object.assign(navigator, { clipboard: { writeText } })
    vi.useFakeTimers()
  })

  afterEach(() => {
    cleanup()
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('does not render copy button when API is connected', () => {
    render(<ApiPill available={true} />)
    expect(screen.queryByTitle('Copy command')).toBeNull()
  })

  it('does not render copy button in neutral state', () => {
    render(<ApiPill available={null} />)
    expect(screen.queryByTitle('Copy command')).toBeNull()
  })

  it('renders copy button when API is unavailable', () => {
    render(<ApiPill available={false} />)
    const btn = screen.getByTitle('Copy command')
    expect(btn).toBeDefined()
    expect(btn.textContent).toBe('copy')
  })

  it('clicking copy writes "make workbench" to clipboard', async () => {
    render(<ApiPill available={false} />)
    const btn = screen.getByTitle('Copy command')

    await act(async () => {
      fireEvent.click(btn)
    })

    expect(writeText).toHaveBeenCalledOnce()
    expect(writeText).toHaveBeenCalledWith('make workbench')
  })

  it('button label changes to "copied" after click, then resets', async () => {
    render(<ApiPill available={false} />)
    const btn = screen.getByTitle('Copy command')

    // Click triggers clipboard write
    await act(async () => {
      fireEvent.click(btn)
    })

    // Label should now be "copied"
    expect(btn.textContent).toBe('copied')

    // After 1500ms the label resets to "copy"
    await act(async () => {
      vi.advanceTimersByTime(1500)
    })

    expect(btn.textContent).toBe('copy')
  })
})
