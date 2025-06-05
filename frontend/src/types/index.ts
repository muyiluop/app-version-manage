export type Platform = "android" | "ios" | "windows" | "linux" | "macos" | "harmony";

export interface Application {
  id: number;
  name: string;
  identifier: string;
  logo?: string;
  description?: string;
  platforms: Platform[];
  createdAt: string;
  updatedAt: string;
}

export interface Version {
  id: number;
  appId: number;
  platform: Platform;
  version: string;
  filePath: string;
  fileName: string;
  fileSize: number;
  changelog?: string;
  forceUpdate: boolean;
  isActive: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface Template {
  id: number;
  appId: number;
  name: string;
  content: string;
  createdAt: string;
  updatedAt: string;
}

export interface File {
  id: number;
  name: string;
  path: string;
  size: number;
  type?: string;
  hash: string;
  createdAt: string;
  updatedAt: string;
}
