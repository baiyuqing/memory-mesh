// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, act, cleanup } from '@testing-library/react'
import { ApiPill, formatHealthTime } from './App'

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
    // Connected badge treatment: parent pill has api-connected class,
    // which activates .api-connected .header-api-target CSS styling
    const pill = target.closest('.header-api-pill')
    expect(pill).not.toBeNull()
    expect(pill!.classList.contains('api-connected')).toBe(true)
  })

  it('target in unavailable state does not get connected badge treatment', () => {
    render(<ApiPill available={false} />)
    const target = screen.getByText('localhost:8080')
    expect(target.className).toBe('header-api-target')
    const pill = target.closest('.header-api-pill')
    expect(pill).not.toBeNull()
    expect(pill!.classList.contains('api-connected')).toBe(false)
    expect(pill!.classList.contains('api-unavailable')).toBe(true)
  })

  it('does not show target in neutral state', () => {
    render(<ApiPill available={null} />)
    expect(screen.queryByText('localhost:8080')).toBeNull()
  })
})

describe('ApiPill target copy action', () => {
  beforeEach(() => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.assign(navigator, { clipboard: { writeText } })
  })

  afterEach(() => {
    cleanup()
    vi.restoreAllMocks()
  })

  it('shows copy button next to target when connected', () => {
    render(<ApiPill available={true} />)
    const copyBtn = screen.getByTitle('Copy target')
    expect(copyBtn).toBeDefined()
    expect(copyBtn.className).toBe('header-api-target-copy')
    expect(copyBtn.textContent).toBe('copy')
  })

  it('copies target text to clipboard on click', async () => {
    render(<ApiPill available={true} />)
    const copyBtn = screen.getByTitle('Copy target')
    copyBtn.click()
    await vi.waitFor(() => {
      expect(navigator.clipboard.writeText).toHaveBeenCalledWith('localhost:8080')
    })
  })

  it('shows copied feedback after click', async () => {
    render(<ApiPill available={true} />)
    const copyBtn = screen.getByTitle('Copy target')
    copyBtn.click()
    await vi.waitFor(() => {
      expect(copyBtn.textContent).toBe('copied')
    })
  })

  it('does not show target copy button when unavailable', () => {
    render(<ApiPill available={false} />)
    expect(screen.queryByTitle('Copy target')).toBeNull()
  })

  it('does not show target copy button in neutral state', () => {
    render(<ApiPill available={null} />)
    expect(screen.queryByTitle('Copy target')).toBeNull()
  })
})

describe('ApiPill target docs link', () => {
  afterEach(() => {
    cleanup()
  })

  it('shows docs link next to target when connected', () => {
    render(<ApiPill available={true} />)
    const link = screen.getByTitle('API docs')
    expect(link).toBeDefined()
    expect(link.className).toBe('header-api-target-docs')
    expect(link.textContent).toBe('docs')
    expect(link.getAttribute('href')).toBe('/QUICKSTART.md')
    expect(link.getAttribute('target')).toBe('_blank')
  })

  it('does not show target docs link when unavailable', () => {
    render(<ApiPill available={false} />)
    expect(screen.queryByTitle('API docs')).toBeNull()
  })

  it('does not show target docs link in neutral state', () => {
    render(<ApiPill available={null} />)
    expect(screen.queryByTitle('API docs')).toBeNull()
  })
})

describe('ApiPill target health action', () => {
  afterEach(() => {
    cleanup()
    vi.restoreAllMocks()
  })

  it('shows ping button when connected with onHealthCheck', () => {
    const onHealthCheck = vi.fn().mockResolvedValue(true)
    render(<ApiPill available={true} onHealthCheck={onHealthCheck} />)
    const btn = screen.getByTitle('Check API health')
    expect(btn).toBeDefined()
    expect(btn.className).toBe('header-api-target-health')
    expect(btn.textContent).toBe('ping')
  })

  it('shows checking then ok on successful health check', async () => {
    let resolve: (v: boolean) => void
    const onHealthCheck = vi.fn().mockImplementation(() => new Promise<boolean>(r => { resolve = r }))
    render(<ApiPill available={true} onHealthCheck={onHealthCheck} />)
    const btn = screen.getByTitle('Check API health')
    btn.click()
    expect(onHealthCheck).toHaveBeenCalledOnce()
    await vi.waitFor(() => {
      expect(btn.textContent).toBe('checking')
    })
    resolve!(true)
    await vi.waitFor(() => {
      expect(btn.textContent).toBe('ok')
    })
  })

  it('shows fail when health check returns false', async () => {
    const onHealthCheck = vi.fn().mockResolvedValue(false)
    render(<ApiPill available={true} onHealthCheck={onHealthCheck} />)
    const btn = screen.getByTitle('Check API health')
    btn.click()
    await vi.waitFor(() => {
      expect(btn.textContent).toBe('fail')
    })
  })

  it('shows fail when health check rejects', async () => {
    const onHealthCheck = vi.fn().mockRejectedValue(new Error('network'))
    render(<ApiPill available={true} onHealthCheck={onHealthCheck} />)
    const btn = screen.getByTitle('Check API health')
    btn.click()
    await vi.waitFor(() => {
      expect(btn.textContent).toBe('fail')
    })
  })

  it('does not show ping button when unavailable', () => {
    const onHealthCheck = vi.fn().mockResolvedValue(true)
    render(<ApiPill available={false} onHealthCheck={onHealthCheck} />)
    expect(screen.queryByTitle('Check API health')).toBeNull()
  })

  it('does not show ping button without onHealthCheck', () => {
    render(<ApiPill available={true} />)
    expect(screen.queryByTitle('Check API health')).toBeNull()
  })

  it('does not show ping button in neutral state', () => {
    const onHealthCheck = vi.fn().mockResolvedValue(true)
    render(<ApiPill available={null} onHealthCheck={onHealthCheck} />)
    expect(screen.queryByTitle('Check API health')).toBeNull()
  })
})

describe('ApiPill health result', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    cleanup()
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('shows reachable result with target after successful health check resets', async () => {
    const onHealthCheck = vi.fn().mockResolvedValue(true)
    render(<ApiPill available={true} onHealthCheck={onHealthCheck} />)
    const btn = screen.getByTitle('Check API health')
    btn.click()
    await vi.waitFor(() => {
      expect(btn.textContent).toBe('ok')
    })
    vi.advanceTimersByTime(1500)
    await vi.waitFor(() => {
      expect(btn.textContent).toBe('ping')
    })
    const result = document.querySelector('.header-api-health-result')
    expect(result).not.toBeNull()
    expect(result!.classList.contains('header-api-health-result-ok')).toBe(true)
    expect(result!.textContent).toContain('reachable')
    const target = result!.querySelector('.header-api-health-target')
    expect(target).not.toBeNull()
    expect(target!.textContent).toBe('localhost:8080')
  })

  it('shows unreachable result with target after failed health check resets', async () => {
    const onHealthCheck = vi.fn().mockResolvedValue(false)
    render(<ApiPill available={true} onHealthCheck={onHealthCheck} />)
    const btn = screen.getByTitle('Check API health')
    btn.click()
    await vi.waitFor(() => {
      expect(btn.textContent).toBe('fail')
    })
    vi.advanceTimersByTime(1500)
    await vi.waitFor(() => {
      expect(btn.textContent).toBe('ping')
    })
    const result = document.querySelector('.header-api-health-result')
    expect(result).not.toBeNull()
    expect(result!.classList.contains('header-api-health-result-fail')).toBe(true)
    expect(result!.textContent).toContain('unreachable')
    const target = result!.querySelector('.header-api-health-target')
    expect(target).not.toBeNull()
    expect(target!.textContent).toBe('localhost:8080')
  })

  it('success result carries emphasis class', async () => {
    const onHealthCheck = vi.fn().mockResolvedValue(true)
    render(<ApiPill available={true} onHealthCheck={onHealthCheck} />)
    screen.getByTitle('Check API health').click()
    await vi.waitFor(() => {
      const result = document.querySelector('.header-api-health-result')
      expect(result).not.toBeNull()
      expect(result!.classList.contains('header-api-health-emphasis')).toBe(true)
      expect(result!.classList.contains('header-api-health-result-ok')).toBe(true)
    })
  })

  it('failure result carries emphasis class', async () => {
    const onHealthCheck = vi.fn().mockResolvedValue(false)
    render(<ApiPill available={true} onHealthCheck={onHealthCheck} />)
    screen.getByTitle('Check API health').click()
    await vi.waitFor(() => {
      const result = document.querySelector('.header-api-health-result')
      expect(result).not.toBeNull()
      expect(result!.classList.contains('header-api-health-emphasis')).toBe(true)
      expect(result!.classList.contains('header-api-health-result-fail')).toBe(true)
    })
  })

  it('does not show result before health check runs', () => {
    const onHealthCheck = vi.fn().mockResolvedValue(true)
    render(<ApiPill available={true} onHealthCheck={onHealthCheck} />)
    expect(screen.queryByText('reachable')).toBeNull()
    expect(screen.queryByText('unreachable')).toBeNull()
  })

  it('keeps last result visible during next health check', async () => {
    let resolve: (v: boolean) => void
    const onHealthCheck = vi.fn().mockImplementation(() => new Promise<boolean>(r => { resolve = r }))
    render(<ApiPill available={true} onHealthCheck={onHealthCheck} />)
    // First check to establish a result
    screen.getByTitle('Check API health').click()
    resolve!(true)
    await vi.waitFor(() => {
      expect(screen.getByTitle('Check API health').textContent).toBe('ok')
    })
    vi.advanceTimersByTime(1500)
    await vi.waitFor(() => {
      expect(screen.getByText('reachable')).toBeDefined()
    })
    // Second check — last result stays visible while checking
    screen.getByTitle('Check API health').click()
    await vi.waitFor(() => {
      expect(screen.getByTitle('Check API health').textContent).toBe('checking')
    })
    expect(screen.getByText('reachable')).toBeDefined()
  })

  it('updates result when new check completes with different outcome', async () => {
    const onHealthCheck = vi.fn().mockResolvedValueOnce(true).mockResolvedValueOnce(false)
    render(<ApiPill available={true} onHealthCheck={onHealthCheck} />)
    // First check: success
    screen.getByTitle('Check API health').click()
    await vi.waitFor(() => {
      expect(screen.getByTitle('Check API health').textContent).toBe('ok')
    })
    vi.advanceTimersByTime(1500)
    await vi.waitFor(() => {
      expect(screen.getByText('reachable')).toBeDefined()
    })
    // Second check: failure — result should update
    screen.getByTitle('Check API health').click()
    await vi.waitFor(() => {
      expect(screen.getByTitle('Check API health').textContent).toBe('fail')
    })
    vi.advanceTimersByTime(1500)
    await vi.waitFor(() => {
      expect(screen.getByText('unreachable')).toBeDefined()
    })
    expect(screen.queryByText('reachable')).toBeNull()
  })
})

describe('formatHealthTime', () => {
  it('formats time as HH:MM:SS', () => {
    const time = new Date(2026, 2, 30, 14, 5, 9)
    expect(formatHealthTime(time)).toBe('14:05:09')
  })

  it('zero-pads single digit values', () => {
    const time = new Date(2026, 0, 1, 1, 2, 3)
    expect(formatHealthTime(time)).toBe('01:02:03')
  })

  it('handles midnight', () => {
    const time = new Date(2026, 0, 1, 0, 0, 0)
    expect(formatHealthTime(time)).toBe('00:00:00')
  })
})

describe('ApiPill health timestamp', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    cleanup()
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('shows timestamp after health check completes', async () => {
    const onHealthCheck = vi.fn().mockResolvedValue(true)
    render(<ApiPill available={true} onHealthCheck={onHealthCheck} />)
    screen.getByTitle('Check API health').click()
    await vi.waitFor(() => {
      expect(screen.getByText('reachable')).toBeDefined()
    })
    const timeEl = document.querySelector('.header-api-health-time')
    expect(timeEl).not.toBeNull()
    expect(timeEl!.textContent).toMatch(/^\d{2}:\d{2}:\d{2}$/)
  })

  it('does not show timestamp before health check', () => {
    const onHealthCheck = vi.fn().mockResolvedValue(true)
    render(<ApiPill available={true} onHealthCheck={onHealthCheck} />)
    expect(document.querySelector('.header-api-health-time')).toBeNull()
  })

  it('shows timestamp after failed health check', async () => {
    const onHealthCheck = vi.fn().mockResolvedValue(false)
    render(<ApiPill available={true} onHealthCheck={onHealthCheck} />)
    screen.getByTitle('Check API health').click()
    await vi.waitFor(() => {
      expect(screen.getByText('unreachable')).toBeDefined()
    })
    const timeEl = document.querySelector('.header-api-health-time')
    expect(timeEl).not.toBeNull()
    expect(timeEl!.textContent).toMatch(/^\d{2}:\d{2}:\d{2}$/)
  })
})

describe('ApiPill health clear action', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    cleanup()
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('shows clear button when health result is visible', async () => {
    const onHealthCheck = vi.fn().mockResolvedValue(true)
    render(<ApiPill available={true} onHealthCheck={onHealthCheck} />)
    screen.getByTitle('Check API health').click()
    await vi.waitFor(() => {
      expect(screen.getByText('reachable')).toBeDefined()
    })
    const clearBtn = screen.getByTitle('Clear health result')
    expect(clearBtn).toBeDefined()
    expect(clearBtn.className).toBe('header-api-health-clear')
    expect(clearBtn.textContent).toBe('clear')
  })

  it('clears result, timestamp, and button status on click', async () => {
    const onHealthCheck = vi.fn().mockResolvedValue(true)
    render(<ApiPill available={true} onHealthCheck={onHealthCheck} />)
    screen.getByTitle('Check API health').click()
    await vi.waitFor(() => {
      expect(screen.getByText('reachable')).toBeDefined()
    })
    // Click clear during the ok window (before 1.5s reset)
    // — should fully clear all visible health state
    const btn = screen.getByTitle('Check API health')
    expect(btn.textContent).toBe('ok')
    act(() => {
      screen.getByTitle('Clear health result').click()
    })
    expect(screen.queryByText('reachable')).toBeNull()
    expect(document.querySelector('.header-api-health-time')).toBeNull()
    expect(screen.queryByTitle('Clear health result')).toBeNull()
    // Button should reset to ping immediately
    expect(btn.textContent).toBe('ping')
  })

  it('does not show clear button before health check', () => {
    const onHealthCheck = vi.fn().mockResolvedValue(true)
    render(<ApiPill available={true} onHealthCheck={onHealthCheck} />)
    expect(screen.queryByTitle('Clear health result')).toBeNull()
  })
})

describe('ApiPill health target change reset', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    cleanup()
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('clears health record when target changes to null', async () => {
    const onHealthCheck = vi.fn().mockResolvedValue(true)
    const { rerender } = render(<ApiPill available={true} onHealthCheck={onHealthCheck} />)
    screen.getByTitle('Check API health').click()
    await vi.waitFor(() => {
      expect(screen.getByText('reachable')).toBeDefined()
    })
    // Target changes: connected (localhost:8080) → neutral (null)
    rerender(<ApiPill available={null} onHealthCheck={onHealthCheck} />)
    // Re-render as connected — stale record should be gone
    rerender(<ApiPill available={true} onHealthCheck={onHealthCheck} />)
    expect(screen.queryByText('reachable')).toBeNull()
    expect(screen.queryByText('unreachable')).toBeNull()
    expect(document.querySelector('.header-api-health-time')).toBeNull()
  })

  it('clears health record including failure emphasis on target change', async () => {
    const onHealthCheck = vi.fn().mockResolvedValue(false)
    const { rerender } = render(<ApiPill available={true} onHealthCheck={onHealthCheck} />)
    screen.getByTitle('Check API health').click()
    await vi.waitFor(() => {
      expect(screen.getByText('unreachable')).toBeDefined()
    })
    expect(document.querySelector('.header-api-health-emphasis')).not.toBeNull()
    // Target changes
    rerender(<ApiPill available={null} onHealthCheck={onHealthCheck} />)
    rerender(<ApiPill available={true} onHealthCheck={onHealthCheck} />)
    expect(screen.queryByText('unreachable')).toBeNull()
    expect(document.querySelector('.header-api-health-emphasis')).toBeNull()
  })

  it('preserves health record when target stays the same', async () => {
    const onHealthCheck = vi.fn().mockResolvedValue(true)
    const { rerender } = render(<ApiPill available={true} onHealthCheck={onHealthCheck} />)
    screen.getByTitle('Check API health').click()
    await vi.waitFor(() => {
      expect(screen.getByText('reachable')).toBeDefined()
    })
    // Re-render with same available=true (same target)
    rerender(<ApiPill available={true} onHealthCheck={onHealthCheck} />)
    expect(screen.getByText('reachable')).toBeDefined()
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
