# Bind-контракты (Frontend ↔ Backend)

**Дата:** 2026-08-01
**Статус:** Утверждён (черновик-контракт для Этапа 1)

---

## Правила

1. **Bind-методы — синхронные вызовы** frontend → backend (Wails bindings). Результат возвращается как `Promise`/`error`.
2. **Асинхронные результаты и статусы — только событиями** (см. `04-events.md`). Методы, которые запускают асинхронную работу, возвращают `void`/`error` (успех запуска), а результат приходит событием.
3. **Ошибки** возвращаются из метода как `error` (не событие `app:error`). `app:error` — для ошибок в фоновых асинхронных операциях.
4. **Типы данных** — источник правды: Go `internal/port/types.go` ↔ TS `frontend/src/common/types/api.types.ts`. Биндинги генерируются `wails generate` → `frontend/wailsjs/`.
5. **Изменение контракта** — через PR + ревью (03-process.md), затрагивающее ADR — через ADR (05-process / adr/README).

---

## Сессия (`SessionBind`)

| Метод            | Вход | Выход     | Ошибки             | События           | Примечание                                   |
| ---------------- | ---- | --------- | ------------------ | ----------------- | -------------------------------------------- |
| `NewSession()`   | —    | `Session` | storage            | `session:created` | Создаёт новую сессию, становится текущей     |
| `GetSession()`   | —    | `Session` | not found, storage | —                 | Возвращает текущую сессию (`id = "current"`) |
| `ClearSession()` | —    | —         | storage            | —                 | Очищает историю текущей сессии               |

## Аудио (`AudioBind`)

| Метод                | Вход         | Выход           | Ошибки             | События                          | Примечание                                        |
| -------------------- | ------------ | --------------- | ------------------ | -------------------------------- | ------------------------------------------------- |
| `StartListening()`   | —            | —               | permission, device | `audio:level`, `transcription:*` | Старт стрима (микрофон + системный звук, ADR-007) |
| `StopListening()`    | —            | —               | —                  | —                                | Останавливает стрим                               |
| `IsListening()`      | —            | `boolean`       | —                  | —                                | —                                                 |
| `GetAudioDevices()`  | —            | `AudioDevice[]` | —                  | —                                | Список микрофонов                                 |
| `SetAudioDevice(id)` | `id: string` | —               | not found          | —                                | Выбор активного микрофона                         |

## Скриншот (`ScreenshotBind`)

| Метод                                  | Вход                        | Выход    | Ошибки                   | События                                      | Примечание                                     |
| -------------------------------------- | --------------------------- | -------- | ------------------------ | -------------------------------------------- | ---------------------------------------------- |
| `CaptureFullScreen()`                  | —                           | —        | permission, capture      | `screenshot:captured`, `screenshot:error`    | Захват всего экрана → preview                  |
| `CaptureRegion()`                      | —                           | —        | permission, capture      | `screenshot:captured`, `screenshot:error`    | Захват выбранной области (селектор на фронте)  |
| `CaptureAndOCR(region)`                | `region: string`            | `string` | permission, capture, ocr | `ocr:done`                                   | Синхронный путь: захват → OCR → возврат текста |
| `SendImage(image)` _(этап 4, ADR-005)_ | `image: { base64, format }` | —        | —                        | `llm:response`, `llm:error`, `llm:cancelled` | Отправить скриншот как есть в LLM (multimodal) |

## LLM (`LLMBind`)

| Метод              | Вход           | Выход | Ошибки | События                                                     | Примечание                                 |
| ------------------ | -------------- | ----- | ------ | ----------------------------------------------------------- | ------------------------------------------ |
| `SendText(text)`   | `text: string` | —     | —      | `llm:started`, `llm:response`, `llm:error`, `llm:cancelled` | Отправка текста в активного агента         |
| `CancelResponse()` | —              | —     | —      | `llm:cancelled`                                             | Отмена текущей генерации (ADR-007, cancel) |

## Агенты (`AgentBind`)

| Метод                | Вход          | Выход           | Ошибки     | События         | Примечание                            |
| -------------------- | ------------- | --------------- | ---------- | --------------- | ------------------------------------- |
| `GetAgents()`        | —             | `AgentConfig[]` | storage    | —               | Список всех агентов                   |
| `GetActiveAgent()`   | —             | `AgentConfig`   | not found  | —               | Активный агент (используется для LLM) |
| `SetActiveAgent(id)` | `id: string`  | —               | not found  | `agent:changed` | Смена активного агента                |
| `SaveAgent(cfg)`     | `AgentConfig` | `AgentConfig`   | validation | —               | Создание/обновление (по `id`)         |
| `DeleteAgent(id)`    | `id: string`  | —               | not found  | —               | Удаление агента                       |

## Настройки (`SettingsBind`)

| Метод                      | Вход                 | Выход         | Ошибки     | События            | Примечание                                                 |
| -------------------------- | -------------------- | ------------- | ---------- | ------------------ | ---------------------------------------------------------- |
| `GetSettings()`            | —                    | `AppSettings` | storage    | —                  | Текущие настройки                                          |
| `SaveSettings(settings)`   | `AppSettings`        | —             | validation | `settings:updated` | Сохранение настроек                                        |
| `GetShortcuts()`           | —                    | `Shortcut[]`  | storage    | —                  | Шорткаты (из настроек)                                     |
| `UpdateShortcut(id, keys)` | `id, keys: string[]` | —             | validation | `settings:updated` | Обновление одного шортката (перерегистрация через Hotkeys) |

## Оверлей (`OverlayBind`, ADR-006) — этап 2

| Метод                  | Вход                                     | Выход    | Ошибки | События        | Примечание                  |
| ---------------------- | ---------------------------------------- | -------- | ------ | -------------- | --------------------------- |
| `ShowOverlay()`        | —                                        | —        | —      | —              | Показать окно               |
| `HideOverlay()`        | —                                        | —        | —      | —              | Скрыть окно                 |
| `SetOverlayMode(mode)` | `mode: 'click-through' \| 'interactive'` | —        | —      | `overlay:mode` | Переключение кликабельности |
| `GetOverlayMode()`     | —                                        | `string` | —      | —              | Текущий режим               |

## Хоткеи (`HotkeysBind`, ADR-006) — этап 2

| Метод                      | Вход                 | Выход | Ошибки                 | События | Примечание                                                |
| -------------------------- | -------------------- | ----- | ---------------------- | ------- | --------------------------------------------------------- |
| `RegisterHotkey(id, keys)` | `id, keys: string[]` | —     | validation, permission | —       | Регистрация глобального шортката (Accessibility, ADR-008) |
| `UnregisterHotkey(id)`     | `id: string`         | —     | —                      | —       | Снятие регистрации                                        |

## Разрешения (`PermissionsBind`, ADR-008)

| Метод                       | Вход            | Выход     | Ошибки | События          | Примечание                       |
| --------------------------- | --------------- | --------- | ------ | ---------------- | -------------------------------- |
| `GetPermissionStatus(p)`    | `p: Permission` | `boolean` | —      | `app:permission` | Выдано ли разрешение             |
| `RequestPermission(p)`      | `p: Permission` | `boolean` | —      | `app:permission` | Системный запрос (если возможно) |
| `OpenPermissionSettings(p)` | `p: Permission` | —         | —      | —                | Открыть нужную панель Настроек   |

`Permission = 'microphone' | 'screen-recording' | 'accessibility'`
