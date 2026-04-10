export function NoteEmptyState() {
  return (
    <div className="note-empty-state">
      <p className="note-empty-state__eyebrow">暂无便签</p>
      <h2 className="note-empty-state__title">这里还没有可协作的事项</h2>
      <p className="note-empty-state__copy">等你把想记住的事情交给便签协作后，这里会按近期要做、后续安排、重复事项和已结束四组方式整理出来。</p>
    </div>
  );
}
