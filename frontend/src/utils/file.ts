import axios from "axios";
import { saveAs } from "file-saver";

/**
 * 将文件路径转换为可访问的URL
 * @param path 文件路径
 * @param download 是否为下载链接
 * @returns 完整的文件URL
 */
export function getFileUrl(path: string, download: boolean = false): string {
  if (!path) return "";
  // 如果是下载链接，使用专门的下载接口
  if (download) {
    return `${import.meta.env.VITE_API_BASE_URL}/files/download/${path}`;
  }

  // 否则使用静态文件访问
  return `${import.meta.env.VITE_FILE_BASE_URL}/${path}`;
}

/**
 * 下载文件
 * @param path 文件路径
 * @param fileName 下载时的文件名
 */
export async function downloadFile(path: string, fileName?: string, isTempLink: boolean = false): Promise<void> {
  try {
    let fileUrl = `/files/download/${path}`;
    if (isTempLink) {
      fileUrl = `/open/download/${path}`;
    }
    const response = await axios.get(fileUrl, {
      responseType: "blob",
    });

    // 使用 file-saver 的 saveAs 方法，提高移动平台兼容性
    saveAs(response.data, fileName || path);
  } catch (error) {
    console.error("下载文件失败:", error);
    throw error;
  }
}
