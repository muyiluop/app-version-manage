import { client } from "./client";
import type { CleanResult, MessageResult, PageQuery, Paginated, StoredFile, UploadResult } from "../types/api";

export interface FileListQuery extends PageQuery {
  keyword?: string;
}

export function listFiles(params: FileListQuery): Promise<Paginated<StoredFile>> {
  return client.get<Paginated<StoredFile>>("/v2/files", { params });
}

/** 上传文件，multipart 字段名固定为 file；onProgress 用于进度条。 */
export function uploadFile(file: File, onProgress?: (percent: number) => void): Promise<UploadResult> {
  const form = new FormData();
  form.append("file", file);
  return client.post<UploadResult>("/v2/files/upload", form, {
    timeout: 0,
    onUploadProgress: (event) => {
      if (onProgress && event.total) {
        onProgress(Math.round((event.loaded / event.total) * 100));
      }
    },
  });
}

/** 仍被版本引用的文件删除会返回 409。 */
export function deleteFile(id: number): Promise<MessageResult> {
  return client.delete<MessageResult>(`/v2/files/${id}`);
}

/** 清理未被引用的孤儿文件。 */
export function cleanFiles(): Promise<CleanResult> {
  return client.post<CleanResult>("/v2/files/clean");
}

export function downloadFile(id: number): Promise<Blob> {
  return client.get<Blob>(`/v2/files/${id}/download`, { responseType: "blob", timeout: 0 });
}
