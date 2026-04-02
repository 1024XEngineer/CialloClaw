import { escapeHtml } from '../utils.js';

export function renderQuickChat(state) {
  if (!state.ui.quickChat.open) return '';
  const lastUser = state.ui.quickChat.user || '还没有发送过内容';
  const lastAgent = state.ui.quickChat.agent || 'Agent 会在这里给出最近一轮回复';

  return `
    <aside class="quick-chat-panel glass-panel" aria-label="轻提示对话层" data-drop-zone="quick-chat">
      <button type="button" class="icon-btn quick-chat-close" data-action="close-quick-chat" aria-label="关闭">×</button>

      <div class="quick-chat-stack">
        <div class="mini-bubble mini-bubble-user">
          <span class="mini-bubble__label">你最近一条输入</span>
          <div class="mini-bubble__body">${escapeHtml(lastUser)}</div>
        </div>
        <div class="mini-bubble mini-bubble-agent">
          <span class="mini-bubble__label">Agent 最近一条回复</span>
          <div class="mini-bubble__body">${escapeHtml(lastAgent)}</div>
        </div>
      </div>

      <div class="quick-chat-input-row">
        <input type="text" class="text-input" placeholder="Enter 直接发送" data-action="quick-chat-input" />
        <button type="button" class="primary-btn" data-action="quick-chat-send">发送</button>
      </div>
    </aside>
  `;
}
