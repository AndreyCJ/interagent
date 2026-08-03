import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import TextInput from '../TextInput.vue'

function mountInput(props = {}) {
  return mount(TextInput, { props })
}

describe('TextInput', () => {
  it('emits submit with trimmed text on Enter', async () => {
    const wrapper = mountInput()
    await wrapper.find('input').setValue('  hello  ')
    await wrapper.find('input').trigger('keyup.enter')
    expect(wrapper.emitted('submit')).toHaveLength(1)
    expect(wrapper.emitted('submit')![0]).toEqual(['hello'])
  })

  it('clears input after submit', async () => {
    const wrapper = mountInput()
    await wrapper.find('input').setValue('hello')
    await wrapper.find('input').trigger('keyup.enter')
    expect((wrapper.find('input').element as HTMLInputElement).value).toBe('')
  })

  it('emits submit on button click', async () => {
    const wrapper = mountInput()
    await wrapper.find('input').setValue('hello')
    await wrapper.find('button').trigger('click')
    expect(wrapper.emitted('submit')).toHaveLength(1)
  })

  it('does not emit submit for blank text', async () => {
    const wrapper = mountInput()
    await wrapper.find('input').setValue('   ')
    await wrapper.find('input').trigger('keyup.enter')
    expect(wrapper.emitted('submit')).toBeUndefined()
  })

  it('is disabled in disabled state', async () => {
    const wrapper = mountInput({ disabled: true })
    await wrapper.find('input').setValue('hello')
    await wrapper.find('input').trigger('keyup.enter')
    expect(wrapper.emitted('submit')).toBeUndefined()
    expect(wrapper.find('button').attributes('disabled')).toBeDefined()
  })
})
