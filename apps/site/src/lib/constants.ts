export const SITE_URL = "https://cialloclaw.vercel.app";

export const PRIMARY_NAV_ITEMS = [
  { href: "#product", label: "Product" },
  { href: "#workflow", label: "How it works" },
  { href: "#safety", label: "Safety" },
  { href: "#download", label: "Download" },
  { href: "/docs", label: "Docs" },
  { href: "https://github.com/1024XEngineer/CialloClaw", label: "GitHub" },
] as const;

export const DOC_LINKS = [
  {
    href: "/docs#architecture-overview",
    label: "Architecture overview",
    description: "Read how task, run, delivery_result, artifact, and governance fit together.",
  },
  {
    href: "/docs#product-interaction-design",
    label: "Product interaction design",
    description: "See how the floating ball, lightweight input, and dashboard work together.",
  },
  {
    href: "/docs#dashboard-design",
    label: "Dashboard design",
    description: "Review the dashboard as the task-centric control surface and trust summary.",
  },
  {
    href: "/docs#control-panel-settings",
    label: "Control panel settings",
    description: "Explore provider, memory, automation, and safety settings in one place.",
  },
  {
    href: "/docs#protocol-design",
    label: "Protocol design",
    description: "Check the JSON-RPC boundary that keeps the desktop UI and local harness aligned.",
  },
  {
    href: "https://github.com/1024XEngineer/CialloClaw/issues/332#issue-4321666828",
    label: "Issue #332",
    description: "Follow the original website requirements for lively interaction and release sync.",
  },
] as const;

export const FAQ_ITEMS = [
  {
    question: "Is CialloClaw a chatbot?",
    answer:
      "No. CialloClaw is a Windows-first desktop collaboration Agent. It uses lightweight nearby entry points, task-centric orchestration, and formal delivery_result and artifact outputs instead of making chat the whole product.",
  },
  {
    question: "What platforms does it target today?",
    answer:
      "The current product direction and engineering baseline are Windows first. The desktop runtime, local-service harness, and task flows are all designed around a Windows desktop workspace.",
  },
  {
    question: "Why is the floating ball emphasized so much?",
    answer:
      "Because the floating ball is the low-friction entry point. It lets users start from selected text, dragged files, current errors, hover input, or voice without switching into a heavy chat window first.",
  },
  {
    question: "How are risky actions handled?",
    answer:
      "File writes, command execution, workspace boundary changes, and other risky actions are designed to flow through approval_request, authorization_record, audit_record, and recovery_point safeguards before formal delivery.",
  },
  {
    question: "What is the difference between Stable and Tip Preview?",
    answer:
      "Stable is the recommended release channel for most users. Tip Preview tracks the faster-moving tag intended for developers and early testers who want the latest changes sooner.",
  },
  {
    question: "Do website downloads talk to the local desktop runtime?",
    answer:
      "No. The website is intentionally independent from apps/desktop, local-service, workers, Named Pipe, and local storage. It only presents product information and synced release metadata.",
  },
] as const;
