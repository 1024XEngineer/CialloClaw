import { escapeHtml } from '../../utils.js';

export function renderSettingsPanel(state, data) {
  const settings = state.data.settings;

  return `
    <div class="workspace-grid workspace-grid-settings">
      <section class="settings-hero glass-panel">
        <div class="panel-kicker">设置模式</div>
        <h2>轻量占位，不做重平台</h2>
        <p>这里主要放悬浮球显示、提醒开关、模型 / 提供商占位、工作区路径和快捷键占位。</p>
      </section>

      <section class="settings-grid">
        <article class="settings-card glass-panel">
          <div class="section-title">悬浮球显示设置</div>
          <label class="toggle-row"><input type="checkbox" ${settings.ballVisible ? 'checked' : ''} data-action="setting-toggle" data-setting="ballVisible" /> 显示悬浮球</label>
          <label class="toggle-row"><input type="checkbox" ${settings.reminderEnabled ? 'checked' : ''} data-action="setting-toggle" data-setting="reminderEnabled" /> 提醒开关</label>
        </article>
        <article class="settings-card glass-panel">
          <div class="section-title">模型 / 提供商占位</div>
          <div class="setting-line"><span>提供商</span><strong>${escapeHtml(settings.provider)}</strong></div>
          <div class="setting-line"><span>模型</span><strong>${escapeHtml(settings.model)}</strong></div>
          <div class="setting-line"><span>工作区路径</span><strong>${escapeHtml(settings.workspacePath)}</strong></div>
          <div class="setting-line"><span>快捷键</span><strong>${escapeHtml(settings.hotkey)}</strong></div>
        </article>
        <article class="settings-card glass-panel">
          <div class="section-title">记忆 / 长期协作</div>
          <label class="toggle-row"><input type="checkbox" ${settings.memoryEnabled ? 'checked' : ''} data-action="setting-toggle" data-setting="memoryEnabled" /> 记忆能力</label>
          <div class="memory-note-list">
            ${data.memoryNotes.map((note) => `<div class="memory-note">${escapeHtml(note)}</div>`).join('')}
          </div>
        </article>
        <article class="settings-card glass-panel">
          <div class="section-title">关于 / 版本信息</div>
          <div class="setting-line"><span>版本</span><strong>${escapeHtml(settings.version)}</strong></div>
          <div class="setting-line"><span>定位</span><strong>桌面常驻、轻提示承接、工作台深入</strong></div>
        </article>
      </section>
    </div>
  `;
}
