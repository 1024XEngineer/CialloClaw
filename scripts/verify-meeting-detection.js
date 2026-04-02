const MEETING_APP_SIGNATURES = [
  {
    id: "tencent-meeting",
    label: "腾讯会议",
    processNames: ["wemeetapp", "wemeeting", "voovmeeting", "voovmeetingapp"],
    titleKeywords: ["腾讯会议", "voov meeting", "wemeet"]
  },
  {
    id: "lark-meeting",
    label: "飞书会议",
    processNames: ["feishu", "lark", "feishumeeting"],
    titleKeywords: ["飞书会议", "lark meetings", "feishu meetings", "飞书"]
  },
  {
    id: "zoom",
    label: "Zoom",
    processNames: ["zoom", "zoom workplace", "zoomrooms"],
    titleKeywords: ["zoom meeting", "zoom workplace", "zoom"]
  },
  {
    id: "teams",
    label: "Microsoft Teams",
    processNames: ["teams", "ms-teams", "msteams"],
    titleKeywords: ["microsoft teams", "teams meeting", "teams"]
  },
  {
    id: "dingtalk-meeting",
    label: "钉钉会议",
    processNames: ["dingtalk"],
    titleKeywords: ["钉钉会议", "钉钉"]
  },
  {
    id: "wecom-meeting",
    label: "企业微信会议",
    processNames: ["wxwork", "wecom"],
    titleKeywords: ["企业微信会议", "企微会议", "wecom"]
  },
  {
    id: "google-meet",
    label: "Google Meet",
    processNames: ["chrome", "msedge", "firefox", "brave", "opera"],
    titleKeywords: ["google meet", "meet.google.com", "google 会议"]
  }
];

function normalizeMatchText(value) {
  return String(value || "").trim().toLowerCase();
}

function matchMeetingApp(windowInfo = {}) {
  const processName = normalizeMatchText(windowInfo.processName);
  const title = normalizeMatchText(windowInfo.title);
  if (!processName && !title) {
    return null;
  }

  return MEETING_APP_SIGNATURES.find((signature) => {
    const processMatched = signature.processNames.some((keyword) => processName.includes(normalizeMatchText(keyword)));
    const titleMatched = signature.titleKeywords.some((keyword) => title.includes(normalizeMatchText(keyword)));
    if (signature.id === "google-meet") {
      return processMatched && titleMatched;
    }
    return processMatched || titleMatched;
  }) || null;
}

const cases = [
  {
    name: "Tencent Meeting process",
    input: {
      processName: "wemeetapp",
      title: "腾讯会议 - 项目周会"
    },
    expected: "tencent-meeting"
  },
  {
    name: "Lark process",
    input: {
      processName: "Feishu",
      title: "飞书会议 - 评审会"
    },
    expected: "lark-meeting"
  },
  {
    name: "Zoom process",
    input: {
      processName: "Zoom",
      title: "Zoom Meeting"
    },
    expected: "zoom"
  },
  {
    name: "Teams title",
    input: {
      processName: "ms-teams",
      title: "Microsoft Teams | 产品例会"
    },
    expected: "teams"
  },
  {
    name: "Google Meet in Edge",
    input: {
      processName: "msedge",
      title: "Standup - Google Meet"
    },
    expected: "google-meet"
  },
  {
    name: "Non meeting browser page",
    input: {
      processName: "chrome",
      title: "OpenAI Docs"
    },
    expected: null
  }
];

let failed = 0;

cases.forEach((testCase) => {
  const matched = matchMeetingApp(testCase.input);
  const actual = matched?.id || null;
  const ok = actual === testCase.expected;
  if (!ok) {
    failed += 1;
  }

  const detail = `${testCase.name}: expected=${testCase.expected || "null"} actual=${actual || "null"}`;
  console.log(ok ? `PASS ${detail}` : `FAIL ${detail}`);
});

if (failed) {
  process.exitCode = 1;
  console.error(`meeting detection smoke test failed: ${failed}`);
} else {
  console.log("meeting detection smoke test passed");
}
