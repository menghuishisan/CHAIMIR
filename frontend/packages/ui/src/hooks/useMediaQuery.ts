/**
 * useMediaQuery:订阅媒体查询结果,供壳层在断点间切换形态。
 * 导航壳用它切常驻侧栏/抽屉,沉浸态工作台壳用它切竖向把手/上下折叠条 ——
 * 这两处是「宽窄两套机制」而非同一套的宽度差,CSS 表达不了,故读一次媒体查询。
 * 本项目为纯客户端 SPA,不做 SSR 分支。
 */
import { useSyncExternalStore } from "react";

export function useMediaQuery(query: string): boolean {
  return useSyncExternalStore(
    (callback) => {
      const mql = window.matchMedia(query);
      mql.addEventListener("change", callback);
      return () => mql.removeEventListener("change", callback);
    },
    () => window.matchMedia(query).matches,
  );
}
