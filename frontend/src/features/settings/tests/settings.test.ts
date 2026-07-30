import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import SettingsPanel from '../SettingsPanel.vue'
import AgentManager from '../AgentManager.vue'
import ShortcutEditor from '../ShortcutEditor.vue'
import { useSettings } from '../useSettings'
import type { AgentConfig, Shortcut } from '../../../types'

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
      { id: '1', name: 'Default', model: 'llama3', systemPrompt: '', temperature: 0.7 },
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
