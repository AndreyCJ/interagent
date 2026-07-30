import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import OverlayWindow from '../OverlayWindow.vue'
import TranscriptionBar from '../TranscriptionBar.vue'
import AnswerBox from '../AnswerBox.vue'

describe('OverlayWindow', () => {
  it('renders with dark theme by default', () => {
    const wrapper = mount(OverlayWindow, {
      props: { visible: true },
      slots: { default: 'content' },
    })
    expect(wrapper.text()).toContain('content')
    expect(wrapper.classes()).toContain('theme-dark')
  })

  it('applies transparent theme class', () => {
    const wrapper = mount(OverlayWindow, {
      props: { visible: true, theme: 'transparent' },
    })
    expect(wrapper.classes()).toContain('theme-transparent')
  })
})

describe('TranscriptionBar', () => {
  it('shows text when provided', () => {
    const wrapper = mount(TranscriptionBar, {
      props: { text: 'hello world' },
    })
    expect(wrapper.text()).toContain('hello world')
  })

  it('shows placeholder when text is empty', () => {
    const wrapper = mount(TranscriptionBar, {
      props: { text: '', placeholder: 'Listening...' },
    })
    expect(wrapper.text()).toContain('Listening...')
  })
})

describe('AnswerBox', () => {
  it('shows answer text', () => {
    const wrapper = mount(AnswerBox, {
      props: { answer: '42', loading: false },
    })
    expect(wrapper.text()).toContain('42')
  })

  it('shows loading state', () => {
    const wrapper = mount(AnswerBox, {
      props: { answer: '', loading: true },
    })
    expect(wrapper.text()).toContain('...')
  })

  it('shows placeholder when empty and not loading', () => {
    const wrapper = mount(AnswerBox, {
      props: { answer: '', loading: false, placeholder: 'waiting' },
    })
    expect(wrapper.text()).toContain('waiting')
  })
})
