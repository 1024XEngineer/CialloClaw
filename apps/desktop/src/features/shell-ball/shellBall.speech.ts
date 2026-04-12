type ShellBallSpeechRecognitionAlternative = {
  transcript: string;
};

type ShellBallSpeechRecognitionResult = {
  isFinal: boolean;
  length: number;
  [index: number]: ShellBallSpeechRecognitionAlternative;
};

export type ShellBallSpeechRecognitionResultList = {
  length: number;
  [index: number]: ShellBallSpeechRecognitionResult;
};

export type ShellBallSpeechRecognitionEvent = {
  results: ShellBallSpeechRecognitionResultList;
};

export type ShellBallSpeechRecognitionErrorEvent = {
  error: string;
};

export type ShellBallSpeechRecognition = {
  continuous: boolean;
  interimResults: boolean;
  lang: string;
  maxAlternatives: number;
  onend: (() => void) | null;
  onerror: ((event: ShellBallSpeechRecognitionErrorEvent) => void) | null;
  onresult: ((event: ShellBallSpeechRecognitionEvent) => void) | null;
  start: () => void;
  stop: () => void;
  abort: () => void;
};

type ShellBallSpeechRecognitionConstructor = new () => ShellBallSpeechRecognition;

type ShellBallSpeechRecognitionWindow = Window & {
  SpeechRecognition?: ShellBallSpeechRecognitionConstructor;
  webkitSpeechRecognition?: ShellBallSpeechRecognitionConstructor;
};

export function getShellBallSpeechRecognitionConstructor(target: Window = window) {
  const host = target as ShellBallSpeechRecognitionWindow;
  return host.SpeechRecognition ?? host.webkitSpeechRecognition ?? null;
}

export function getShellBallSpeechRecognitionLanguage(target: Navigator = navigator) {
  return target.language || "zh-CN";
}

export function normalizeShellBallSpeechTranscript(transcript: string) {
  return transcript.replace(/\s+/g, " ").trim();
}

export function collectShellBallSpeechTranscript(results: ShellBallSpeechRecognitionResultList) {
  let transcript = "";

  for (let index = 0; index < results.length; index += 1) {
    const alternative = results[index]?.[0];

    if (alternative?.transcript === undefined) {
      continue;
    }

    transcript += ` ${alternative.transcript}`;
  }

  return normalizeShellBallSpeechTranscript(transcript);
}

export function composeShellBallSpeechDraft(baseDraft: string, transcript: string) {
  const normalizedTranscript = normalizeShellBallSpeechTranscript(transcript);

  if (normalizedTranscript === "") {
    return baseDraft;
  }

  if (baseDraft.trim() === "") {
    return normalizedTranscript;
  }

  const trimmedBaseDraft = baseDraft.trimEnd();
  if (/[\u3400-\u9fff]$/.test(trimmedBaseDraft) || /[。！？!?，、；：:,.]$/.test(trimmedBaseDraft)) {
    return `${trimmedBaseDraft}${normalizedTranscript}`;
  }

  return `${trimmedBaseDraft} ${normalizedTranscript}`;
}
