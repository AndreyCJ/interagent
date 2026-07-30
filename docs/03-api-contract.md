# API Contract

**Дата:** 2026-07-30
**Статус:** Черновик

---

## Data Types

### Message

```typescript
interface Message {
  role: "interviewer" | "user" | "assistant";
  text: string;
  timestamp: number;
}
```

- `interviewer` — транскрипция голоса интервьювера (STT)
- `user` — ручной ввод / комментарий кандидата
- `assistant` — ответ LLM

### Session

```typescript
interface Session {
  id: string;
  chatHistory: Message[];
  startedAt: number;
}
```

### AgentConfig

```typescript
interface AgentConfig {
  id: string;
  name: string;
  model: string; // из списка доступных (llama3-8b, codellama, ...)
  systemPrompt: string; // системный промпт
  temperature: number;
}
```

### AppSettings

```typescript
interface AppSettings {
  theme: "dark" | "light" | "transparent";
  language: string;
  shortcuts: Shortcut[];
  autoStartListening: boolean;
}
```

### Shortcut

```typescript
interface Shortcut {
  id: string; // 'mic_toggle'
  label: string; // 'Вкл/Выкл микрофон'
  keys: string[]; // ['Control', 'Shift', 'M']
  enabled: boolean;
}
```

### AudioDevice

```typescript
interface AudioDevice {
  id: string;
  name: string;
  isDefault: boolean;
}
```

---

## Bind-методы (Frontend → Backend)

### Session

| Метод            | Принимает | Возвращает | Описание                             |
| ---------------- | --------- | ---------- | ------------------------------------ |
| `NewSession()`   | —         | `Session`  | Создать новую сессию, вернуть пустую |
| `GetSession()`   | —         | `Session`  | Текущая сессия с историей            |
| `ClearSession()` | —         | `void`     | Сбросить историю текущей сессии      |

### Audio

| Метод                       | Принимает | Возвращает      | Описание                                        |
| --------------------------- | --------- | --------------- | ----------------------------------------------- |
| `StartListening()`          | —         | `void`          | Захват микрофона → STT (результат через events) |
| `StopListening()`           | —         | `void`          | Остановить захват                               |
| `IsListening()`             | —         | `bool`          | Статус микрофона                                |
| `GetAudioDevices()`         | —         | `AudioDevice[]` | Список устройств ввода                          |
| `SetAudioDevice(id string)` | `id`      | `void`          | Выбрать устройство                              |

### Screenshot

| Метод                    | Принимает | Возвращает | Описание                                 |
| ------------------------ | --------- | ---------- | ---------------------------------------- |
| `CaptureFullScreen()`    | —         | `void`     | Скриншот всего экрана → LLM              |
| `CaptureRegion()`        | —         | `void`     | Выбор области → LLM                      |
| `CaptureAndOCR(region?)` | `region?` | `string`   | Скриншот → OCR → вернуть текст (без LLM) |

### LLM

| Метод                   | Принимает | Возвращает | Описание                                     |
| ----------------------- | --------- | ---------- | -------------------------------------------- |
| `SendText(text string)` | `text`    | `void`     | Отправить текст в LLM, ответ придёт событием |
| `CancelResponse()`      | —         | `void`     | Отменить генерацию                           |

### Agents

| Метод                        | Принимает | Возвращает      | Описание               |
| ---------------------------- | --------- | --------------- | ---------------------- |
| `GetAgents()`                | —         | `AgentConfig[]` | Список всех агентов    |
| `GetActiveAgent()`           | —         | `AgentConfig`   | Текущий активный агент |
| `SetActiveAgent(id string)`  | `id`      | `void`          | Переключить агента     |
| `SaveAgent(cfg AgentConfig)` | `cfg`     | `AgentConfig`   | Создать или обновить   |
| `DeleteAgent(id string)`     | `id`      | `void`          | Удалить агента         |

### Settings

| Метод                                      | Принимает  | Возвращает    | Описание                   |
| ------------------------------------------ | ---------- | ------------- | -------------------------- |
| `GetSettings()`                            | —          | `AppSettings` | Текущие настройки          |
| `SaveSettings(s AppSettings)`              | `s`        | `void`        | Сохранить настройки        |
| `GetShortcuts()`                           | —          | `Shortcut[]`  | Список шорткатов           |
| `UpdateShortcut(id string, keys string[])` | `id, keys` | `void`        | Обновить комбинацию клавиш |

### App

| Метод          | Принимает | Возвращает | Описание           |
| -------------- | --------- | ---------- | ------------------ |
| `GetVersion()` | —         | `string`   | Версия приложения  |
| `Quit()`       | —         | `void`     | Закрыть приложение |

---

## Events (Backend → Frontend)

### Сессия

| Event             | Данные    | Когда                |
| ----------------- | --------- | -------------------- |
| `session:created` | `Session` | Новая сессия создана |

### Аудио / STT

| Event                   | Данные                                 | Когда                         |
| ----------------------- | -------------------------------------- | ----------------------------- |
| `audio:level`           | `{ level: number }`                    | Уровень микрофона (throttled) |
| `transcription:partial` | `{ text: string }`                     | Промежуточный текст STT       |
| `transcription:done`    | `{ text: string, confidence: number }` | Финальный текст STT           |

### Скриншот / OCR

| Event                 | Данные               | Когда                    |
| --------------------- | -------------------- | ------------------------ |
| `screenshot:captured` | `{ base64: string }` | Скриншот готов (preview) |
| `screenshot:error`    | `{ error: string }`  | Ошибка захвата           |
| `ocr:done`            | `{ text: string }`   | OCR завершён             |

### LLM

| Event           | Данные              | Когда              |
| --------------- | ------------------- | ------------------ |
| `llm:response`  | `{ text: string }`  | Полный ответ LLM   |
| `llm:error`     | `{ error: string }` | Ошибка LLM         |
| `llm:cancelled` | `{}`                | Генерация отменена |

### Общие

| Event       | Данные                             | Когда        |
| ----------- | ---------------------------------- | ------------ |
| `app:error` | `{ stage: string, error: string }` | Любая ошибка |
