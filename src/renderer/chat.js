const api = window.pixelOrb;

const elements = {
  avatar: document.getElementById("chatAvatar"),
  statusLabel: document.getElementById("statusLabel"),
  pinButton: document.getElementById("pinButton"),
  historyButton: document.getElementById("historyButton"),
  closeButton: document.getElementById("closeButton"),
  chatBody: document.getElementById("chatBody"),
  historyPanel: document.getElementById("historyPanel"),
  taskStatus: document.getElementById("taskStatus"),
  taskStatusLabel: document.getElementById("taskStatusLabel"),
  taskStatusMeta: document.getElementById("taskStatusMeta"),
  taskProgressBar: document.getElementById("taskProgressBar"),
  cancelTaskButton: document.getElementById("cancelTaskButton"),
  meetingPromptCard: document.getElementById("meetingPromptCard"),
  meetingPromptStatus: document.getElementById("meetingPromptStatus"),
  meetingPromptMeta: document.getElementById("meetingPromptMeta"),
  meetingPromptAcceptButton: document.getElementById("meetingPromptAcceptButton"),
  meetingPromptDismissButton: document.getElementById("meetingPromptDismissButton"),
  meetingCancelButton: document.getElementById("meetingCancelButton"),
  meetingSummaryCard: document.getElementById("meetingSummaryCard"),
  meetingSummaryButton: document.getElementById("meetingSummaryButton"),
  meetingSummaryStatus: document.getElementById("meetingSummaryStatus"),
  meetingSummaryUpdated: document.getElementById("meetingSummaryUpdated"),
  meetingSummaryMeta: document.getElementById("meetingSummaryMeta"),
  meetingSummaryOverlayState: document.getElementById("meetingSummaryOverlayState"),
  meetingCaptionsOverlayState: document.getElementById("meetingCaptionsOverlayState"),
  showSummaryOverlayButton: document.getElementById("showSummaryOverlayButton"),
  showCaptionsOverlayButton: document.getElementById("showCaptionsOverlayButton"),
  showSummaryOverlayToolbarButton: document.getElementById("showSummaryOverlayToolbarButton"),
  showCaptionsOverlayToolbarButton: document.getElementById("showCaptionsOverlayToolbarButton"),
  messageList: document.getElementById("messageList"),
  quickChips: document.getElementById("quickChips"),
  selectionButton: document.getElementById("selectionButton"),
  clipboardButton: document.getElementById("clipboardButton"),
  composerInput: document.getElementById("composerInput"),
  sendButton: document.getElementById("sendButton")
};

const avatarContext = elements.avatar.getContext("2d");
avatarContext.imageSmoothingEnabled = false;

const DEFAULT_MEETING_INTERVAL_MS = 2 * 60 * 1000;
const DEFAULT_TRANSCRIPT_INTERVAL_MS = 10 * 1000;

function createMeetingSourceRuntime() {
  return {
    stream: null,
    recorder: null,
    captureContext: null,
    inputNode: null,
    processorNode: null,
    sinkNode: null,
    queue: Promise.resolve(),
    sequence: 0,
    pendingStopResolve: null,
    closing: false,
    mode: "chunk"
  };
}

const state = {
  snapshot: null,
  historyOpen: false,
  lastRenderedMessageCount: 0,
  lastHandledMeetingCaptureRequestId: "",
  meetingStopping: false,
  meetingTranscriptionMode: "chunk",
  meetingSummaryTimer: null,
  meetingSources: {
    system: createMeetingSourceRuntime(),
    microphone: createMeetingSourceRuntime()
  }
};

let sharedAudioContext;

function normalizeErrorMessage(error, fallback = "会议总结启动失败") {
  if (!error) {
    return fallback;
  }
  if (typeof error === "string") {
    return error;
  }
  if (typeof error.message === "string" && error.message.trim()) {
    return error.message.trim();
  }
  return fallback;
}

function getMeetingState() {
  return state.snapshot?.state?.meetingSummary || null;
}

function getMeetingDetectionState() {
  return state.snapshot?.state?.meetingDetection || null;
}

function getMeetingTranscriptionMode() {
  return getMeetingState()?.transcriptionMode || state.meetingTranscriptionMode || "chunk";
}

function getSourceLabel(source) {
  return source === "microphone" ? "麦克风" : "系统音频";
}

function getSourceRuntime(source) {
  return state.meetingSources[source];
}

function hasActiveMeetingRuntime() {
  return Object.values(state.meetingSources).some((runtime) => Boolean(
    runtime.recorder
    || runtime.stream
    || runtime.captureContext
    || runtime.processorNode
  ));
}

function clearMeetingSummaryTimer() {
  if (state.meetingSummaryTimer) {
    clearInterval(state.meetingSummaryTimer);
    state.meetingSummaryTimer = null;
  }
}

function startMeetingSummaryTimer(intervalMs) {
  clearMeetingSummaryTimer();
  state.meetingSummaryTimer = setInterval(() => {
    void api.refreshMeetingSummary({ force: false }).then(async (result) => {
      if (result?.ok) {
        return;
      }
      if (result?.handled) {
        await stopMeetingSummaryCapture({ notifyMain: false });
      }
    });
  }, intervalMs);
}

function applySnapshot(snapshot) {
  state.snapshot = snapshot;
  state.meetingTranscriptionMode = snapshot.state.meetingSummary?.transcriptionMode || state.meetingTranscriptionMode;
  document.title = `Pixel Chat - ${snapshot.state.statusLabel}`;
  syncMeetingCaptureWithSnapshot(snapshot);
  renderHeader();
  renderTaskStatus();
  renderMeetingPrompt();
  renderMeetingControls();
  renderMessages();
  renderQuickChips();
  renderHistory();
}

function renderHeader() {
  if (!state.snapshot) {
    return;
  }

  elements.statusLabel.textContent = state.snapshot.state.statusLabel;
  elements.pinButton.classList.toggle("active", Boolean(state.snapshot.state.chatPinned));
}

function renderTaskStatus() {
  if (!state.snapshot) {
    return;
  }

  const current = state.snapshot.state;
  const active = Boolean(current.currentTask);
  elements.taskStatus.classList.toggle("hidden", !active);
  if (!active) {
    return;
  }

  elements.taskStatusLabel.textContent = current.progressLabel || "正在执行";
  elements.taskStatusMeta.textContent = current.statusNote || current.statusLabel;
  elements.taskProgressBar.style.width = `${Math.round((current.progress || 0) * 100)}%`;
}

function renderMeetingPrompt() {
  if (!state.snapshot) {
    elements.meetingPromptCard.classList.add("hidden");
    return;
  }

  const detection = getMeetingDetectionState();
  const visible = Boolean(detection?.promptVisible);
  elements.meetingPromptCard.classList.toggle("hidden", !visible);
  if (!visible) {
    return;
  }

  elements.meetingPromptStatus.textContent = detection.statusLabel || "待确认";
  elements.meetingPromptMeta.textContent = detection.note
    || (detection.candidateApp
      ? `检测到你可能正在参加 ${detection.candidateApp}，确认后会自动开始会议总结。`
      : "检测到你可能正在开会，确认后会自动开始会议总结。");
}

function renderMeetingSummary() {
  if (!state.snapshot) {
    elements.meetingCancelButton.classList.add("hidden");
    return;
  }

  const meeting = getMeetingState();
  const shouldShow = Boolean(meeting?.enabled || meeting?.latestSummary || meeting?.error);
  const canCancel = Boolean(meeting?.enabled || state.meetingStopping || hasActiveMeetingRuntime());
  elements.meetingSummaryCard.classList.toggle("hidden", !shouldShow);
  elements.meetingSummaryButton.classList.toggle("active", Boolean(meeting?.enabled));
  elements.meetingSummaryButton.textContent = meeting?.enabled ? "停止总结" : "会议总结";
  elements.meetingCancelButton.classList.toggle("hidden", !canCancel);
  elements.meetingCancelButton.disabled = state.meetingStopping;
  elements.meetingCancelButton.textContent = state.meetingStopping ? "正在取消..." : "取消会议总结";

  if (!meeting) {
    return;
  }

  elements.meetingSummaryStatus.textContent = meeting.statusLabel || "未开始";
  elements.meetingSummaryUpdated.textContent = meeting.lastUpdatedAt
    ? `更新于 ${meeting.lastUpdatedAt}`
    : "等待第一次总结";
  elements.meetingSummaryMeta.textContent = meeting.error
    || meeting.note
    || "会议进行中会每 2 分钟生成一段当前概要。";
  elements.meetingSummaryContent.textContent = meeting.latestSummary
    || "会议总结启动后，这里会显示最近两分钟内容的一段话概要。";
}

function renderMeetingSource(source) {
  const sourceElements = getSourceElements(source);
  const sourceState = getMeetingState()?.sources?.[source];
  if (!sourceState) {
    sourceElements.status.textContent = "未连接";
    sourceElements.meta.textContent = "等待输入";
    sourceElements.latest.textContent = "暂无转写";
    sourceElements.log.replaceChildren();
    return;
  }

  sourceElements.status.textContent = sourceState.statusLabel || "未连接";
  sourceElements.meta.textContent = sourceState.error || sourceState.note || "等待输入";
  sourceElements.latest.textContent = sourceState.latestText || "暂无转写";
  sourceElements.log.replaceChildren();

  const items = sourceState.recentItems || [];
  if (!items.length) {
    const placeholder = document.createElement("div");
    placeholder.className = "meeting-debug-log-item";
    placeholder.innerHTML = "<time>暂无片段</time><span>这一路的实时转写会显示在这里。</span>";
    sourceElements.log.appendChild(placeholder);
    return;
  }

  items.forEach((item) => {
    const row = document.createElement("div");
    row.className = "meeting-debug-log-item";
    row.innerHTML = `<time>${item.time}</time><span>${item.text}</span>`;
    sourceElements.log.appendChild(row);
  });
}

function renderMeetingDebug() {
  const meeting = getMeetingState();
  const shouldShow = state.debugOpen && Boolean(
    meeting
    && (
      meeting.enabled
      || meeting.error
      || Object.values(meeting.sources || {}).some((sourceState) => (
        sourceState.latestText
        || sourceState.error
        || (sourceState.recentItems || []).length
      ))
    )
  );

  elements.meetingDebugCard.classList.toggle("hidden", !shouldShow);
  elements.meetingDebugButton.classList.toggle("active", state.debugOpen);
  elements.meetingDebugButton.textContent = state.debugOpen ? "隐藏调试" : "转写调试";
  elements.meetingDebugMeta.textContent = getMeetingTranscriptionMode() === "realtime"
    ? "系统音频与麦克风分开流式转写"
    : "系统音频与麦克风分开实时转写";

  if (!meeting) {
    return;
  }

  renderMeetingSource("system");
  renderMeetingSource("microphone");
}

function renderMeetingControls() {
  if (!state.snapshot) {
    elements.meetingCancelButton.classList.add("hidden");
    return;
  }

  const meeting = getMeetingState();
  const shouldShow = Boolean(
    meeting
    && (
      meeting.enabled
      || meeting.error
      || meeting.lastUpdatedAt
      || meeting.summaryOverlayVisible
      || meeting.captionsOverlayVisible
    )
  );
  const canCancel = Boolean(meeting?.enabled || state.meetingStopping || hasActiveMeetingRuntime());
  elements.meetingSummaryCard.classList.toggle("hidden", !shouldShow);
  elements.meetingSummaryButton.classList.toggle("active", Boolean(meeting?.enabled));
  elements.meetingSummaryButton.textContent = meeting?.enabled ? "停止总结" : "会议总结";
  elements.meetingCancelButton.classList.toggle("hidden", !canCancel);
  elements.meetingCancelButton.disabled = state.meetingStopping;
  elements.meetingCancelButton.textContent = state.meetingStopping ? "正在取消..." : "取消会议总结";

  if (!meeting) {
    return;
  }

  elements.meetingSummaryStatus.textContent = meeting.statusLabel || "未开始";
  elements.meetingSummaryUpdated.textContent = meeting.lastUpdatedAt
    ? `更新于 ${meeting.lastUpdatedAt}`
    : "等待第一轮概要";
  elements.meetingSummaryMeta.textContent = meeting.error
    || meeting.note
    || "会议进行中会每 2 分钟更新概要窗，并持续刷新实时字幕窗。";
  elements.meetingSummaryOverlayState.textContent = meeting.summaryOverlayVisible ? "已显示" : "已隐藏";
  elements.meetingCaptionsOverlayState.textContent = meeting.captionsOverlayVisible ? "已显示" : "已隐藏";
  elements.showSummaryOverlayButton.disabled = Boolean(meeting.summaryOverlayVisible);
  elements.showCaptionsOverlayButton.disabled = Boolean(meeting.captionsOverlayVisible);
  elements.showSummaryOverlayToolbarButton.disabled = Boolean(meeting.summaryOverlayVisible);
  elements.showCaptionsOverlayToolbarButton.disabled = Boolean(meeting.captionsOverlayVisible);
}

function renderMessages() {
  if (!state.snapshot) {
    return;
  }

  const previousMessageCount = state.lastRenderedMessageCount;
  const nextMessageCount = state.snapshot.state.chatHistory.length;
  const shouldStickToBottom = !elements.chatBody
    || previousMessageCount === 0
    || (elements.chatBody.scrollHeight - elements.chatBody.scrollTop - elements.chatBody.clientHeight) < 64;

  elements.messageList.replaceChildren();
  state.snapshot.state.chatHistory.forEach((message) => {
    const article = document.createElement("article");
    article.className = `message ${message.role}`;
    article.textContent = message.text;

    const meta = document.createElement("div");
    meta.className = "message-meta";
    meta.textContent = message.timestamp;
    article.appendChild(meta);
    elements.messageList.appendChild(article);
  });

  state.lastRenderedMessageCount = nextMessageCount;
  if (elements.chatBody && shouldStickToBottom) {
    elements.chatBody.scrollTop = elements.chatBody.scrollHeight;
  }
}

function renderQuickChips() {
  if (!state.snapshot) {
    return;
  }

  const chipActions = {
    "总结选中文本": () => api.runTask("summarize", "chat"),
    "解释这个错误": () => api.runTask("explain", "chat"),
    "帮我起草回复": () => api.sendChat("帮我起草一条简短回复"),
    "翻译为英文": () => api.runTask("translate", "chat")
  };

  elements.quickChips.replaceChildren();
  state.snapshot.state.quickChips.forEach((label) => {
    const chip = document.createElement("button");
    chip.type = "button";
    chip.className = "chip-button";
    chip.textContent = label;
    chip.addEventListener("click", () => {
      const action = chipActions[label];
      if (action) {
        action();
      } else {
        api.sendChat(label);
      }
    });
    elements.quickChips.appendChild(chip);
  });
}

function renderHistory() {
  if (!state.snapshot || !state.historyOpen) {
    elements.historyPanel.classList.add("hidden");
    return;
  }

  elements.historyPanel.replaceChildren();
  elements.historyPanel.classList.remove("hidden");
  if (!state.snapshot.state.recentActions.length) {
    const row = document.createElement("div");
    row.className = "history-item";
    row.innerHTML = "<strong>还没有最近动作</strong><span>你执行过的操作会显示在这里，方便回看。</span>";
    elements.historyPanel.appendChild(row);
    return;
  }

  state.snapshot.state.recentActions.forEach((item) => {
    const row = document.createElement("div");
    row.className = "history-item";
    row.innerHTML = `<strong>${item.title}</strong><span>${item.detail}</span><span>${item.time}</span>`;
    elements.historyPanel.appendChild(row);
  });
}

function autoresize() {
  elements.composerInput.style.height = "44px";
  elements.composerInput.style.height = `${Math.min(elements.composerInput.scrollHeight, 92)}px`;
}

function sendInput() {
  const value = elements.composerInput.value.trim();
  if (!value) {
    return;
  }

  api.sendChat(value);
  elements.composerInput.value = "";
  autoresize();
}

function drawCell(x, y, width, height, color) {
  avatarContext.fillStyle = color;
  avatarContext.fillRect(x * 2, y * 2, width * 2, height * 2);
}

function drawAvatar() {
  avatarContext.clearRect(0, 0, 48, 48);
  const current = state.snapshot?.state;
  if (!current) {
    return;
  }

  const accent = current.status === "thinking" || current.status === "needs-user" ? "#efbc77" : "#67d8cb";
  drawCell(8, 8, 8, 1, "#22313b");
  drawCell(7, 9, 10, 5, "#22313b");
  drawCell(8, 10, 8, 4, "#d8f1f5");
  drawCell(8, 9, 2, 2, accent);
  drawCell(14, 9, 2, 2, accent);
  drawCell(10, 11, 1, 2, "#13222c");
  drawCell(13, 11, 1, 2, "#13222c");
  drawCell(11, 13, 2, 1, current.agentPaused ? "#8ca3b6" : current.status === "error" ? "#ee9174" : "#355466");
  if (current.status === "running") {
    drawCell(6, 11, 1, 3, accent);
    drawCell(17, 11, 1, 3, accent);
  }
  if (current.agentPaused) {
    drawCell(9, 7, 7, 2, "#6c85a3");
    drawCell(15, 8, 2, 3, "#6c85a3");
  }
}

function getMeetingRecorderMimeType() {
  const candidates = [
    "audio/webm;codecs=opus",
    "audio/webm",
    "audio/mp4"
  ];
  return candidates.find((mimeType) => MediaRecorder.isTypeSupported(mimeType)) || "";
}

function arrayBufferToBase64(arrayBuffer) {
  const bytes = new Uint8Array(arrayBuffer);
  const chunkSize = 0x8000;
  let binary = "";

  for (let index = 0; index < bytes.length; index += chunkSize) {
    const slice = bytes.subarray(index, index + chunkSize);
    binary += String.fromCharCode(...slice);
  }

  return btoa(binary);
}

function getAudioContext() {
  if (!sharedAudioContext) {
    const AudioContextClass = window.AudioContext || window.webkitAudioContext;
    sharedAudioContext = new AudioContextClass();
  }
  return sharedAudioContext;
}

function mixAudioBufferToMono(audioBuffer) {
  const { numberOfChannels, length } = audioBuffer;
  if (numberOfChannels === 1) {
    return audioBuffer.getChannelData(0);
  }

  const mixed = new Float32Array(length);
  for (let channelIndex = 0; channelIndex < numberOfChannels; channelIndex += 1) {
    const channelData = audioBuffer.getChannelData(channelIndex);
    for (let index = 0; index < length; index += 1) {
      mixed[index] += channelData[index] / numberOfChannels;
    }
  }
  return mixed;
}

function encodeMonoPcm16Wav(audioBuffer) {
  const channelData = mixAudioBufferToMono(audioBuffer);
  const sampleRate = audioBuffer.sampleRate;
  const bytesPerSample = 2;
  const dataLength = channelData.length * bytesPerSample;
  const buffer = new ArrayBuffer(44 + dataLength);
  const view = new DataView(buffer);

  function writeAscii(offset, value) {
    for (let index = 0; index < value.length; index += 1) {
      view.setUint8(offset + index, value.charCodeAt(index));
    }
  }

  writeAscii(0, "RIFF");
  view.setUint32(4, 36 + dataLength, true);
  writeAscii(8, "WAVE");
  writeAscii(12, "fmt ");
  view.setUint32(16, 16, true);
  view.setUint16(20, 1, true);
  view.setUint16(22, 1, true);
  view.setUint32(24, sampleRate, true);
  view.setUint32(28, sampleRate * bytesPerSample, true);
  view.setUint16(32, bytesPerSample, true);
  view.setUint16(34, 16, true);
  writeAscii(36, "data");
  view.setUint32(40, dataLength, true);

  let offset = 44;
  for (let index = 0; index < channelData.length; index += 1) {
    const sample = Math.max(-1, Math.min(1, channelData[index]));
    view.setInt16(offset, sample < 0 ? sample * 0x8000 : sample * 0x7fff, true);
    offset += bytesPerSample;
  }

  return buffer;
}

async function buildUploadPayload(blob) {
  const arrayBuffer = await blob.arrayBuffer();
  try {
    const audioBuffer = await getAudioContext().decodeAudioData(arrayBuffer.slice(0));
    const wavBuffer = encodeMonoPcm16Wav(audioBuffer);
    return {
      audioBase64: arrayBufferToBase64(wavBuffer),
      mimeType: "audio/wav"
    };
  } catch (_error) {
    return {
      audioBase64: arrayBufferToBase64(arrayBuffer),
      mimeType: blob.type || getMeetingRecorderMimeType() || "audio/webm"
    };
  }
}

function downsampleFloat32Buffer(samples, inputSampleRate, outputSampleRate) {
  if (!samples?.length) {
    return new Float32Array(0);
  }

  if (inputSampleRate === outputSampleRate) {
    return samples;
  }

  const sampleRateRatio = inputSampleRate / outputSampleRate;
  const outputLength = Math.max(1, Math.round(samples.length / sampleRateRatio));
  const result = new Float32Array(outputLength);
  let outputIndex = 0;
  let inputIndex = 0;

  while (outputIndex < outputLength) {
    const nextInputIndex = Math.min(samples.length, Math.round((outputIndex + 1) * sampleRateRatio));
    let sum = 0;
    let count = 0;

    while (inputIndex < nextInputIndex) {
      sum += samples[inputIndex];
      inputIndex += 1;
      count += 1;
    }

    result[outputIndex] = count ? sum / count : samples[Math.min(inputIndex, samples.length - 1)] || 0;
    outputIndex += 1;
  }

  return result;
}

function encodePcm16Base64(samples) {
  if (!samples?.length) {
    return "";
  }

  const pcmBuffer = new ArrayBuffer(samples.length * 2);
  const pcmView = new DataView(pcmBuffer);
  for (let index = 0; index < samples.length; index += 1) {
    const sample = Math.max(-1, Math.min(1, samples[index]));
    pcmView.setInt16(index * 2, sample < 0 ? sample * 0x8000 : sample * 0x7fff, true);
  }
  return arrayBufferToBase64(pcmBuffer);
}

function buildRealtimeAudioPayload(inputBuffer, outputSampleRate) {
  const monoSamples = mixAudioBufferToMono(inputBuffer);
  const normalizedSamples = downsampleFloat32Buffer(monoSamples, inputBuffer.sampleRate, outputSampleRate);
  return encodePcm16Base64(normalizedSamples);
}

function pushRealtimeAudioChunk(source, audioBase64) {
  if (!audioBase64) {
    return;
  }

  const runtime = getSourceRuntime(source);
  runtime.queue = runtime.queue.then(() => {
    api.appendMeetingSummaryAudio({
      source,
      audioBase64
    });
  });
}

async function attachRealtimeStream(source, stream, outputSampleRate) {
  const runtime = getSourceRuntime(source);
  const AudioContextClass = window.AudioContext || window.webkitAudioContext;
  const captureContext = new AudioContextClass();
  const inputNode = captureContext.createMediaStreamSource(stream);
  const processorNode = captureContext.createScriptProcessor(4096, 1, 1);
  const sinkNode = captureContext.createGain();

  sinkNode.gain.value = 0;
  runtime.stream = stream;
  runtime.captureContext = captureContext;
  runtime.inputNode = inputNode;
  runtime.processorNode = processorNode;
  runtime.sinkNode = sinkNode;
  runtime.queue = Promise.resolve();
  runtime.sequence = 0;
  runtime.pendingStopResolve = null;
  runtime.closing = false;
  runtime.mode = "realtime";

  processorNode.onaudioprocess = (event) => {
    if (runtime.closing || state.meetingStopping) {
      return;
    }

    const audioBase64 = buildRealtimeAudioPayload(event.inputBuffer, outputSampleRate);
    if (audioBase64) {
      pushRealtimeAudioChunk(source, audioBase64);
    }
  };

  inputNode.connect(processorNode);
  processorNode.connect(sinkNode);
  sinkNode.connect(captureContext.destination);

  try {
    await captureContext.resume();
  } catch (_error) {
    // noop
  }
}

async function stopMeetingSource(source) {
  const runtime = getSourceRuntime(source);
  runtime.closing = true;

  const stopPromise = runtime.recorder && runtime.recorder.state !== "inactive"
    ? new Promise((resolve) => {
      runtime.pendingStopResolve = resolve;
      runtime.recorder.stop();
    })
    : Promise.resolve();

  if (runtime.processorNode) {
    runtime.processorNode.onaudioprocess = null;
    runtime.processorNode.disconnect();
  }
  if (runtime.inputNode) {
    runtime.inputNode.disconnect();
  }
  if (runtime.sinkNode) {
    runtime.sinkNode.disconnect();
  }

  if (runtime.stream) {
    runtime.stream.getTracks().forEach((track) => {
      if (track.readyState === "live") {
        track.stop();
      }
    });
  }

  await stopPromise;
  if (runtime.captureContext) {
    await runtime.captureContext.close().catch(() => {});
  }
  await runtime.queue.catch(() => {});
  state.meetingSources[source] = createMeetingSourceRuntime();
}

async function handleSourceCaptureFailure(source, error, options = {}) {
  const runtime = getSourceRuntime(source);
  if (!runtime.stream && !runtime.recorder && !runtime.captureContext && !options.forceNotify) {
    return;
  }

  await stopMeetingSource(source);

  if (!options.skipNotifyMain) {
    await api.reportMeetingSummarySourceError({
      source,
      message: normalizeErrorMessage(error, `${getSourceLabel(source)}采集失败`)
    });
  }

  if (!hasActiveMeetingRuntime() && !state.meetingStopping) {
    clearMeetingSummaryTimer();
    await api.reportMeetingSummaryError("系统音频和麦克风都不可用或已停止，请检查共享权限和输入设备。");
  }
}

function enqueueMeetingChunk(source, blob) {
  if (!blob || !blob.size) {
    return getSourceRuntime(source).queue;
  }

  const runtime = getSourceRuntime(source);
  const sequence = ++runtime.sequence;
  runtime.queue = runtime.queue.then(async () => {
    const uploadPayload = await buildUploadPayload(blob);
    const result = await api.processMeetingSummaryChunk({
      source,
      audioBase64: uploadPayload.audioBase64,
      mimeType: uploadPayload.mimeType,
      sequence
    });

    if (!result?.ok) {
      if (result?.handled) {
        await handleSourceCaptureFailure(source, new Error(result.error || `${getSourceLabel(source)}转写失败`), {
          skipNotifyMain: true
        });
        return;
      }
      throw new Error(result?.error || `${getSourceLabel(source)}转写失败`);
    }
  }).catch(async (error) => {
    await handleSourceCaptureFailure(source, error);
  });

  return runtime.queue;
}

function attachRecorder(source, stream, transcriptIntervalMs) {
  const runtime = getSourceRuntime(source);
  const mimeType = getMeetingRecorderMimeType();
  const recorderOptions = mimeType
    ? { mimeType, audioBitsPerSecond: 96000 }
    : { audioBitsPerSecond: 96000 };
  const recorder = new MediaRecorder(stream, recorderOptions);

  runtime.stream = stream;
  runtime.recorder = recorder;
  runtime.queue = Promise.resolve();
  runtime.sequence = 0;
  runtime.pendingStopResolve = null;
  runtime.closing = false;
  runtime.mode = "chunk";

  recorder.addEventListener("dataavailable", (event) => {
    if (event.data && event.data.size > 0) {
      void enqueueMeetingChunk(source, event.data);
    }
  });

  recorder.addEventListener("stop", () => {
    if (runtime.pendingStopResolve) {
      runtime.pendingStopResolve();
      runtime.pendingStopResolve = null;
    }
  });

  stream.getAudioTracks().forEach((track) => {
    track.addEventListener("ended", () => {
      if (!runtime.closing && !state.meetingStopping) {
        void handleSourceCaptureFailure(source, new Error(`${getSourceLabel(source)}输入已结束`));
      }
    }, { once: true });
  });

  recorder.start(transcriptIntervalMs);
}

async function startSystemAudioSource(options = {}) {
  try {
    const captureStream = await navigator.mediaDevices.getDisplayMedia({
      video: true,
      audio: {
        echoCancellation: false,
        noiseSuppression: false,
        autoGainControl: false
      }
    });

    const audioTracks = captureStream.getAudioTracks();
    captureStream.getVideoTracks().forEach((track) => track.stop());
    if (!audioTracks.length) {
      captureStream.getTracks().forEach((track) => track.stop());
      throw new Error("没有获取到系统音频，请确认会议声音正在通过电脑播放。");
    }

    const audioStream = new MediaStream(audioTracks);
    if (options.transcriptionMode === "realtime") {
      await attachRealtimeStream("system", audioStream, options.realtimeSampleRate || 16000);
    } else {
      attachRecorder("system", audioStream, options.transcriptIntervalMs || DEFAULT_TRANSCRIPT_INTERVAL_MS);
    }
    api.meetingSummaryCaptureReady({ source: "system" });
    return true;
  } catch (error) {
    await api.reportMeetingSummarySourceError({
      source: "system",
      message: normalizeErrorMessage(error, "系统音频接入失败")
    });
    return false;
  }
}

async function startMicrophoneSource(options = {}) {
  try {
    const stream = await navigator.mediaDevices.getUserMedia({
      audio: {
        echoCancellation: true,
        noiseSuppression: true,
        autoGainControl: true
      }
    });

    const audioTracks = stream.getAudioTracks();
    if (!audioTracks.length) {
      stream.getTracks().forEach((track) => track.stop());
      throw new Error("没有检测到麦克风输入。");
    }

    const audioStream = new MediaStream(audioTracks);
    if (options.transcriptionMode === "realtime") {
      await attachRealtimeStream("microphone", audioStream, options.realtimeSampleRate || 16000);
    } else {
      attachRecorder("microphone", audioStream, options.transcriptIntervalMs || DEFAULT_TRANSCRIPT_INTERVAL_MS);
    }
    api.meetingSummaryCaptureReady({ source: "microphone" });
    return true;
  } catch (error) {
    await api.reportMeetingSummarySourceError({
      source: "microphone",
      message: normalizeErrorMessage(error, "麦克风接入失败")
    });
    return false;
  }
}

async function beginMeetingSummaryCapture(result) {
  state.meetingTranscriptionMode = result.transcriptionMode || "chunk";
  state.lastHandledMeetingCaptureRequestId = result.captureRequestId || state.lastHandledMeetingCaptureRequestId;
  clearMeetingSummaryTimer();
  await Promise.all([
    stopMeetingSource("system"),
    stopMeetingSource("microphone")
  ]);

  const sourceOptions = {
    transcriptionMode: state.meetingTranscriptionMode,
    transcriptIntervalMs: result.transcriptIntervalMs || DEFAULT_TRANSCRIPT_INTERVAL_MS,
    realtimeSampleRate: result.realtimeSampleRate || 16000
  };
  const [systemReady, microphoneReady] = await Promise.all([
    startSystemAudioSource(sourceOptions),
    startMicrophoneSource(sourceOptions)
  ]);

  renderMeetingControls();

  if (!systemReady && !microphoneReady) {
    await api.reportMeetingSummaryError("系统音频和麦克风都没有连接成功，请检查权限和设备。");
    return;
  }

  startMeetingSummaryTimer(result.intervalMs || DEFAULT_MEETING_INTERVAL_MS);
}

async function startMeetingSummaryCapture(trigger = "manual") {
  const result = await api.startMeetingSummary({ trigger });
  if (!result?.ok) {
    return;
  }

  await beginMeetingSummaryCapture(result);
}

async function stopMeetingSummaryCapture(options = {}) {
  if (state.meetingStopping) {
    return;
  }

  state.meetingStopping = true;
  renderMeetingControls();
  clearMeetingSummaryTimer();

  await Promise.all([
    stopMeetingSource("system"),
    stopMeetingSource("microphone")
  ]);

  if (getMeetingTranscriptionMode() === "realtime") {
    await api.finalizeMeetingRealtimeTranscription();
  }

  await api.refreshMeetingSummary({ force: true });
  if (options.notifyMain !== false) {
    await api.stopMeetingSummary();
  }

  state.meetingStopping = false;
  renderMeetingControls();
}

function syncMeetingCaptureWithSnapshot(snapshot) {
  const meeting = snapshot.state.meetingSummary;
  if (meeting?.enabled) {
    const captureRequestId = meeting.captureRequestId || "";
    if (captureRequestId && captureRequestId !== state.lastHandledMeetingCaptureRequestId && !hasActiveMeetingRuntime()) {
      state.lastHandledMeetingCaptureRequestId = captureRequestId;
      void beginMeetingSummaryCapture({
        captureRequestId,
        transcriptionMode: meeting.transcriptionMode,
        transcriptIntervalMs: meeting.transcriptIntervalMs,
        realtimeSampleRate: meeting.realtimeSampleRate,
        intervalMs: meeting.intervalMs
      });
    }
    return;
  }

  state.lastHandledMeetingCaptureRequestId = "";
  if (hasActiveMeetingRuntime() && !state.meetingStopping) {
    void stopMeetingSummaryCapture({ notifyMain: false });
  }
}

async function toggleMeetingSummary() {
  const meeting = getMeetingState();
  if (meeting?.enabled || hasActiveMeetingRuntime()) {
    await stopMeetingSummaryCapture();
    return;
  }

  await startMeetingSummaryCapture();
}

function bindEvents() {
  elements.pinButton.addEventListener("click", () => {
    api.updatePreferences({
      chatPinned: !state.snapshot?.state.chatPinned
    });
  });

  elements.historyButton.addEventListener("click", () => {
    state.historyOpen = !state.historyOpen;
    renderHistory();
    elements.historyButton.classList.toggle("active", state.historyOpen);
  });

  elements.closeButton.addEventListener("click", () => {
    api.closeChat();
  });

  elements.cancelTaskButton.addEventListener("click", () => {
    api.cancelTask();
  });

  elements.meetingPromptAcceptButton.addEventListener("click", async () => {
    await api.acceptMeetingDetectionPrompt();
  });

  elements.meetingPromptDismissButton.addEventListener("click", async () => {
    await api.dismissMeetingDetectionPrompt();
  });

  elements.meetingCancelButton.addEventListener("click", () => {
    void stopMeetingSummaryCapture();
  });

  elements.meetingSummaryButton.addEventListener("click", () => {
    void toggleMeetingSummary();
  });

  elements.showSummaryOverlayButton.addEventListener("click", () => {
    api.showMeetingSummaryOverlay();
  });

  elements.showCaptionsOverlayButton.addEventListener("click", () => {
    api.showMeetingCaptionsOverlay();
  });

  elements.showSummaryOverlayToolbarButton.addEventListener("click", () => {
    api.showMeetingSummaryOverlay();
  });

  elements.showCaptionsOverlayToolbarButton.addEventListener("click", () => {
    api.showMeetingCaptionsOverlay();
  });

  elements.selectionButton.addEventListener("click", () => {
    api.sendChat("请基于当前选区继续");
  });

  elements.clipboardButton.addEventListener("click", () => {
    api.runTask("clipboard", "chat");
  });

  elements.sendButton.addEventListener("click", sendInput);
  elements.composerInput.addEventListener("input", autoresize);
  elements.composerInput.addEventListener("keydown", (event) => {
    if (event.key === "Enter" && !event.shiftKey) {
      event.preventDefault();
      sendInput();
    }
  });
}

async function init() {
  const snapshot = await api.bootstrap();
  applySnapshot(snapshot);
  drawAvatar();
  api.onSnapshot((next) => {
    applySnapshot(next);
    drawAvatar();
  });
  bindEvents();
  autoresize();
}

init();
