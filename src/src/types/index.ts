export type OrbStatus = 'idle' | 'helping' | 'nudging' | 'working'

export interface SidecarInfo {
  baseUrl: string
  source: string
}

export type OrbEdge = 'left' | 'right'
export type MotionCue = 'idle' | 'snap' | 'chat-bloom' | 'nudge-bloom'

export interface WindowMotionState {
  orbX: number
  orbY: number
  orbWidth: number
  orbHeight: number
  edge: OrbEdge
  chatX: number
  chatY: number
  nudgeX: number
  nudgeY: number
  cue: MotionCue
}

export interface HomeStatus {
  status: OrbStatus
  title: string
  subtitle: string
  mood: string
}

export interface NudgeAction {
  id: string
  label: string
  intent: 'view' | 'later' | 'ignore'
  tone?: 'primary' | 'secondary' | 'ghost'
}

export interface NudgeItem {
  id: string
  level: 'L1' | 'L2' | 'L3'
  title: string
  summary: string
  scene: string
  actions: NudgeAction[]
}

export interface ScenarioItem {
  id: string
  category: string
  title: string
  summary: string
  detail: string
  risk: 'green' | 'yellow' | 'red'
  suggestedAction: string
}

export interface SettingsSection {
  id: string
  title: string
  description: string
  items: Array<{
    id: string
    label: string
    description: string
    type: 'toggle' | 'choice' | 'info'
    value: string | boolean
    options?: string[]
  }>
}
