/**
 * useReducedMotion:响应系统「减少动态」偏好(FE-2/§3.2)。
 * JS 驱动的动效(计数滚动、WAAPI、编排)在此为 true 时必须退化为直接呈现。
 * 本项目为纯客户端 SPA,不做 SSR 分支;但仍声明 getServerSnapshot ——
 * `useSyncExternalStore` 少了它会在任何非浏览器渲染器下直接抛错,
 * 令依赖本钩子的组件无法在浏览器之外被渲染验证。服务端快照取 false(不减少动态),
 * 客户端接手后由真实 matchMedia 覆盖。
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

/** 非浏览器环境的快照:无从得知偏好,按「不减少动态」处理,挂载后立即被真实值替换 */
function getServerSnapshot(): boolean {
  return false;
}

export function useReducedMotion(): boolean {
  return useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
}
