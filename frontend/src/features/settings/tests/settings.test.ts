import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import type { AppSettings } from '../../../common/types/api.types'

const { mockWails } = vi.hoisted(() => ({
  mockWails: { GetSettings: vi.fn(), SaveSettings: vi.fn() },
}))

vi.mock('../../../common/utils/wails', () => mockWails)

import { useSettings } from '../useSettings'
import SettingsPanel from '../SettingsPanel.vue'

const base: AppSettings = {
  theme: 'transparent',
  language: 'en',
  shortcuts: [],
  autoStartListening: false,
  sttModel: 'base',
  sttLanguage: 'auto',
}

describe('useSettings', () => {
  beforeEach(() => vi.clearAllMocks())

  it('loads settings from the backend', async () => {
    mockWails.GetSettings.mockResolvedValue(base)
    const { settings, load } = useSettings()
    await load()
    expect(settings.value).toEqual(base)
  })

  it('normalizes the PascalCase binding payload to camelCase AppSettings', async () => {
    mockWails.GetSettings.mockResolvedValue({
      Theme: 'dark',
      Language: 'ru',
      Shortcuts: [],
      AutoStartListening: true,
      SttModel: 'small',
      SttLanguage: 'en',
    })
    const { settings, load } = useSettings()
    await load()
    expect(settings.value).toEqual({
      theme: 'dark',
      language: 'ru',
      shortcuts: [],
      autoStartListening: true,
      sttModel: 'small',
      sttLanguage: 'en',
    })
  })

  it('saves settings via the backend and keeps the local copy in sync', async () => {
    mockWails.SaveSettings.mockResolvedValue(undefined)
    const { settings, save } = useSettings()
    await save({ ...base, sttModel: 'small' })
    expect(mockWails.SaveSettings).toHaveBeenCalledWith({ ...base, sttModel: 'small' })
    expect(settings.value?.sttModel).toBe('small')
  })

  it('captures load errors', async () => {
    mockWails.GetSettings.mockRejectedValue(new Error('bind failed'))
    const { settings, error, load } = useSettings()
    await load()
    expect(settings.value).toBeNull()
    expect(error.value).toBe('bind failed')
  })
})

describe('SettingsPanel', () => {
  beforeEach(() => vi.clearAllMocks())

  it('renders STT model and language selects and saves on click', async () => {
    mockWails.GetSettings.mockResolvedValue(base)
    mockWails.SaveSettings.mockResolvedValue(undefined)
    const wrapper = mount(SettingsPanel)
    await flushPromises()
    const selects = wrapper.findAll('select')
    expect(selects).toHaveLength(2)
    expect((selects[0].element as HTMLSelectElement).value).toBe('base')
    expect((selects[1].element as HTMLSelectElement).value).toBe('auto')
    await wrapper.find('button').trigger('click')
    expect(mockWails.SaveSettings).toHaveBeenCalledWith(base)
  })
})
