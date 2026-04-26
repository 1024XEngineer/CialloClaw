import { FilePlus2, NotebookText, RefreshCcw, Save, ScanSearch, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import type { NoteListItem, SourceNoteEditorDraft } from "../notePage.types";

type SourceNoteStudioProps = {
  availabilityMessage: string | null;
  draft: SourceNoteEditorDraft;
  editingItem: NoteListItem | null;
  fileLabel: string | null;
  isCreating: boolean;
  isDirty: boolean;
  isInspecting: boolean;
  isLoading: boolean;
  isSaving: boolean;
  onChange: (draft: SourceNoteEditorDraft) => void;
  onClose?: () => void;
  onCreate: () => void;
  onInspect: () => void;
  onReload: () => void;
  onSave: () => void;
  sourceRoots: string[];
  syncMessage: string | null;
};

const bucketLabels = {
  closed: "已结束",
  later: "后续",
  recurring_rule: "重复",
  upcoming: "近期",
} as const;

function formatMetadataValue(value: string) {
  return value.trim() === "" ? "未设置" : value.trim();
}

/**
 * Renders the single-note editor used by the notes dashboard modal. The
 * editor writes one checklist block back into the shared markdown source file
 * instead of exposing the whole file body.
 *
 * @param props Current note draft, editor actions, and sync state.
 * @returns The source-note editor layout.
 */
export function SourceNoteStudio({
  availabilityMessage,
  draft,
  editingItem,
  fileLabel,
  isCreating,
  isDirty,
  isInspecting,
  isLoading,
  isSaving,
  onChange,
  onClose,
  onCreate,
  onInspect,
  onReload,
  onSave,
  sourceRoots,
  syncMessage,
}: SourceNoteStudioProps) {
  const canEdit = availabilityMessage === null && sourceRoots.length > 0;
  const hasMeaningfulContent = draft.title.trim() !== "" || draft.noteText.trim() !== "";
  const saveDisabled = !canEdit || isSaving || !hasMeaningfulContent || (!isDirty && !isCreating);
  const editorTitle = isCreating ? "新建便签" : draft.title.trim() || editingItem?.item.title || "编辑当前便签";
  const editorMeta = isCreating
    ? (fileLabel ? `保存时会追加到 ${fileLabel}` : "保存时会追加到主 markdown 便签文件")
    : (fileLabel ? `当前正在编辑 ${fileLabel} 里的这张便签` : "当前正在编辑这张便签的 markdown 块");

  return (
    <section className="note-source-studio">
      <div className="note-source-studio__header">
        <div className="note-source-studio__heading">
          <p className="note-preview-page__eyebrow">Markdown Notes</p>
          <div className="note-source-studio__title-row">
            <NotebookText className="note-source-studio__title-icon" />
            <div>
              <h2>单张便签编辑</h2>
              <p>这里不再展开整份源文件，只编辑当前这张便签对应的 markdown 块。保存后会把元数据和正文一起写回主便签文件，并立即触发一次巡检。</p>
            </div>
          </div>
        </div>

        <div className="note-source-studio__actions">
          <Button className="note-source-studio__button" disabled={!canEdit || isSaving} onClick={onCreate} type="button" variant="ghost">
            <FilePlus2 className="h-4 w-4" />
            新建便签
          </Button>
          <Button className="note-source-studio__button" disabled={!canEdit || isInspecting || isSaving} onClick={onInspect} type="button" variant="ghost">
            <ScanSearch className="h-4 w-4" />
            {isInspecting ? "巡检中..." : "立即巡检"}
          </Button>
          <Button className="note-source-studio__button" disabled={!canEdit || isLoading || isSaving} onClick={onReload} type="button" variant="ghost">
            <RefreshCcw className="h-4 w-4" />
            刷新主文件
          </Button>
          <Button className="note-source-studio__button note-source-studio__button--primary" disabled={saveDisabled} onClick={onSave} type="button">
            <Save className="h-4 w-4" />
            {isSaving ? "保存中..." : "保存便签"}
          </Button>
          {onClose ? (
            <Button className="note-source-studio__button" onClick={onClose} type="button" variant="ghost">
              <X className="h-4 w-4" />
              关闭
            </Button>
          ) : null}
        </div>
      </div>

      <div className="note-source-studio__status-bar">
        <span className="note-source-studio__status-copy">
          {availabilityMessage ?? syncMessage ?? `已连接 ${sourceRoots.length} 个任务来源目录，当前只编辑单张便签。`}
        </span>
        {isDirty ? <span className="note-source-studio__dirty-pill">未保存</span> : null}
      </div>

      <div className="note-source-studio__body note-source-studio__body--single">
        <div className="note-source-studio__editor note-source-studio__editor--single">
          <div className="note-source-studio__editor-head">
            <div>
              <strong>{editorTitle}</strong>
              <p>{editorMeta}</p>
            </div>
          </div>

          <div className="note-source-studio__field-grid">
            <label className="note-source-studio__field">
              <span>标题</span>
              <input
                className="note-source-studio__input"
                disabled={!canEdit}
                onChange={(event) => onChange({ ...draft, title: event.target.value })}
                placeholder="可以留空，保存时会用正文第一行生成标题"
                type="text"
                value={draft.title}
              />
            </label>

            <label className="note-source-studio__field">
              <span>分组</span>
              <select
                className="note-source-studio__select"
                disabled={!canEdit}
                onChange={(event) => onChange({ ...draft, bucket: event.target.value as SourceNoteEditorDraft["bucket"] })}
                value={draft.bucket}
              >
                <option value="later">后续</option>
                <option value="upcoming">近期</option>
                <option value="recurring_rule">重复</option>
                <option value="closed">已结束</option>
              </select>
            </label>

            <label className="note-source-studio__field">
              <span>计划时间</span>
              <input
                className="note-source-studio__input"
                disabled={!canEdit}
                onChange={(event) => onChange({ ...draft, dueAt: event.target.value })}
                placeholder="例如 2026-04-25 18:30"
                type="text"
                value={draft.dueAt}
              />
            </label>

            <label className="note-source-studio__field">
              <span>下次发生</span>
              <input
                className="note-source-studio__input"
                disabled={!canEdit}
                onChange={(event) => onChange({ ...draft, nextOccurrenceAt: event.target.value })}
                placeholder="重复便签可填写下一次触发时间"
                type="text"
                value={draft.nextOccurrenceAt}
              />
            </label>

            <label className="note-source-studio__field">
              <span>重复规则</span>
              <input
                className="note-source-studio__input"
                disabled={!canEdit}
                onChange={(event) => onChange({ ...draft, repeatRule: event.target.value })}
                placeholder="例如 每周一 09:00"
                type="text"
                value={draft.repeatRule}
              />
            </label>

            <label className="note-source-studio__field">
              <span>前置条件</span>
              <input
                className="note-source-studio__input"
                disabled={!canEdit}
                onChange={(event) => onChange({ ...draft, prerequisite: event.target.value })}
                placeholder="例如 等待对方确认 / 准备材料"
                type="text"
                value={draft.prerequisite}
              />
            </label>

            <label className="note-source-studio__field note-source-studio__field--wide">
              <span>Agent 建议</span>
              <input
                className="note-source-studio__input"
                disabled={!canEdit}
                onChange={(event) => onChange({ ...draft, agentSuggestion: event.target.value })}
                placeholder="这一条建议会写回 markdown 元数据"
                type="text"
                value={draft.agentSuggestion}
              />
            </label>
          </div>

          <div className="note-source-studio__meta-strip">
            <div className="note-source-studio__meta-pill">
              <span>写入分组</span>
              <strong>{bucketLabels[draft.repeatRule.trim() !== "" ? "recurring_rule" : draft.bucket]}</strong>
            </div>
            <div className="note-source-studio__meta-pill">
              <span>创建时间</span>
              <strong>{formatMetadataValue(draft.createdAt)}</strong>
            </div>
            <div className="note-source-studio__meta-pill">
              <span>最近更新</span>
              <strong>{formatMetadataValue(draft.updatedAt)}</strong>
            </div>
          </div>

          <label className="note-source-studio__field note-source-studio__field--stacked">
            <span>便签正文</span>
            <Textarea
              className="note-source-studio__textarea note-source-studio__textarea--single"
              disabled={!canEdit}
              onChange={(event) => onChange({ ...draft, noteText: event.target.value })}
              placeholder="直接写内容也可以。标题留空时，保存会自动把第一行变成 - [ ] 标题。"
              value={draft.noteText}
            />
          </label>
        </div>
      </div>
    </section>
  );
}
