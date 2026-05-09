import { FilePlus2, NotebookText, RefreshCcw, Save, ScanSearch, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import type { NoteListItem, SourceNoteEditorDraft } from "../notePage.types";

type SourceNoteStudioProps = {
  availabilityMessage: string | null;
  draft: SourceNoteEditorDraft;
  editorContent: string;
  editingItem: NoteListItem | null;
  isCreating: boolean;
  isDirty: boolean;
  isInspecting: boolean;
  isLoading: boolean;
  isSaving: boolean;
  onChange: (content: string) => void;
  onClose?: () => void;
  onCreate: () => void;
  onInspect: () => void;
  onReload: () => void;
  onSave: () => void;
  sourceRoots: string[];
  syncMessage: string | null;
};

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
  editorContent,
  editingItem,
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
  const hasMeaningfulContent = editorContent.trim() !== "";
  const saveDisabled = !canEdit || isSaving || !hasMeaningfulContent || (!isDirty && !isCreating);
  const createDisabled = !canEdit || isSaving || (isCreating && !isDirty);
  const editorTitle = isCreating ? "新建便签" : draft.title.trim() || editingItem?.item.title || "编辑当前便签";
  const editorMeta = isCreating
    ? "点击“保存便签”后，系统会创建这张新便签。"
    : "这里始终只编辑便签内容，隐藏元数据继续由系统维护。";

  return (
    <section className="note-source-studio">
      <div className="note-source-studio__header">
        <div className="note-source-studio__heading">
          <p className="note-preview-page__eyebrow">Markdown Notes</p>
          <div className="note-source-studio__title-row">
            <NotebookText className="note-source-studio__title-icon" />
            <div>
              <h2>内容式便签编辑</h2>
              <p>这里只让用户写便签内容。第一行会作为标题，其余内容会作为正文；markdown 元数据继续由系统保留并在保存后写回主便签文件。</p>
            </div>
          </div>
        </div>

        <div className="note-source-studio__actions">
          <Button className="note-source-studio__button" disabled={createDisabled} onClick={onCreate} type="button" variant="ghost">
            <FilePlus2 className="h-4 w-4" />
            {isCreating ? "重新开始" : "开始新便签"}
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
          {availabilityMessage ?? syncMessage ?? `已连接 ${sourceRoots.length} 个任务来源目录，当前只编辑单张便签的内容。`}
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

          <label className="note-source-studio__field note-source-studio__field--stacked">
            <span>便签内容</span>
            <Textarea
              className="note-source-studio__textarea note-source-studio__textarea--single"
              disabled={!canEdit}
              onChange={(event) => onChange(event.target.value)}
              placeholder={"第一行会作为标题，其余内容会作为正文。\n\n例如：\n整理 PR365 的前端便签问题\n把创建入口改成只输入内容，其他元数据继续由系统维护。"}
              value={editorContent}
            />
          </label>
        </div>
      </div>
    </section>
  );
}
