import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import type { AgentConfig, Shortcut } from '../../../common/types/api.types.js'
import AgentManager from '../AgentManager.vue'
import SettingsPanel from '../SettingsPanel.vue'
import ShortcutEditor from '../ShortcutEditor.vue'
import { useSettings } from '../useSettings'

describe('SettingsPanel', () => {
  it('renders slot content', () => {
    const wrapper = mount(SettingsPanel, {
      slots: { default: 'settings content' },
    })
    expect(wrapper.text()).toContain('settings content')
  })
})

describe('AgentManager', () => {
  it('renders list of agents', () => {
    const agents: AgentConfig[] = [
      {
        id: '1',
        name: 'Default',
        provider: 'local',
        model: 'llama3',
        baseUrl: '',
        apiKey: '',
        systemPrompt: '',
        temperature: 0.7,
      },
    ]
    const wrapper = mount(AgentManager, {
      props: { agents },
    })
    expect(wrapper.text()).toContain('Default')
  })

  it('renders empty state', () => {
    const wrapper = mount(AgentManager, {
      props: { agents: [] },
    })
    expect(wrapper.findAll('.agent').length).toBe(0)
  })
})

describe('ShortcutEditor', () => {
  it('renders shortcuts with keys', () => {
    const shortcuts: Shortcut[] = [
      { id: 'mic_toggle', label: 'Toggle Mic', keys: ['Ctrl', 'Shift', 'M'], enabled: true },
    ]
    const wrapper = mount(ShortcutEditor, {
      props: { shortcuts },
    })
    expect(wrapper.text()).toContain('Toggle Mic')
    expect(wrapper.text()).toContain('Ctrl + Shift + M')
  })
})

describe('useSettings', () => {
  it('starts with null settings', () => {
    const { settings } = useSettings()
    expect(settings.value).toBeNull()
  })

  it('updates settings on save', async () => {
    const { settings, save } = useSettings()
    const s = { theme: 'dark' as const, language: 'en', shortcuts: [], autoStartListening: false }
    await save(s)
    expect(settings.value).toEqual(s)
  })
})
