/**
 * 浏览器端二进制保存。
 *
 * 为什么不用 file-saver：依赖已被移除，且这里只需处理 Blob 与临时 URL，
 * 自己实现可以避免为一个小功能保留一个额外依赖。
 */

function triggerDownload(url: string, filename: string): void {
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = filename;
  anchor.rel = "noopener";
  document.body.appendChild(anchor);
  anchor.click();
  document.body.removeChild(anchor);
}

/** 保存 Blob 到本地。 */
export function saveBlob(blob: Blob, filename: string): void {
  const url = URL.createObjectURL(blob);
  try {
    triggerDownload(url, filename);
  } finally {
    // 立即回收会让部分浏览器来不及开始下载，延迟释放更稳妥。
    window.setTimeout(() => URL.revokeObjectURL(url), 10_000);
  }
}

/** 直接跳转到无需鉴权的下载地址（如 /api/open/download/<token>）。 */
export function downloadByUrl(url: string, filename?: string): void {
  triggerDownload(url, filename ?? "");
}
