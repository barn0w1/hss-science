export function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B"
  const k = 1024
  const sizes = ["B", "KB", "MB", "GB", "TB"]
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`
}

export function formatDate(iso: string): string {
  return new Intl.DateTimeFormat("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
  }).format(new Date(iso))
}

export type MimeCategory =
  | "image"
  | "pdf"
  | "video"
  | "audio"
  | "archive"
  | "code"
  | "text"
  | "spreadsheet"
  | "presentation"
  | "default"

export function getMimeCategory(mimeType: string): MimeCategory {
  if (mimeType.startsWith("image/")) return "image"
  if (mimeType === "application/pdf") return "pdf"
  if (mimeType.startsWith("video/")) return "video"
  if (mimeType.startsWith("audio/")) return "audio"
  if (
    mimeType === "application/zip" ||
    mimeType === "application/x-tar" ||
    mimeType === "application/gzip" ||
    mimeType === "application/x-7z-compressed" ||
    mimeType === "application/x-rar-compressed"
  )
    return "archive"
  if (
    mimeType.startsWith("text/") ||
    mimeType === "application/json" ||
    mimeType === "application/xml" ||
    mimeType === "application/javascript" ||
    mimeType === "application/typescript"
  ) {
    const codeTypes = [
      "javascript",
      "typescript",
      "json",
      "xml",
      "html",
      "css",
      "x-sh",
      "x-python",
      "x-ruby",
      "x-go",
      "x-rust",
    ]
    if (codeTypes.some((t) => mimeType.includes(t))) return "code"
    if (mimeType === "text/markdown" || mimeType === "text/plain") return "text"
    return "code"
  }
  if (
    mimeType.includes("spreadsheet") ||
    mimeType === "application/vnd.ms-excel"
  )
    return "spreadsheet"
  if (
    mimeType.includes("presentation") ||
    mimeType === "application/vnd.ms-powerpoint"
  )
    return "presentation"
  return "default"
}
