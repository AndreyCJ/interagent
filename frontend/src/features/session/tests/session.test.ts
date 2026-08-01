import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import type { Message } from '../../../common/types/api.types.js'
import ChatHistory from '../ChatHistory.vue'
import MessageItem from '../MessageItem.vue'

describe('MessageItem', () => {
  it('renders message text and role', () => {
    const msg: Message = { role: 'assistant', text: 'hello', timestamp: 100 }
    const wrapper = mount(MessageItem, {
      props: { message: msg },
    })
    expect(wrapper.text()).toContain('hello')
    expect(wrapper.text()).toContain('assistant')
    expect(wrapper.classes()).toContain('assistant')
  })

  it('applies role class for interviewer', () => {
    const msg: Message = { role: 'interviewer', text: 'question?', timestamp: 200 }
    const wrapper = mount(MessageItem, {
      props: { message: msg },
    })
    expect(wrapper.classes()).toContain('interviewer')
  })
})

describe('ChatHistory', () => {
  it('renders list of messages', () => {
    const messages: Message[] = [
      { role: 'interviewer', text: 'q1', timestamp: 1 },
      { role: 'assistant', text: 'a1', timestamp: 2 },
    ]
    const wrapper = mount(ChatHistory, {
      props: { messages },
    })
    expect(wrapper.findAllComponents(MessageItem).length).toBe(2)
  })

  it('renders empty state', () => {
    const wrapper = mount(ChatHistory, {
      props: { messages: [] },
    })
    expect(wrapper.findAllComponents(MessageItem).length).toBe(0)
  })
})
