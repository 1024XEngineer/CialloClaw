import { defineStore } from 'pinia'
import { fetchHomeStatus, fetchNudges, fetchScenarios, fetchSettings, performAction } from '@/services/api'
import { getMotionState, subscribeWindowMotion } from '@/services/tauri'
import type { HomeStatus, NudgeItem, ScenarioItem, SettingsSection, WindowMotionState } from '@/types'

const defaultMotionState = (): WindowMotionState => ({
  orbX: 24,
  orbY: 140,
  orbWidth: 78,
  orbHeight: 78,
  edge: 'right',
  chatX: 0,
  chatY: 0,
  nudgeX: 0,
  nudgeY: 0,
  cue: 'idle',
})

export const useAppStore = defineStore('app', {
  state: () => ({
    home: null as HomeStatus | null,
    nudges: [] as NudgeItem[],
    scenarios: [] as ScenarioItem[],
    settings: [] as SettingsSection[],
    loading: false,
    motion: defaultMotionState(),
    motionBridgeReady: false,
  }),
  actions: {
    async hydrateHome() {
      this.home = await fetchHomeStatus()
    },
    async hydrateNudges() {
      this.nudges = await fetchNudges()
    },
    async hydrateScenarios() {
      this.scenarios = await fetchScenarios()
    },
    async hydrateSettings() {
      this.settings = await fetchSettings()
    },
    async hydrateAll() {
      this.loading = true
      try {
        await Promise.all([
          this.hydrateHome(),
          this.hydrateNudges(),
          this.hydrateScenarios(),
          this.hydrateSettings(),
        ])
      } finally {
        this.loading = false
      }
    },
    async trigger(action: string, payload?: Record<string, unknown>) {
      return performAction(action, payload)
    },
    applyMotion(payload: WindowMotionState) {
      this.motion = payload
    },
    async ensureMotionBridge() {
      if (this.motionBridgeReady) return
      this.motionBridgeReady = true
      this.applyMotion(await getMotionState())
      await subscribeWindowMotion((payload) => {
        this.applyMotion(payload)
      })
    },
  },
})
