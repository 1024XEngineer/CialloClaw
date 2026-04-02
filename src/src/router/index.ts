import { createRouter, createWebHistory } from 'vue-router'
import OrbWindow from '@/windows/OrbWindow.vue'
import ChatWindow from '@/windows/ChatWindow.vue'
import SettingsWindow from '@/windows/SettingsWindow.vue'
import NudgeWindow from '@/windows/NudgeWindow.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/orb' },
    { path: '/orb', component: OrbWindow },
    { path: '/chat', component: ChatWindow },
    { path: '/settings', component: SettingsWindow },
    { path: '/nudge', component: NudgeWindow },
  ],
})

export default router
