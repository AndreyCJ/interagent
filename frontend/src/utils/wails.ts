import type { Session, AgentConfig, AppSettings, Shortcut, AudioDevice } from '../types'

// Session
export declare function NewSession(): Promise<Session>
export declare function GetSession(): Promise<Session>
export declare function ClearSession(): Promise<void>

// Audio
export declare function StartListening(): Promise<void>
export declare function StopListening(): Promise<void>
export declare function IsListening(): Promise<boolean>
export declare function GetAudioDevices(): Promise<AudioDevice[]>
export declare function SetAudioDevice(id: string): Promise<void>

// Screenshot
export declare function CaptureFullScreen(): Promise<void>
export declare function CaptureRegion(): Promise<void>
export declare function CaptureAndOCR(region?: string): Promise<string>

// LLM
export declare function SendText(text: string): Promise<void>
export declare function CancelResponse(): Promise<void>

// Agents
export declare function GetAgents(): Promise<AgentConfig[]>
export declare function GetActiveAgent(): Promise<AgentConfig>
export declare function SetActiveAgent(id: string): Promise<void>
export declare function SaveAgent(cfg: AgentConfig): Promise<AgentConfig>
export declare function DeleteAgent(id: string): Promise<void>

// Settings
export declare function GetSettings(): Promise<AppSettings>
export declare function SaveSettings(s: AppSettings): Promise<void>
export declare function GetShortcuts(): Promise<Shortcut[]>
export declare function UpdateShortcut(id: string, keys: string[]): Promise<void>

// App
export declare function GetVersion(): Promise<string>
export declare function Quit(): Promise<void>
