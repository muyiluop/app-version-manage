import { useCallback, useState } from "react";

/**
 * 统一的提交态管理：驱动表单按钮 loading，并吞掉异常。
 * 错误提示已由 axios 响应拦截器统一处理，这里不再重复弹窗。
 */
export function useSubmit(): [boolean, (task: () => Promise<void>) => Promise<void>] {
  const [submitting, setSubmitting] = useState(false);

  const run = useCallback(async (task: () => Promise<void>) => {
    setSubmitting(true);
    try {
      await task();
    } catch {
      // 由拦截器提示
    } finally {
      setSubmitting(false);
    }
  }, []);

  return [submitting, run];
}
