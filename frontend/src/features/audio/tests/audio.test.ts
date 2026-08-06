import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import MicButton from '../MicButton.vue'
import { useAudio } from '../useAudio'

const { mockWails } = vi.hoisted(() => ({
  mockWails: {
    IsListening: vi.fn(),
    StartListening: vi.fn(),
    StopListening: vi.fn(),
    OpenPermissionSettings: vi.fn(),
  },
}))

vi.mock('../../../common/utils/wails', () => mockWails)

describe('MicButton', () => {
  it('shows Listen when not listening', () => {
    const wrapper = mount(MicButton, {
      props: { listening: false },
    })

    expect(wrapper.text()).toContain('Listen')
    expect(wrapper.classes()).not.toContain('active')
  })

  it('shows Stop when listening', () => {
    const wrapper = mount(MicButton, {
      props: { listening: true },
    })

    expect(wrapper.text()).toContain('Stop')
    expect(wrapper.classes()).toContain('active')
  })

  it('emits toggle on click', () => {
    const wrapper = mount(MicButton, {
      props: { listening: false },
    })
    wrapper.trigger('click')
    expect(wrapper.emitted('toggle')).toBeTruthy()
  })
})

describe('useAudio', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('starts with isListening false and no error', () => {
    const { isListening, error } = useAudio()
    expect(isListening.value).toBe(false)
    expect(error.value).toBeNull()
  })

  it('load seeds state from IsListening', async () => {
    mockWails.IsListening.mockResolvedValue(true)
    const { isListening, load } = useAudio()
    await load()
    expect(isListening.value).toBe(true)
    expect(mockWails.IsListening).toHaveBeenCalledTimes(1)
  })

  it('toggle starts listening when off', async () => {
    mockWails.IsListening.mockResolvedValue(false)
    mockWails.StartListening.mockResolvedValue(undefined)
    const { isListening, load, toggle } = useAudio()
    await load()
    await toggle()
    expect(mockWails.StartListening).toHaveBeenCalledTimes(1)
    expect(mockWails.StopListening).not.toHaveBeenCalled()
    expect(isListening.value).toBe(true)
  })

  it('toggle stops listening when on', async () => {
    mockWails.IsListening.mockResolvedValue(true)
    mockWails.StopListening.mockResolvedValue(undefined)
    const { isListening, load, toggle } = useAudio()
    await load()
    await toggle()
    expect(mockWails.StopListening).toHaveBeenCalledTimes(1)
    expect(mockWails.StartListening).not.toHaveBeenCalled()
    expect(isListening.value).toBe(false)
  })

  it('captures error and keeps state unchanged on failure', async () => {
    mockWails.IsListening.mockResolvedValue(false)
    mockWails.StartListening.mockRejectedValue(new Error('microphone permission required'))
    const { isListening, error, load, toggle } = useAudio()
    await load()
    await toggle()
    expect(error.value).toBe('microphone permission required')
    expect(isListening.value).toBe(false)
  })

  it('ignores toggle while busy', async () => {
    mockWails.IsListening.mockResolvedValue(false)
    let resolveStart!: () => void
    mockWails.StartListening.mockImplementation(
      () => new Promise<void>(resolve => (resolveStart = resolve)),
    )
    const { toggle, busy } = useAudio()
    const first = toggle()
    expect(busy.value).toBe(true)
    await toggle()
    expect(mockWails.StartListening).toHaveBeenCalledTimes(1)
    resolveStart()
    await first
    expect(busy.value).toBe(false)
  })

  it('openSettings opens the screen-recording settings pane', async () => {
    mockWails.OpenPermissionSettings.mockResolvedValue(undefined)
    const { openSettings } = useAudio()
    await openSettings()
    expect(mockWails.OpenPermissionSettings).toHaveBeenCalledWith('screen-recording')
  })
})
