/**
 * useReducedMotion:响应系统「减少动态」偏好(FE-2/§3.2)。
 * JS 驱动的动效(计数滚动、WAAPI、编排)在此为 true 时必须退化为直接呈现。
 * 本项目为纯客户端 SPA,不做 SSR 分支。
 */
import { useSyncExternalStore } from "react";

const QUERY = "(prefers-reduced-motion: reduce)";

function subscribe(callback: () => void): () => void {
  const mql = window.matchMedia(QUERY);
  mql.addEventListener("change", callback);
  return () => mql.removeEventListener("change", callback);
}

function getSnapshot(): boolean {
  return window.matchMedia(QUERY).matches;
}

export function useReducedMotion(): boolean {
  return useSyncExternalStore(subscribe, getSnapshot);
}
