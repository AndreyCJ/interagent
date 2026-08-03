export {
  GetAgents,
  GetActiveAgent,
  SetActiveAgent,
  SaveAgent,
  DeleteAgent,
} from '../../../wailsjs/go/main/AgentBind'

export {
  GetAudioDevices,
  IsListening,
  SetAudioDevice,
  StartListening,
  StopListening,
} from '../../../wailsjs/go/main/AudioBind'

export { GetVersion, Quit } from '../../../wailsjs/go/main/App'

export { CancelResponse, SendText } from '../../../wailsjs/go/main/LLMBind'

export {
  CaptureAndOCR,
  CaptureFullScreen,
  CaptureRegion,
} from '../../../wailsjs/go/main/ScreenshotBind'

export { ClearSession, GetSession, NewSession } from '../../../wailsjs/go/main/SessionBind'

export {
  GetSettings,
  GetShortcuts,
  SaveSettings,
  UpdateShortcut,
} from '../../../wailsjs/go/main/SettingsBind'

export {
  GetOverlayMode,
  HideOverlay,
  SetOverlayMode,
  ShowOverlay,
} from '../../../wailsjs/go/main/OverlayBind'

export { RegisterHotkey, UnregisterHotkey } from '../../../wailsjs/go/main/HotkeysBind'

export {
  GetPermissionStatus,
  OpenPermissionSettings,
  RequestPermission,
} from '../../../wailsjs/go/main/PermissionsBind'
