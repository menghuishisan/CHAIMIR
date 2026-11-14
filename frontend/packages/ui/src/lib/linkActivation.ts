/**
 * linkActivation:整行、整卡、列表行链接的点击判定(规范 §6.5.0 通则 4)。
 * 普通左键在本页跳转(由调用方交给路由,不整页刷新);Ctrl/⌘/Shift 加左键与中键在新标签页打开;
 * Alt 加左键(浏览器的「下载链接」)与右键不处理,交给浏览器。
 * 本文件供 node:test 直接运行,不依赖 React 与 DOM。
 */

/** 一次指针点击里与链接判定有关的字段(MouseEvent 的子集)。 */
export interface PointerActivation {
  button: number;
  ctrlKey: boolean;
  metaKey: boolean;
  shiftKey: boolean;
  altKey: boolean;
}

/** 点击的去向:本页跳转 / 新标签页打开 / 不处理。 */
export type LinkActivation = "same-tab" | "new-tab" | "ignore";

/** 主键(左键)与中键的 MouseEvent.button 取值 */
const PRIMARY_BUTTON = 0;
const MIDDLE_BUTTON = 1;

/** linkActivation 判断一次点击该如何打开链接。 */
export function linkActivation(event: PointerActivation): LinkActivation {
  if (event.button === MIDDLE_BUTTON) return "new-tab";
  if (event.button !== PRIMARY_BUTTON) return "ignore";
  if (event.altKey) return "ignore";
  if (event.ctrlKey || event.metaKey || event.shiftKey) return "new-tab";
  return "same-tab";
}
