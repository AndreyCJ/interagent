import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { OverlayMode } from '../../../common/types/api.types'

const { mockWails } = vi.hoisted(() => ({
  mockWails: {
    ShowOverlay: vi.fn(),
    HideOverlay: vi.fn(),
    SetOverlayMode: vi.fn(),
    GetOverlayMode: vi.fn(),
  },
}))

const handlers: Record<string, (payload: unknown) => void> = {}

vi.mock('../../../common/utils/wails', () => mockWails)
vi.mock('../../../common/utils/events', () => ({
  onEvent: (name: string, cb: (payload: unknown) => void) => {
    handlers[name] = cb
  },
}))

import { useOverlay } from '../useOverlay'

function fireMode(payload: { mode: OverlayMode }) {
  handlers['overlay:mode']?.(payload)
}

describe('useOverlay', () => {
  beforeEach(() => {
    handlers['overlay:mode'] = () => {}
    vi.clearAllMocks()
  })

  it('defaults to click-through mode', () => {
    const { mode } = useOverlay()
    expect(mode.value).toBe('click-through')
  })

  it('updates mode on overlay:mode event', () => {
    const { mode } = useOverlay()
    fireMode({ mode: 'interactive' })
    expect(mode.value).toBe('interactive')
  })

  it('loadMode reads current mode from backend', async () => {
    mockWails.GetOverlayMode.mockResolvedValue('interactive')
    const { mode, loadMode } = useOverlay()
    await loadMode()
    expect(mode.value).toBe('interactive')
    expect(mockWails.GetOverlayMode).toHaveBeenCalledTimes(1)
  })

  it('setMode delegates to backend', async () => {
    mockWails.SetOverlayMode.mockResolvedValue(undefined)
    const { setMode } = useOverlay()
    await setMode('click-through')
    expect(mockWails.SetOverlayMode).toHaveBeenCalledWith('click-through')
  })

  it('show and hide delegate to backend', async () => {
    const { show, hide } = useOverlay()
    await show()
    await hide()
    expect(mockWails.ShowOverlay).toHaveBeenCalledTimes(1)
    expect(mockWails.HideOverlay).toHaveBeenCalledTimes(1)
  })
})
