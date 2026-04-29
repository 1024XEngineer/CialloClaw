import type { NoteListItem, SourceNoteDocument, SourceNoteEditorBlock, SourceNoteEditorDraft, SourceNoteMetadataEntry } from "./notePage.types";

const SOURCE_NOTE_CHECKLIST_PATTERN = /^(?: {0,3})[-*]\s+\[( |x|X)\]\s+(.+)$/;
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

function parseChecklistLine(line: string) {
  const candidate = line.trimEnd();
  if (/^\t/.test(candidate) || /^ {4,}/.test(candidate)) {
    return null;
  }

  const match = SOURCE_NOTE_CHECKLIST_PATTERN.exec(candidate);
  if (!match) {
    return null;
  }

  return {
    checked: match[1].toLowerCase() === "x",
    title: match[2].trim(),
  };
}

function isIndentedSourceNoteLine(line: string) {
  return /^\s/.test(line);
}

function normalizeNaturalNoteLine(line: string) {
  const trimmedRight = line.trimEnd();
  const trimmed = trimmedRight.trim();
  if (trimmed.startsWith("#")) {
    return trimmed.replace(/^#+\s*/, "").trim();
  }
  return trimmedRight;
}

function serializeChecklistBodyLine(line: string) {
  return line === "" ? "" : `  ${line}`;
}

function normalizeChecklistBodyLine(line: string) {
  const trimmedRight = line.trimEnd();
  if (trimmedRight.startsWith("  ")) {
    return trimmedRight.slice(2);
  }
  if (trimmedRight.startsWith("\t")) {
    return trimmedRight.slice(1);
  }
  return trimmedRight;
}

function isNaturalHeadingLine(line: string) {
  return line.trim().startsWith("#") && normalizeNaturalNoteLine(line) !== "";
}

function hasNaturalNoteContent(lines: string[]) {
  return lines.some((line) => line.trim() !== "");
}

// Blank lines inside note bodies are content; only outer blank boundaries are trimmed.
function trimBoundaryBlankLines(lines: string[]) {
  let start = 0;
  while (start < lines.length && lines[start]?.trim() === "") {
    start += 1;
  }

  let end = lines.length;
  while (end > start && lines[end - 1]?.trim() === "") {
    end -= 1;
  }

  return lines.slice(start, end);
}

function normalizeEditorNoteText(value: string) {
  return trimBoundaryBlankLines(normalizeLineEndings(value).split("\n")).join("\n");
}

function splitNaturalNoteContent(lines: string[]) {
  const normalized = trimBoundaryBlankLines(lines.map(normalizeNaturalNoteLine));
  if (normalized.length === 0) {
    return null;
  }

  return {
    noteText: trimBoundaryBlankLines(normalized.slice(1)).join("\n"),
    title: normalized[0]?.trim() ?? "",
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
      noteText: normalizeEditorNoteText(normalizedNoteText),
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
  const remainingLines = normalizeEditorNoteText(bodyLines.slice(firstContentLineIndex + 1).join("\n"));

  return {
    // Blank-title drafts derive their task state from the synthesized first
    // line only. Reusing a stale checked flag here would silently re-close a
    // note that the user already moved back to an active bucket.
    checked: checklist?.checked ?? false,
    noteText: remainingLines,
    title: (checklist?.title ?? firstContentLine).trim() || "New note",
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
    noteText: normalizeEditorNoteText(draft.noteText),
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

function getDefaultBucketForSourcePath(
  sourcePath: string | null,
  checked: boolean,
  recurring: boolean,
): SourceNoteEditorDraft["bucket"] {
  if (checked) {
    return "closed";
  }

  const normalized = sourcePath?.trim().toLowerCase() ?? "";
  if (recurring || normalized.includes("recurring") || normalized.includes("weekly") || normalized.includes("repeat")) {
    return "recurring_rule";
  }
  if (normalized.includes("later") || normalized.includes("backlog")) {
    return "later";
  }
  if (normalized.includes("inbox") || normalized.includes("upcoming") || normalized.includes("today") || normalized.includes("urgent")) {
    return "upcoming";
  }
  return "upcoming";
}

function normalizeSourceNoteBucket(
  bucket: SourceNoteEditorDraft["bucket"],
  checked: boolean,
  sourcePath: string | null,
  recurring: boolean,
): SourceNoteEditorDraft["bucket"] {
  if (checked) {
    return "closed";
  }
  if (bucket === "closed") {
    return "closed";
  }
  if (recurring || bucket === "recurring_rule") {
    return "recurring_rule";
  }
  if (bucket === "upcoming" || bucket === "later") {
    return bucket;
  }
  return getDefaultBucketForSourcePath(sourcePath, checked, recurring);
}

function normalizeSourceNoteCheckedState(
  bucket: SourceNoteEditorDraft["bucket"],
  checked: boolean,
) {
  return checked || bucket === "closed";
}

function buildDraftFromParsedBlock(block: SourceNoteEditorBlock, fallbackItem?: NoteListItem | null): SourceNoteEditorDraft {
  const fallbackDraft = fallbackItem ? buildSourceNoteEditorDraftFromItem(fallbackItem, block.sourcePath) : null;

  return {
    agentSuggestion: block.agentSuggestion || fallbackDraft?.agentSuggestion || "",
    bucket: normalizeSourceNoteBucket(
      block.bucketIsExplicit ? block.bucket : (fallbackDraft?.bucket ?? block.bucket),
      block.checked,
      block.sourcePath,
      block.repeatRule !== "",
    ),
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
    bucket: getDefaultBucketForSourcePath(sourcePath, false, false),
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
    // Closed remains a bucket-level concept in the editor. Keeping `checked`
    // tied to the formal completion state avoids stale local flags when a
    // closed source note is reopened into an active bucket.
    checked: item.item.status === "completed",
    createdAt: formatTimestampForEditor(readTodoCreatedAt(item.item)),
    dueAt: formatTimestampForEditor(item.item.due_at),
    effectiveScope: item.item.effective_scope?.trim() ?? "",
    endedAt: formatTimestampForEditor(item.experience.endedAt),
    extraMetadata: [],
    nextOccurrenceAt: formatTimestampForEditor(item.item.next_occurrence_at),
    noteText: normalizeEditorNoteText(item.item.note_text ?? ""),
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
  let naturalLines: string[] = [];
  let naturalStartLine: number | null = null;
  let naturalStartedWithHeading = false;
  let current:
    | (SourceNoteEditorDraft & {
        bodyLines: string[];
        bucketIsExplicit: boolean;
        endLine: number;
        noteMetadataText: string;
      })
    | null = null;
  let currentPendingBodySpacer = false;

  const flushCurrent = () => {
    if (!current) {
      return;
    }

    const bodyLines = trimBoundaryBlankLines(current.bodyLines);
    const noteTextParts = [
      current.noteMetadataText,
      bodyLines.length > 0 ? bodyLines.join("\n") : "",
    ].filter((part) => part !== "");
    blocks.push({
      agentSuggestion: current.agentSuggestion,
      bucket: current.bucket,
      bucketIsExplicit: current.bucketIsExplicit,
      checked: current.checked,
      createdAt: current.createdAt,
      dueAt: current.dueAt,
      effectiveScope: current.effectiveScope,
      endLine: current.endLine,
      endedAt: current.endedAt,
      extraMetadata: current.extraMetadata,
      nextOccurrenceAt: current.nextOccurrenceAt,
      noteText: noteTextParts.join("\n\n"),
      prerequisite: current.prerequisite,
      recentInstanceStatus: current.recentInstanceStatus,
      repeatRule: current.repeatRule,
      sourceLine: current.sourceLine,
      sourcePath: current.sourcePath,
      title: current.title,
      updatedAt: current.updatedAt,
    });
    current = null;
    currentPendingBodySpacer = false;
  };
  const flushNatural = (endLine: number) => {
    const naturalContent = splitNaturalNoteContent(naturalLines);
    if (naturalContent && naturalStartLine !== null) {
      blocks.push({
        ...createEmptySourceNoteEditorDraft(note.path),
        bucketIsExplicit: false,
        endLine,
        noteText: naturalContent.noteText,
        sourceLine: naturalStartLine,
        title: naturalContent.title,
      });
    }
    naturalLines = [];
    naturalStartLine = null;
    naturalStartedWithHeading = false;
  };

  normalizedLines.forEach((line, index) => {
    const checklist = current && isIndentedSourceNoteLine(line) ? null : parseChecklistLine(line);
    // Only top-level checklist rows start structured blocks; indented rows are
    // body content written by the editor for round-trip-safe natural notes.
    if (checklist) {
      flushNatural(index);
      flushCurrent();
      current = {
        ...createEmptySourceNoteEditorDraft(note.path),
        bodyLines: [],
        bucketIsExplicit: false,
        checked: checklist.checked,
        endLine: index + 1,
        noteMetadataText: "",
        sourceLine: index + 1,
        title: checklist.title,
      };
      currentPendingBodySpacer = false;
      return;
    }

    if (!current) {
      if (isNaturalHeadingLine(line) && hasNaturalNoteContent(naturalLines)) {
        flushNatural(index);
      }
      const naturalLine = normalizeNaturalNoteLine(line);
      if (naturalLine.trim() === "") {
        if (hasNaturalNoteContent(naturalLines)) {
          if (naturalStartedWithHeading) {
            naturalLines.push("");
          } else {
            // Plain-text natural notes use blank lines as item separators so
            // multiple paragraph notes do not collapse into one editor block.
            flushNatural(index);
          }
        }
        return;
      }
      if (naturalStartLine === null) {
        naturalStartLine = index + 1;
        naturalStartedWithHeading = isNaturalHeadingLine(line);
      }
      naturalLines.push(naturalLine);
      return;
    }

    current.endLine = index + 1;
    if (line.trim() === "") {
      if (current.bodyLines.length > 0) {
        current.bodyLines.push("");
      } else {
        // Spacer lines before unindented metadata are not body content; they
        // keep existing source notes with visual separation parseable.
        currentPendingBodySpacer = true;
      }
      return;
    }

    const metadata =
      current.bodyLines.length === 0 && (!currentPendingBodySpacer || !isIndentedSourceNoteLine(line))
        ? splitMetadataLine(line.trim())
        : null;
    if (!metadata) {
      currentPendingBodySpacer = false;
      current.bodyLines.push(normalizeChecklistBodyLine(line));
      return;
    }

    currentPendingBodySpacer = false;
    switch (metadata.key) {
      case "agent":
      case "suggest":
        current.agentSuggestion = metadata.value;
        return;
      case "bucket":
        if (metadata.value === "upcoming" || metadata.value === "later" || metadata.value === "recurring_rule" || metadata.value === "closed") {
          current.bucket = metadata.value;
          current.bucketIsExplicit = true;
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
        current.bucketIsExplicit = true;
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
        current.bodyLines.push(normalizeChecklistBodyLine(line));
    }
  });

  flushCurrent();
  flushNatural(normalizedLines.length);
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
  const normalizedChecked = normalizeSourceNoteCheckedState(
    draft.bucket,
    draft.title.trim() === "" ? derivedContent.checked : false,
  );
  const normalizedBucket = normalizeSourceNoteBucket(
    draft.bucket,
    normalizedChecked,
    draft.sourcePath,
    draft.repeatRule.trim() !== "",
  );
  const normalizedDraft = createDraftSignaturePayload({
    ...draft,
    bucket: normalizedBucket,
    checked: normalizedChecked,
    createdAt: draft.createdAt.trim() || normalizedNow,
    endedAt: normalizedChecked ? (draft.endedAt.trim() || normalizedNow) : "",
    noteText: derivedContent.noteText,
    title: derivedContent.title,
    updatedAt: normalizedNow,
  } satisfies SourceNoteEditorDraft);
  const sourceDefaultBucket = getDefaultBucketForSourcePath(normalizedDraft.sourcePath, normalizedDraft.checked, normalizedDraft.repeatRule !== "");
  const hasKnownSourcePath = normalizedDraft.sourcePath !== null && normalizedDraft.sourcePath.trim() !== "";
  const shouldPersistBucket = !hasKnownSourcePath || normalizedDraft.bucket !== sourceDefaultBucket || normalizedDraft.repeatRule !== "";
  const metadataLines = [
    normalizedDraft.createdAt ? `created_at: ${normalizedDraft.createdAt}` : null,
    normalizedDraft.updatedAt ? `updated_at: ${normalizedDraft.updatedAt}` : null,
    normalizedDraft.endedAt ? `ended_at: ${normalizedDraft.endedAt}` : null,
    shouldPersistBucket ? `bucket: ${normalizedDraft.bucket}` : null,
    normalizedDraft.dueAt ? `due: ${normalizedDraft.dueAt}` : null,
    normalizedDraft.nextOccurrenceAt ? `next: ${normalizedDraft.nextOccurrenceAt}` : null,
    normalizedDraft.repeatRule ? `repeat: ${normalizedDraft.repeatRule}` : null,
    normalizedDraft.prerequisite ? `prerequisite: ${normalizedDraft.prerequisite}` : null,
    normalizedDraft.agentSuggestion ? `agent: ${normalizedDraft.agentSuggestion}` : null,
    normalizedDraft.effectiveScope ? `scope: ${normalizedDraft.effectiveScope}` : null,
    normalizedDraft.recentInstanceStatus ? `status: ${normalizedDraft.recentInstanceStatus}` : null,
    ...normalizedDraft.extraMetadata
      .filter((entry) => !SOURCE_NOTE_RESERVED_METADATA_KEYS.has(entry.key))
      .map((entry) => `${entry.key}: ${entry.value}`),
  ].filter((line): line is string => Boolean(line));
  const bodyText = normalizedDraft.noteText;
  const bodyLines = bodyText === "" ? [] : ["", ...bodyText.split("\n").map(serializeChecklistBodyLine)];
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
      checked: normalizedDraft.checked,
      createdAt: normalizedDraft.createdAt,
      dueAt: normalizedDraft.dueAt,
      endedAt: normalizedDraft.endedAt,
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
