import type { NoteListItem, SourceNoteDocument, SourceNoteEditorBlock, SourceNoteEditorDraft, SourceNoteMetadataEntry } from "./notePage.types";

const SOURCE_NOTE_CHECKLIST_PATTERN = /^[-*]\s+\[( |x|X)\]\s+(.+)$/;
const SOURCE_NOTE_RESERVED_METADATA_KEYS = new Set([
  "agent",
  "bucket",
  "created_at",
  "due",
  "ended_at",
  "next",
  "note",
  "prerequisite",
  "recent_instance_status",
  "reminder",
  "repeat",
  "resource",
  "scope",
  "status",
  "suggest",
  "tags",
  "updated_at",
]);

function normalizeLineEndings(value: string) {
  return value.replace(/\r\n/g, "\n");
}

function trimTrailingEmptyLines(lines: string[]) {
  const trimmedLines = [...lines];
  while (trimmedLines.length > 0 && trimmedLines[trimmedLines.length - 1]?.trim() === "") {
    trimmedLines.pop();
  }
  return trimmedLines;
}

function stripLeadingStructuralBlankLine(lines: string[]) {
  if (lines[0]?.trim() === "") {
    return lines.slice(1);
  }
  return lines;
}

function parseChecklistLine(line: string) {
  const match = SOURCE_NOTE_CHECKLIST_PATTERN.exec(line.trim());
  if (!match) {
    return null;
  }

  return {
    checked: match[1].toLowerCase() === "x",
    title: match[2].trim(),
  };
}

function splitMetadataLine(line: string) {
  const separatorIndex = line.indexOf(":");
  if (separatorIndex <= 0 || separatorIndex >= line.length - 1) {
    return null;
  }

  const key = line.slice(0, separatorIndex).trim().toLowerCase();
  const value = line.slice(separatorIndex + 1).trim();
  if (key === "" || value === "") {
    return null;
  }

  return { key, value };
}

function formatTimestampForEditor(value: string | null | undefined) {
  if (!value) {
    return "";
  }

  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return value.trim();
  }

  const year = parsed.getFullYear();
  const month = `${parsed.getMonth() + 1}`.padStart(2, "0");
  const day = `${parsed.getDate()}`.padStart(2, "0");
  const hour = `${parsed.getHours()}`.padStart(2, "0");
  const minute = `${parsed.getMinutes()}`.padStart(2, "0");
  return `${year}-${month}-${day} ${hour}:${minute}`;
}

function formatTimestampForScheduleInput(parsed: Date) {
  const year = parsed.getFullYear();
  const month = `${parsed.getMonth() + 1}`.padStart(2, "0");
  const day = `${parsed.getDate()}`.padStart(2, "0");
  const hour = `${parsed.getHours()}`.padStart(2, "0");
  const minute = `${parsed.getMinutes()}`.padStart(2, "0");
  return `${year}-${month}-${day}T${hour}:${minute}`;
}

function readTodoSourcePath(item: NoteListItem["item"]) {
  const sourcePath = (item as NoteListItem["item"] & { source_path?: unknown }).source_path;
  return typeof sourcePath === "string" && sourcePath.trim() !== "" ? sourcePath : null;
}

function readTodoSourceLine(item: NoteListItem["item"]) {
  const sourceLine = (item as NoteListItem["item"] & { source_line?: unknown }).source_line;
  return typeof sourceLine === "number" && Number.isFinite(sourceLine) && sourceLine > 0 ? sourceLine : null;
}

function readTodoCreatedAt(item: NoteListItem["item"]) {
  const createdAt = (item as NoteListItem["item"] & { created_at?: unknown }).created_at;
  return typeof createdAt === "string" && createdAt.trim() !== "" ? createdAt : null;
}

function readTodoUpdatedAt(item: NoteListItem["item"]) {
  const updatedAt = (item as NoteListItem["item"] & { updated_at?: unknown }).updated_at;
  return typeof updatedAt === "string" && updatedAt.trim() !== "" ? updatedAt : null;
}

function normalizeMetadataEntries(entries: SourceNoteMetadataEntry[]) {
  return entries
    .map((entry) => ({
      key: entry.key.trim().toLowerCase(),
      value: entry.value.trim(),
    }))
    .filter((entry) => entry.key !== "" && entry.value !== "");
}

function deriveDraftTitleAndBody(draft: SourceNoteEditorDraft) {
  const explicitTitle = draft.title.trim();
  const normalizedNoteText = normalizeLineEndings(draft.noteText);
  if (explicitTitle !== "") {
    return {
      checked: draft.checked,
      noteText: trimTrailingEmptyLines(normalizedNoteText.split("\n")).join("\n"),
      title: explicitTitle,
    };
  }

  const bodyLines = normalizedNoteText.split("\n");
  const firstContentLineIndex = bodyLines.findIndex((line) => line.trim() !== "");
  if (firstContentLineIndex === -1) {
    return {
      checked: draft.checked,
      noteText: "",
      title: "New note",
    };
  }

  const firstContentLine = bodyLines[firstContentLineIndex].trim();
  const checklist = parseChecklistLine(firstContentLine);
  const remainingLines = trimTrailingEmptyLines(bodyLines.slice(firstContentLineIndex + 1)).join("\n");

  return {
    checked: draft.checked,
    noteText: remainingLines,
    title: (checklist?.title ?? firstContentLine).trim() || "New note",
  };
}

/**
 * Formats the structured note draft back into the single content field shown
 * to users. The first line stays reserved for the derived title and the rest
 * of the lines remain the editable body content.
 *
 * @param draft Current source-note editor draft.
 * @returns Single textarea content shown in the notes editor.
 */
export function formatSourceNoteEditorContent(draft: SourceNoteEditorDraft) {
  const title = draft.title.trim();
  const noteText = normalizeLineEndings(draft.noteText);
  if (title === "") {
    return noteText;
  }

  return noteText === "" ? title : `${title}\n${noteText}`;
}

/**
 * Formats hidden schedule metadata into the `datetime-local` value used by the
 * lightweight schedule dialog.
 *
 * @param value Existing note schedule metadata from markdown or protocol state.
 * @returns Local `datetime-local` input value, or an empty string when unknown.
 */
export function formatSourceNoteScheduleInputValue(value: string | null | undefined) {
  if (!value) {
    return "";
  }

  const normalizedValue = value.trim();
  if (normalizedValue === "") {
    return "";
  }

  const parsed = new Date(normalizedValue);
  if (!Number.isNaN(parsed.getTime())) {
    return formatTimestampForScheduleInput(parsed);
  }

  const normalizedLocalValue = normalizedValue.replace(" ", "T");
  const localParsed = new Date(normalizedLocalValue);
  if (!Number.isNaN(localParsed.getTime())) {
    return formatTimestampForScheduleInput(localParsed);
  }

  return "";
}

/**
 * Serializes a `datetime-local` input value back into the hidden markdown time
 * metadata. The editor stores RFC3339 so later inspections preserve the user's
 * local wall-clock selection without timezone drift.
 *
 * @param value Current `datetime-local` input value.
 * @returns RFC3339 timestamp for markdown metadata, or an empty string.
 */
export function serializeSourceNoteScheduleInputValue(value: string) {
  const normalizedValue = value.trim();
  if (normalizedValue === "") {
    return "";
  }

  const parsed = new Date(normalizedValue);
  if (Number.isNaN(parsed.getTime())) {
    return normalizedValue;
  }

  return parsed.toISOString();
}

/**
 * Resolves the persisted note bucket that should accompany hidden schedule
 * metadata. Timed notes become upcoming, recurring notes become recurring
 * rules, and notes without schedule metadata fall back to later.
 *
 * @param schedule Hidden schedule fields about to be written back.
 * @returns Bucket value that keeps later inspections aligned with the dialog.
 */
export function resolveSourceNoteDraftBucketForSchedule(schedule: Pick<SourceNoteEditorDraft, "dueAt" | "repeatRule">): SourceNoteEditorDraft["bucket"] {
  if (schedule.repeatRule.trim() !== "") {
    return "recurring_rule";
  }

  if (schedule.dueAt.trim() !== "") {
    return "upcoming";
  }

  return "later";
}

/**
 * Applies the single content field back onto the structured note draft while
 * keeping the hidden metadata untouched. Users edit only free-form content;
 * the editor still derives the title from the first line, but hidden checklist
 * completion remains owned by the existing draft state.
 *
 * @param draft Current source-note editor draft.
 * @param content Updated textarea content from the editor.
 * @returns Next draft with derived title and body content.
 */
export function updateSourceNoteEditorDraftContent(draft: SourceNoteEditorDraft, content: string): SourceNoteEditorDraft {
  if (content.trim() === "") {
    return {
      ...draft,
      checked: draft.checked,
      noteText: "",
      title: "",
    };
  }

  const derivedContent = deriveDraftTitleAndBody({
    ...draft,
    noteText: content,
    title: "",
  });

  return {
    ...draft,
    checked: derivedContent.checked,
    noteText: derivedContent.noteText,
    title: derivedContent.title,
  };
}

function createDraftSignaturePayload(draft: SourceNoteEditorDraft) {
  return {
    agentSuggestion: draft.agentSuggestion.trim(),
    bucket: draft.bucket,
    checked: draft.checked,
    createdAt: draft.createdAt.trim(),
    dueAt: draft.dueAt.trim(),
    effectiveScope: draft.effectiveScope.trim(),
    endedAt: draft.endedAt.trim(),
    extraMetadata: normalizeMetadataEntries(draft.extraMetadata),
    nextOccurrenceAt: draft.nextOccurrenceAt.trim(),
    noteText: normalizeLineEndings(draft.noteText),
    prerequisite: draft.prerequisite.trim(),
    recentInstanceStatus: draft.recentInstanceStatus.trim(),
    repeatRule: draft.repeatRule.trim(),
    sourceLine: draft.sourceLine ?? null,
    sourcePath: draft.sourcePath?.trim() ?? null,
    title: draft.title.trim(),
    updatedAt: draft.updatedAt.trim(),
  };
}

function toIsoTimestamp(value: Date) {
  return value.toISOString();
}

function buildDraftFromParsedBlock(block: SourceNoteEditorBlock, fallbackItem?: NoteListItem | null): SourceNoteEditorDraft {
  const fallbackDraft = fallbackItem ? buildSourceNoteEditorDraftFromItem(fallbackItem, block.sourcePath) : null;

  return {
    agentSuggestion: block.agentSuggestion || fallbackDraft?.agentSuggestion || "",
    bucket: block.bucket || fallbackDraft?.bucket || "later",
    checked: block.checked,
    createdAt: block.createdAt || fallbackDraft?.createdAt || "",
    dueAt: block.dueAt || fallbackDraft?.dueAt || "",
    effectiveScope: block.effectiveScope || fallbackDraft?.effectiveScope || "",
    endedAt: block.endedAt || fallbackDraft?.endedAt || "",
    extraMetadata: [...block.extraMetadata],
    nextOccurrenceAt: block.nextOccurrenceAt || fallbackDraft?.nextOccurrenceAt || "",
    noteText: block.noteText,
    prerequisite: block.prerequisite || fallbackDraft?.prerequisite || "",
    recentInstanceStatus: block.recentInstanceStatus || fallbackDraft?.recentInstanceStatus || "",
    repeatRule: block.repeatRule || fallbackDraft?.repeatRule || "",
    sourceLine: block.sourceLine,
    sourcePath: block.sourcePath,
    title: block.title || fallbackDraft?.title || "",
    updatedAt: block.updatedAt || fallbackDraft?.updatedAt || "",
  };
}

/**
 * Creates a blank note-editor draft for appending one more block into the
 * shared markdown source file.
 *
 * @param sourcePath Current primary markdown note path when available.
 * @returns Empty draft state for the note editor.
 */
export function createEmptySourceNoteEditorDraft(sourcePath: string | null = null): SourceNoteEditorDraft {
  return {
    agentSuggestion: "",
    bucket: "later",
    checked: false,
    createdAt: "",
    dueAt: "",
    effectiveScope: "",
    endedAt: "",
    extraMetadata: [],
    nextOccurrenceAt: "",
    noteText: "",
    prerequisite: "",
    recentInstanceStatus: "",
    repeatRule: "",
    sourceLine: null,
    sourcePath,
    title: "",
    updatedAt: "",
  };
}

/**
 * Builds one editor draft from an existing note item so the source-note modal
 * can prefill metadata from the detail view even before the markdown block is
 * reparsed locally.
 *
 * @param item Note item selected from the notes page.
 * @param sourcePath Fallback source path when the item does not carry one.
 * @returns Editor draft seeded from the current note data.
 */
export function buildSourceNoteEditorDraftFromItem(
  item: NoteListItem,
  sourcePath: string | null = null,
): SourceNoteEditorDraft {
  return {
    agentSuggestion: item.item.agent_suggestion?.trim() ?? "",
    bucket: item.item.bucket,
    checked: item.item.status === "completed",
    createdAt: formatTimestampForEditor(readTodoCreatedAt(item.item)),
    dueAt: formatTimestampForEditor(item.item.due_at),
    effectiveScope: item.item.effective_scope?.trim() ?? "",
    endedAt: formatTimestampForEditor(item.experience.endedAt),
    extraMetadata: [],
    nextOccurrenceAt: formatTimestampForEditor(item.item.next_occurrence_at),
    noteText: item.item.note_text?.trim() ?? "",
    prerequisite: item.item.prerequisite?.trim() ?? "",
    recentInstanceStatus: item.item.recent_instance_status?.trim() ?? "",
    repeatRule: item.item.repeat_rule?.trim() ?? "",
    sourceLine: item.sourceNote?.sourceLine ?? readTodoSourceLine(item.item),
    sourcePath: item.sourceNote?.path ?? readTodoSourcePath(item.item) ?? sourcePath,
    title: item.item.title,
    updatedAt: formatTimestampForEditor(readTodoUpdatedAt(item.item)),
  };
}

/**
 * Parses the shared markdown source file into editor-friendly note blocks.
 *
 * @param note Shared markdown source document.
 * @returns Parsed note blocks with metadata and line ranges.
 */
export function parseSourceNoteEditorBlocks(note: SourceNoteDocument): SourceNoteEditorBlock[] {
  const normalizedLines = normalizeLineEndings(note.content).split("\n");
  const blocks: SourceNoteEditorBlock[] = [];
  let current:
    | (SourceNoteEditorDraft & {
        bodyLines: string[];
        bodyStarted: boolean;
        endLine: number;
        noteMetadataText: string;
      })
    | null = null;

  const flushCurrent = () => {
    if (!current) {
      return;
    }

    const bodyText = trimTrailingEmptyLines(stripLeadingStructuralBlankLine(current.bodyLines)).join("\n");
    const noteText = current.noteMetadataText !== ""
      ? [current.noteMetadataText, bodyText].filter(Boolean).join("\n\n")
      : bodyText;
    blocks.push({
      agentSuggestion: current.agentSuggestion,
      bucket: current.bucket,
      checked: current.checked,
      createdAt: current.createdAt,
      dueAt: current.dueAt,
      effectiveScope: current.effectiveScope,
      endLine: current.endLine,
      endedAt: current.endedAt,
      extraMetadata: current.extraMetadata,
      nextOccurrenceAt: current.nextOccurrenceAt,
      noteText,
      prerequisite: current.prerequisite,
      recentInstanceStatus: current.recentInstanceStatus,
      repeatRule: current.repeatRule,
      sourceLine: current.sourceLine,
      sourcePath: current.sourcePath,
      title: current.title,
      updatedAt: current.updatedAt,
    });
    current = null;
  };

  normalizedLines.forEach((line, index) => {
    const checklist = parseChecklistLine(line);
    if (checklist) {
      flushCurrent();
      current = {
        ...createEmptySourceNoteEditorDraft(note.path),
        bodyLines: [],
        bodyStarted: false,
        checked: checklist.checked,
        endLine: index + 1,
        noteMetadataText: "",
        sourceLine: index + 1,
        title: checklist.title,
      };
      return;
    }

    if (!current) {
      return;
    }

    current.endLine = index + 1;
    if (line.trim() === "") {
      current.bodyStarted = true;
      current.bodyLines.push("");
      return;
    }

    // Hidden metadata only belongs to the header segment above the body.
    // After the body starts, reserved "key: value" prefixes must stay as literal note content.
    if (current.bodyStarted) {
      current.bodyLines.push(line.trimEnd());
      return;
    }

    const metadata = splitMetadataLine(line.trim());
    if (!metadata) {
      current.bodyStarted = true;
      current.bodyLines.push(line.trimEnd());
      return;
    }

    switch (metadata.key) {
      case "agent":
      case "suggest":
        current.agentSuggestion = metadata.value;
        return;
      case "bucket":
        if (metadata.value === "upcoming" || metadata.value === "later" || metadata.value === "recurring_rule" || metadata.value === "closed") {
          current.bucket = metadata.value;
        }
        return;
      case "created_at":
        current.createdAt = metadata.value;
        return;
      case "due":
        current.dueAt = metadata.value;
        return;
      case "ended_at":
        current.endedAt = metadata.value;
        return;
      case "next":
        current.nextOccurrenceAt = metadata.value;
        return;
      case "note":
        current.noteMetadataText = metadata.value;
        return;
      case "prerequisite":
        current.prerequisite = metadata.value;
        return;
      case "repeat":
        current.repeatRule = metadata.value;
        current.bucket = "recurring_rule";
        return;
      case "scope":
        current.effectiveScope = metadata.value;
        return;
      case "status":
        current.recentInstanceStatus = metadata.value;
        return;
      case "updated_at":
        current.updatedAt = metadata.value;
        return;
      case "resource":
      case "reminder":
      case "tags":
        current.extraMetadata.push(metadata);
        return;
      default:
        // Keep any remaining header "key: value" lines hidden so the editor
        // always round-trips metadata without showing it as editable content.
        current.extraMetadata.push(metadata);
        return;
    }
  });

  flushCurrent();
  return blocks;
}

/**
 * Finds the markdown block that corresponds to the selected note item.
 *
 * @param note Shared markdown source document.
 * @param item Note item selected in the dashboard.
 * @returns Matching markdown block when it can be resolved locally.
 */
export function findSourceNoteEditorBlock(note: SourceNoteDocument, item: NoteListItem): SourceNoteEditorBlock | null {
  const blocks = parseSourceNoteEditorBlocks(note);
  const sourceLine = item.sourceNote?.sourceLine ?? readTodoSourceLine(item.item);
  if (typeof sourceLine === "number" && sourceLine > 0) {
    const matchedByLine = blocks.find((block) => block.sourceLine === sourceLine);
    if (matchedByLine) {
      return matchedByLine;
    }
  }

  const normalizedTitle = item.item.title.trim().toLowerCase();
  const candidates = blocks.filter((block) => block.title.trim().toLowerCase() === normalizedTitle);
  return candidates.length === 1 ? candidates[0] : candidates[0] ?? null;
}

/**
 * Builds one editor draft either from the matching markdown block or, when the
 * block cannot be resolved, from the current note detail data.
 *
 * @param note Shared markdown source document.
 * @param item Note item selected in the dashboard.
 * @returns Editor draft aligned to the current note.
 */
export function buildSourceNoteEditorDraftFromNote(
  note: SourceNoteDocument,
  item: NoteListItem,
): SourceNoteEditorDraft {
  const matchedBlock = findSourceNoteEditorBlock(note, item);
  if (matchedBlock) {
    return buildDraftFromParsedBlock(matchedBlock, item);
  }

  return buildSourceNoteEditorDraftFromItem(item, note.path);
}

/**
 * Produces a stable signature for editor dirty checks without depending on the
 * raw markdown layout of the whole source file.
 *
 * @param draft Current editor draft.
 * @returns Comparable signature for dirty-state checks.
 */
export function createSourceNoteEditorDraftSignature(draft: SourceNoteEditorDraft) {
  return JSON.stringify(createDraftSignaturePayload(draft));
}

/**
 * Serializes one editor draft into the backend-compatible checklist block
 * format stored inside the shared markdown file.
 *
 * @param draft Current editor draft.
 * @param now Timestamp used for editor-side created and updated defaults.
 * @returns Serialized markdown block plus the normalized draft payload.
 */
export function serializeSourceNoteEditorDraft(draft: SourceNoteEditorDraft, now = new Date()) {
  const normalizedNow = toIsoTimestamp(now);
  const derivedContent = deriveDraftTitleAndBody(draft);
  const normalizedDraft = createDraftSignaturePayload({
    ...draft,
    bucket: draft.repeatRule.trim() !== "" ? "recurring_rule" : draft.bucket,
    checked: derivedContent.checked,
    createdAt: draft.createdAt.trim() || normalizedNow,
    noteText: derivedContent.noteText,
    title: derivedContent.title,
    updatedAt: normalizedNow,
  } satisfies SourceNoteEditorDraft);
  const bodyText = normalizedDraft.noteText;
  const inlineNoteText = bodyText !== "" && !bodyText.includes("\n") ? bodyText : null;
  const metadataLines = [
    `bucket: ${normalizedDraft.bucket}`,
    normalizedDraft.createdAt ? `created_at: ${normalizedDraft.createdAt}` : null,
    normalizedDraft.dueAt ? `due: ${normalizedDraft.dueAt}` : null,
    normalizedDraft.nextOccurrenceAt ? `next: ${normalizedDraft.nextOccurrenceAt}` : null,
    normalizedDraft.repeatRule ? `repeat: ${normalizedDraft.repeatRule}` : null,
    normalizedDraft.prerequisite ? `prerequisite: ${normalizedDraft.prerequisite}` : null,
    normalizedDraft.agentSuggestion ? `agent: ${normalizedDraft.agentSuggestion}` : null,
    normalizedDraft.effectiveScope ? `scope: ${normalizedDraft.effectiveScope}` : null,
    normalizedDraft.recentInstanceStatus ? `status: ${normalizedDraft.recentInstanceStatus}` : null,
    inlineNoteText ? `note: ${inlineNoteText}` : null,
    normalizedDraft.endedAt ? `ended_at: ${normalizedDraft.endedAt}` : null,
    normalizedDraft.updatedAt ? `updated_at: ${normalizedDraft.updatedAt}` : null,
    ...normalizedDraft.extraMetadata
      .filter((entry) => !SOURCE_NOTE_RESERVED_METADATA_KEYS.has(entry.key))
      .map((entry) => `${entry.key}: ${entry.value}`),
  ].filter((line): line is string => Boolean(line));
  const bodyLines = inlineNoteText || bodyText === "" ? [] : ["", ...bodyText.split("\n")];
  const checklistMarker = normalizedDraft.checked ? "[x]" : "[ ]";
  const blockLines = [
    `- ${checklistMarker} ${normalizedDraft.title}`,
    ...metadataLines,
    ...bodyLines,
  ];

  return {
    blockContent: blockLines.join("\n"),
    normalizedDraft: {
      ...draft,
      bucket: normalizedDraft.bucket,
      createdAt: normalizedDraft.createdAt,
      dueAt: normalizedDraft.dueAt,
      extraMetadata: normalizedDraft.extraMetadata,
      nextOccurrenceAt: normalizedDraft.nextOccurrenceAt,
      noteText: normalizedDraft.noteText,
      prerequisite: normalizedDraft.prerequisite,
      repeatRule: normalizedDraft.repeatRule,
      title: normalizedDraft.title,
      updatedAt: normalizedDraft.updatedAt,
    } satisfies SourceNoteEditorDraft,
  };
}

/**
 * Replaces the currently edited markdown block inside the shared source file.
 * When the block cannot be resolved anymore, the new block is appended instead.
 *
 * @param note Shared markdown source document.
 * @param draft Current editor draft.
 * @returns Updated file content plus the resolved source line of the block.
 */
export function upsertSourceNoteEditorBlock(note: SourceNoteDocument, draft: SourceNoteEditorDraft) {
  const { blockContent, normalizedDraft } = serializeSourceNoteEditorDraft(draft);
  const normalizedContent = normalizeLineEndings(note.content);
  const lines = normalizedContent.split("\n");
  const blocks = parseSourceNoteEditorBlocks(note);
  let matchedBlock: SourceNoteEditorBlock | null = null;

  if (typeof draft.sourceLine === "number" && draft.sourceLine > 0) {
    matchedBlock = blocks.find((block) => block.sourceLine === draft.sourceLine) ?? null;
  }

  if (!matchedBlock && normalizedDraft.title !== "") {
    const candidates = blocks.filter((block) => block.title.trim().toLowerCase() === normalizedDraft.title.trim().toLowerCase());
    matchedBlock = candidates.length === 1 ? candidates[0] : candidates[0] ?? null;
  }

  if (!matchedBlock || matchedBlock.sourceLine === null) {
    const trimmed = normalizedContent.trimEnd();
    const lineCount = trimmed === "" ? 0 : trimmed.split("\n").length;
    return {
      content: trimmed === "" ? `${blockContent}\n` : `${trimmed}\n\n${blockContent}\n`,
      sourceLine: lineCount === 0 ? 1 : lineCount + 2,
    };
  }

  const nextLines = [
    ...lines.slice(0, matchedBlock.sourceLine - 1),
    ...blockContent.split("\n"),
    ...lines.slice(matchedBlock.endLine),
  ];

  return {
    content: `${nextLines.join("\n").trimEnd()}\n`,
    sourceLine: matchedBlock.sourceLine,
  };
}
