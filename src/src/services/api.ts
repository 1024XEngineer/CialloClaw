import type { HomeStatus, NudgeItem, ScenarioItem, SettingsSection } from '@/types'
import { getSidecarInfo } from './tauri'

async function fetchJson<T>(path: string): Promise<T> {
  const { baseUrl } = await getSidecarInfo()
  const response = await fetch(`${baseUrl}${path}`)
  if (!response.ok) {
    throw new Error(`Request failed: ${path}`)
  }
  return response.json() as Promise<T>
}

export async function fetchHomeStatus(): Promise<HomeStatus> {
  return fetchJson<HomeStatus>('/api/home')
}

export async function fetchNudges(): Promise<NudgeItem[]> {
  return fetchJson<NudgeItem[]>('/api/nudges')
}

export async function fetchScenarios(): Promise<ScenarioItem[]> {
  return fetchJson<ScenarioItem[]>('/api/scenarios')
}

export async function fetchSettings(): Promise<SettingsSection[]> {
  return fetchJson<SettingsSection[]>('/api/settings')
}

export async function performAction(action: string, payload?: Record<string, unknown>) {
  const { baseUrl } = await getSidecarInfo()
  const response = await fetch(`${baseUrl}/api/actions/${action}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload ?? {}),
  })
  if (!response.ok) {
    throw new Error(`Action failed: ${action}`)
  }
  return response.json()
}
