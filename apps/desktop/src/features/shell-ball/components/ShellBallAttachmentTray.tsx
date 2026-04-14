import { FileText, X } from "lucide-react";

type ShellBallAttachmentTrayProps = {
  paths: string[];
  onRemove: (path: string) => void;
};

export function ShellBallAttachmentTray({ paths, onRemove }: ShellBallAttachmentTrayProps) {
  if (paths.length === 0) {
    return null;
  }

  return (
    <ul className="shell-ball-attachment-tray" aria-label="Pending file attachments">
      {paths.map((path) => {
        const fileName = getShellBallAttachmentFileName(path);

        return (
          <li key={path} className="shell-ball-attachment-tray__item" title={fileName}>
            <div className="shell-ball-attachment-tray__meta">
              <span className="shell-ball-attachment-tray__leading">
                <FileText className="shell-ball-attachment-tray__icon" />
                <span className="shell-ball-attachment-tray__title" title={fileName}>{fileName}</span>
              </span>
            </div>
            <button
              type="button"
              className="shell-ball-attachment-tray__remove"
              aria-label={`Remove ${fileName}`}
              onClick={() => {
                onRemove(path);
              }}
            >
              <X className="shell-ball-attachment-tray__remove-icon" />
            </button>
          </li>
        );
      })}
    </ul>
  );
}

function getShellBallAttachmentFileName(filePath: string) {
  const normalizedPath = filePath.replace(/\\/g, "/").trim();
  if (normalizedPath === "") {
    return "未命名文件";
  }

  const segments = normalizedPath.split("/").filter((segment) => segment !== "");
  return segments.at(-1) ?? normalizedPath;
}
