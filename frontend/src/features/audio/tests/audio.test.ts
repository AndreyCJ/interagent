import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import MicButton from '../MicButton.vue'
import { useAudio } from '../useAudio'

describe('MicButton', () => {
  it('shows Mic when not listening', () => {
    const wrapper = mount(MicButton, {
      props: { listening: false },
    })

    expect(wrapper.text()).toContain('Mic')
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
  it('starts with isListening false', () => {
    const { isListening } = useAudio()
    expect(isListening.value).toBe(false)
  })

  it('sets isListening true after start', async () => {
    const { isListening, start } = useAudio()
    await start()
    expect(isListening.value).toBe(true)
  })

  it('sets isListening false after stop', async () => {
    const { isListening, start, stop } = useAudio()
    await start()
    await stop()
    expect(isListening.value).toBe(false)
  })
})
