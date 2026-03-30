// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, act, cleanup } from '@testing-library/react'
import { ApiPill } from './App'

describe('ApiPill connected-source emphasis', () => {
  beforeEach(() => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.assign(navigator, { clipboard: { writeText } })
  })

  afterEach(() => {
    cleanup()
    vi.restoreAllMocks()
  })

  it('shows credential sources live note when connected', () => {
    render(<ApiPill available={true} />)
    expect(screen.getByText('credential sources live')).toBeDefined()
  })

  it('does not show connected note when unavailable', () => {
    render(<ApiPill available={false} />)
    expect(screen.queryByText('credential sources live')).toBeNull()
  })

  it('does not show connected note in neutral state', () => {
    render(<ApiPill available={null} />)
    expect(screen.queryByText('credential sources live')).toBeNull()
  })
})

describe('ApiPill connected confirmation', () => {
  beforeEach(() => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.assign(navigator, { clipboard: { writeText } })
    vi.useFakeTimers()
  })

  afterEach(() => {
    cleanup()
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('shows recovery confirmation when transitioning from unavailable to connected', () => {
    const { rerender } = render(<ApiPill available={false} />)
    rerender(<ApiPill available={true} />)
    expect(screen.getByText('credential sources ready')).toBeDefined()
  })

  it('does not show confirmation on initial connected state', () => {
    render(<ApiPill available={true} />)
    expect(screen.queryByText('credential sources ready')).toBeNull()
  })

  it('does not show confirmation on initial neutral state', () => {
    render(<ApiPill available={null} />)
    expect(screen.queryByText('credential sources ready')).toBeNull()
  })

  it('confirmation auto-clears after 3 seconds', async () => {
    const { rerender } = render(<ApiPill available={false} />)
    rerender(<ApiPill available={true} />)
    expect(screen.getByText('credential sources ready')).toBeDefined()

    await act(async () => {
      vi.advanceTimersByTime(3000)
    })

    expect(screen.queryByText('credential sources ready')).toBeNull()
  })
})

describe('ApiPill target note', () => {
  beforeEach(() => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.assign(navigator, { clipboard: { writeText } })
  })

  afterEach(() => {
    cleanup()
    vi.restoreAllMocks()
  })

  it('shows target endpoint when unavailable', () => {
    render(<ApiPill available={false} />)
    expect(screen.getByText('localhost:8080')).toBeDefined()
  })

  it('shows target as badge when connected', () => {
    render(<ApiPill available={true} />)
    const target = screen.getByText('localhost:8080')
    expect(target).toBeDefined()
    expect(target.className).toBe('header-api-target')
  })

  it('does not show target in neutral state', () => {
    render(<ApiPill available={null} />)
    expect(screen.queryByText('localhost:8080')).toBeNull()
  })
})

describe('ApiPill docs link', () => {
  beforeEach(() => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.assign(navigator, { clipboard: { writeText } })
  })

  afterEach(() => {
    cleanup()
    vi.restoreAllMocks()
  })

  it('renders docs link when unavailable', () => {
    render(<ApiPill available={false} />)
    const link = screen.getByTitle('Setup instructions')
    expect(link).toBeDefined()
    expect(link.textContent).toBe('docs')
    expect(link.getAttribute('href')).toBe('/QUICKSTART.md')
    expect(link.getAttribute('target')).toBe('_blank')
  })

  it('does not render docs link when connected', () => {
    render(<ApiPill available={true} />)
    expect(screen.queryByTitle('Setup instructions')).toBeNull()
  })

  it('does not render docs link in neutral state', () => {
    render(<ApiPill available={null} />)
    expect(screen.queryByTitle('Setup instructions')).toBeNull()
  })
})

describe('ApiPill retry action', () => {
  beforeEach(() => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.assign(navigator, { clipboard: { writeText } })
  })

  afterEach(() => {
    cleanup()
    vi.restoreAllMocks()
  })

  it('does not render retry button when no onRetry prop', () => {
    render(<ApiPill available={false} />)
    expect(screen.queryByTitle('Retry connection')).toBeNull()
  })

  it('renders retry button when unavailable and onRetry provided', () => {
    const onRetry = vi.fn()
    render(<ApiPill available={false} onRetry={onRetry} />)
    const btn = screen.getByTitle('Retry connection')
    expect(btn).toBeDefined()
    expect(btn.textContent).toBe('retry')
  })

  it('does not render retry button when connected', () => {
    const onRetry = vi.fn()
    render(<ApiPill available={true} onRetry={onRetry} />)
    expect(screen.queryByTitle('Retry connection')).toBeNull()
  })

  it('calls onRetry when retry button is clicked', () => {
    const onRetry = vi.fn()
    render(<ApiPill available={false} onRetry={onRetry} />)
    fireEvent.click(screen.getByTitle('Retry connection'))
    expect(onRetry).toHaveBeenCalledOnce()
  })

  it('flips to connected state after successful retry', () => {
    const onRetry = vi.fn()
    const { rerender } = render(<ApiPill available={false} onRetry={onRetry} />)

    // Retry button visible in unavailable state
    expect(screen.getByTitle('Retry connection')).toBeDefined()

    // Simulate parent updating available to true after retry succeeds
    rerender(<ApiPill available={true} onRetry={onRetry} />)

    // Retry button and hint should disappear
    expect(screen.queryByTitle('Retry connection')).toBeNull()
    expect(screen.getByText('API connected')).toBeDefined()
  })
})

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
