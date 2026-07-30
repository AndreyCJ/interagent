import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import ScreenshotButton from '../ScreenshotButton.vue'
import { useScreenshot } from '../useScreenshot'

describe('ScreenshotButton', () => {
  it('renders two buttons', () => {
    const wrapper = mount(ScreenshotButton)
    const buttons = wrapper.findAll('button')
    expect(buttons.length).toBe(2)
    expect(buttons[0].text()).toContain('Full Screen')
    expect(buttons[1].text()).toContain('Region')
  })

  it('emits captureFull on first button click', () => {
    const wrapper = mount(ScreenshotButton)
    wrapper.findAll('button')[0].trigger('click')
    expect(wrapper.emitted('captureFull')).toBeTruthy()
  })

  it('emits captureRegion on second button click', () => {
    const wrapper = mount(ScreenshotButton)
    wrapper.findAll('button')[1].trigger('click')
    expect(wrapper.emitted('captureRegion')).toBeTruthy()
  })
})

describe('useScreenshot', () => {
  it('starts with capturing false', () => {
    const { capturing } = useScreenshot()
    expect(capturing.value).toBe(false)
  })

  it('sets capturing true after captureFull', async () => {
    const { capturing, captureFull } = useScreenshot()
    await captureFull()
    expect(capturing.value).toBe(true)
  })
})
