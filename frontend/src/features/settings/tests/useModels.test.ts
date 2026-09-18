import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockWails } = vi.hoisted(() => ({
  mockWails: {
    DownloadSTTModel: vi.fn(),
    GetModelStatus: vi.fn(),
  },
}))

const handlers: Record<string, (payload: unknown) => void> = {}

vi.mock('../../../common/utils/wails', () => mockWails)
vi.mock('../../../common/utils/events', () => ({
  onEvent: (name: string, cb: (payload: unknown) => void) => {
    handlers[name] = cb
  },
}))

import { useModels } from '../useModels'

function fire(name: string, payload?: unknown) {
  handlers[name]?.(payload)
}

describe('useModels', () => {
  beforeEach(() => {
    Object.keys(handlers).forEach(k => delete handlers[k])
    vi.clearAllMocks()
  })

  it('tracks download progress per model', () => {
    const models = useModels()
    fire('model:download-progress', { model: 'ggml-base', received: 50, total: 100 })
    expect(models.progress.value['ggml-base']).toBe(50)
  })

  it('marks a model installed after model:downloaded and finishes progress', () => {
    const models = useModels()
    fire('model:downloaded', { model: 'ggml-base' })
    expect(models.status.value['ggml-base']).toBe(true)
    expect(models.progress.value['ggml-base']).toBe(100)
  })

  it('records models app:error', () => {
    const models = useModels()
    fire('app:error', { stage: 'models', error: 'network down' })
    expect(models.error.value).toBe('network down')
  })

  it('downloads models through the backend', async () => {
    mockWails.DownloadSTTModel.mockResolvedValue(undefined)
    const models = useModels()
    await models.download()
    expect(mockWails.DownloadSTTModel).toHaveBeenCalledTimes(1)
    expect(models.error.value).toBeNull()
  })

  it('refresh loads the model status from the backend', async () => {
    mockWails.GetModelStatus.mockResolvedValue({ installed: true, path: '/m/ggml-base.bin' })
    const models = useModels()
    await models.refresh('base')
    expect(mockWails.GetModelStatus).toHaveBeenCalledWith('base')
    expect(models.status.value['base']).toBe(true)
  })
})
