/**
 * 基于 react-query 的轻量请求 hook。
 *
 * 为什么包一层：组件只关心 data/loading/error/refresh，不希望到处写
 * queryKey 与 error 收窄；同时保留契约要求的 useRequest 入口。
 */
import { useQuery, useQueryClient, type QueryKey } from "@tanstack/react-query";
import { getErrorMessage } from "../api/client";

export interface UseRequestOptions {
  /** 为 false 时不发起请求（例如必填参数尚未就绪）。 */
  enabled?: boolean;
  /** 数据新鲜时长（毫秒）。 */
  staleTime?: number;
}

export interface UseRequestResult<T> {
  data: T | undefined;
  loading: boolean;
  error: string | null;
  /** 重新拉取，供增删改后刷新列表。 */
  refresh: () => void;
}

export function useRequest<T>(
  queryKey: QueryKey,
  fetcher: () => Promise<T>,
  options: UseRequestOptions = {}
): UseRequestResult<T> {
  const query = useQuery({
    queryKey,
    queryFn: fetcher,
    enabled: options.enabled ?? true,
    staleTime: options.staleTime ?? 0,
  });

  return {
    data: query.data,
    // isFetching 覆盖首次加载与后台刷新，表格 loading 才不会闪断
    loading: query.isFetching,
    error: query.error === null ? null : getErrorMessage(query.error),
    refresh: () => {
      void query.refetch();
    },
  };
}

/** 供需要跨组件失效缓存的场景使用。 */
export function useInvalidate(): (key: QueryKey) => void {
  const queryClient = useQueryClient();
  return (key: QueryKey) => {
    void queryClient.invalidateQueries({ queryKey: key });
  };
}
