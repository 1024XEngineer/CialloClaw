const fs = require("fs");
const os = require("os");
const path = require("path");

function sanitizeModelConfig(input = {}) {
  return {
    baseUrl: typeof input.baseUrl === "string" ? input.baseUrl.trim().replace(/\/+$/, "") : "",
    apiKey: typeof input.apiKey === "string" ? input.apiKey.trim() : "",
    model: typeof input.model === "string" ? input.model.trim() : ""
  };
}

function readSavedConfig() {
  const settingsPath = path.join(process.env.APPDATA || path.join(os.homedir(), "AppData", "Roaming"), "Pixel Orb Demo", "pixel-orb-settings.json");
  const parsed = JSON.parse(fs.readFileSync(settingsPath, "utf8"));
  return {
    settingsPath,
    transcription: sanitizeModelConfig(parsed?.modelConfig?.transcription),
    summary: sanitizeModelConfig(parsed?.modelConfig?.summary)
  };
}

function buildApiUrl(baseUrl, route) {
  const normalizedBase = baseUrl.replace(/\/+$/, "");
  const normalizedRoute = route.startsWith("/") ? route : `/${route}`;
  return `${normalizedBase}${normalizedRoute}`;
}

function isDashScopeCompatibleBaseUrl(baseUrl) {
  return /dashscope(-intl)?\.aliyuncs\.com\/compatible-mode\/v1\/?$/i.test((baseUrl || "").trim());
}

function buildInputAudioDataUri(mimeType, audioBase64) {
  return `data:${mimeType || "audio/wav"};base64,${audioBase64}`;
}

function extractAssistantText(payload) {
  const content = payload?.choices?.[0]?.message?.content;
  if (typeof content === "string") {
    return content.trim();
  }
  if (Array.isArray(content)) {
    return content
      .map((item) => {
        if (typeof item === "string") {
          return item;
        }
        if (typeof item?.text === "string") {
          return item.text;
        }
        return "";
      })
      .join("")
      .trim();
  }
  return "";
}

function getApiErrorMessage(payload) {
  if (!payload) {
    return "";
  }
  if (typeof payload === "string") {
    return payload.trim();
  }
  if (typeof payload?.error?.message === "string") {
    return payload.error.message.trim();
  }
  if (typeof payload?.message === "string") {
    return payload.message.trim();
  }
  if (typeof payload?.code === "string") {
    return payload.code.trim();
  }
  return "";
}

async function parseApiResponse(response) {
  const contentType = response.headers.get("content-type") || "";
  if (contentType.includes("application/json")) {
    return response.json();
  }

  const text = await response.text();
  try {
    return JSON.parse(text);
  } catch (_error) {
    return text;
  }
}

function assertCompleteConfig(name, config) {
  if (!config.baseUrl || !config.apiKey || !config.model) {
    throw new Error(`${name} 配置不完整`);
  }
}

async function transcribeAudio(config, audioPath) {
  assertCompleteConfig("转写", config);
  const audioBuffer = fs.readFileSync(audioPath);
  const audioBase64 = audioBuffer.toString("base64");
  const mimeType = "audio/wav";

  if (isDashScopeCompatibleBaseUrl(config.baseUrl)) {
    const response = await fetch(buildApiUrl(config.baseUrl, "/chat/completions"), {
      method: "POST",
      headers: {
        Authorization: `Bearer ${config.apiKey}`,
        "Content-Type": "application/json"
      },
      body: JSON.stringify({
        model: config.model,
        messages: [
          {
            role: "user",
            content: [
              {
                type: "input_audio",
                input_audio: {
                  data: buildInputAudioDataUri(mimeType, audioBase64)
                }
              }
            ]
          }
        ],
        stream: false,
        asr_options: {
          enable_itn: false
        }
      })
    });

    const payload = await parseApiResponse(response);
    if (!response.ok) {
      throw new Error(`转写请求失败 (${response.status}): ${getApiErrorMessage(payload) || "未知错误"}`);
    }

    return extractAssistantText(payload);
  }

  const audioBlob = new Blob([audioBuffer], { type: mimeType });
  const formData = new FormData();
  formData.set("file", audioBlob, path.basename(audioPath));
  formData.set("model", config.model);

  const response = await fetch(buildApiUrl(config.baseUrl, "/audio/transcriptions"), {
    method: "POST",
    headers: {
      Authorization: `Bearer ${config.apiKey}`
    },
    body: formData
  });

  const payload = await parseApiResponse(response);
  if (!response.ok) {
    throw new Error(`转写请求失败 (${response.status}): ${getApiErrorMessage(payload) || "未知错误"}`);
  }

  if (typeof payload === "string") {
    return payload.trim();
  }
  return typeof payload?.text === "string" ? payload.text.trim() : "";
}

async function summarizeTranscript(config, transcript) {
  assertCompleteConfig("总结", config);
  const response = await fetch(buildApiUrl(config.baseUrl, "/chat/completions"), {
    method: "POST",
    headers: {
      Authorization: `Bearer ${config.apiKey}`,
      "Content-Type": "application/json"
    },
    body: JSON.stringify({
      model: config.model,
      temperature: 0.2,
      messages: [
        {
          role: "system",
          content: "你是一个严谨的会议纪要助手。请把给出的转写内容概括成主题、结论、待办、风险四部分，用简体中文输出。"
        },
        {
          role: "user",
          content: transcript
        }
      ]
    })
  });

  const payload = await parseApiResponse(response);
  if (!response.ok) {
    throw new Error(`总结请求失败 (${response.status}): ${getApiErrorMessage(payload) || "未知错误"}`);
  }

  const summary = extractAssistantText(payload);
  if (!summary) {
    throw new Error("总结接口返回成功，但没有解析出文本");
  }
  return summary;
}

async function main() {
  const audioPath = process.argv[2];
  if (!audioPath) {
    throw new Error("请传入待验证的 wav 音频路径");
  }

  const startedAt = Date.now();
  const configBundle = readSavedConfig();
  const transcript = await transcribeAudio(configBundle.transcription, audioPath);
  const summary = await summarizeTranscript(configBundle.summary, transcript || "测试音频未识别出清晰文本");
  const elapsedMs = Date.now() - startedAt;

  process.stdout.write(JSON.stringify({
    ok: true,
    settingsPath: configBundle.settingsPath,
    transcriptionModel: configBundle.transcription.model,
    summaryModel: configBundle.summary.model,
    transcript,
    summary,
    elapsedMs
  }, null, 2));
}

main().catch((error) => {
  process.stderr.write(String(error?.stack || error?.message || error));
  process.exitCode = 1;
});
