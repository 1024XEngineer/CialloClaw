import { invoke } from '@tauri-apps/api/core'
import { listen } from '@tauri-apps/api/event'
import { getCurrentWebviewWindow } from '@tauri-apps/api/webviewWindow'
import type { SidecarInfo, WindowMotionState } from '@/types'

export async function getSidecarInfo(): Promise<SidecarInfo> {
  try {
    return await invoke<SidecarInfo>('get_sidecar_info')
  } catch {
    return { baseUrl: 'http://127.0.0.1:47831', source: 'fallback' }
  }
}

export async function invokeVoid(command: string, payload?: Record<string, unknown>) {
  return invoke(command, payload)
}

export async function getMotionState(): Promise<WindowMotionState> {
  return invoke<WindowMotionState>('get_motion_state')
}

export async function subscribeWindowMotion(handler: (payload: WindowMotionState) => void) {
  return listen<WindowMotionState>('window-motion', (event) => {
    handler(event.payload)
  })
}

export async function startDrag() {
  try {
    await getCurrentWebviewWindow().startDragging()
  } catch {
    // ignore in browser preview
  }
}
