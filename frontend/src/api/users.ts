import { client } from "./client";
import type {
  CreateUserInput,
  MessageResult,
  PageQuery,
  Paginated,
  UpdateUserInput,
  UserProfile,
} from "../types/api";

/** 用户列表（仅 admin）。 */
export function listUsers(params: PageQuery): Promise<Paginated<UserProfile>> {
  return client.get<Paginated<UserProfile>>("/v2/users", { params });
}

export function createUser(input: CreateUserInput): Promise<UserProfile> {
  return client.post<UserProfile>("/v2/users", input);
}

/** 改角色/状态/重置密码都走这一接口。 */
export function updateUser(id: number, input: UpdateUserInput): Promise<UserProfile> {
  return client.put<UserProfile>(`/v2/users/${id}`, input);
}

export function deleteUser(id: number): Promise<MessageResult> {
  return client.delete<MessageResult>(`/v2/users/${id}`);
}
