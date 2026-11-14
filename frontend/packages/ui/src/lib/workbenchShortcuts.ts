/**
 * workbenchShortcuts:沉浸工作台快捷键的匹配、焦点归类、显示文字与登记处
 * (设计 §7.1:顶栏「快捷键」列出当前工作台的全部快捷键)。
 * 快捷键由工作台里的各部分声明(顶栏主操作、播放控制、代码编辑区),都登记到壳持有的同一个登记处:
 * 壳按它监听按键,顶栏按它列出,所以「列出的」与「生效的」是同一份声明。
 * 单键先让给焦点所在的控件:输入框与代码编辑器里一律不抢;按钮上不抢空格与回车;
 * 标签页、单选组、滑块上连方向键也不抢。带 Ctrl/⌘ 的组合在这些地方照样生效(如「Ctrl + S 写入当前文件」)。
 * 终端与弹窗、菜单、下拉独占全部按键:终端里 Ctrl 组合本身就是 shell 的命令,浮层打开时动作应留在浮层里。
 * 本文件供 node:test 直接运行,不依赖 React 与 DOM。
 */

/** 一个快捷键:键名与修饰键(mod 在 Windows 是 Ctrl,在 Mac 是 ⌘) */
export interface ShortcutKey {
  /** KeyboardEvent.key,如 "Enter"、"ArrowLeft"、"s"、" " */
  key: string;
  mod?: boolean;
  shift?: boolean;
  /** 按住不放时连续执行(如逐局回溯);默认只在按下时执行一次 */
  repeat?: boolean;
}

/** 工作台的一个快捷键:键位、说明与动作。disabled 的项照样列出,按下时不执行 */
export interface WorkbenchShortcut extends ShortcutKey {
  description: string;
  run: () => void;
  disabled?: boolean;
}

/**
 * 焦点所在位置对按键的占用:
 * none 不占用;control 按钮、链接等占用空格与回车;composite 标签页、单选组、滑块等再加上方向键与翻页键;
 * editable 输入框与代码编辑器占用全部单键;exclusive 终端与弹窗、浮层占用全部按键,连组合键也不让。
 */
export type ShortcutFocus = "none" | "control" | "composite" | "editable" | "exclusive";

/** 匹配用到的按键事件字段 */
export interface ShortcutKeyEvent {
  key: string;
  ctrlKey: boolean;
  metaKey: boolean;
  shiftKey: boolean;
  altKey: boolean;
  /** 按住不放产生的重复事件;是否执行由 resolveShortcut 决定,匹配不看它 */
  repeat: boolean;
  focus: ShortcutFocus;
}

/** 焦点元素的特征:壳从 DOM 读出,归类规则放在这里以便单独验证 */
export interface ShortcutTarget {
  /** 元素标签名(HTML 为大写,SVG 为小写) */
  tag: string;
  /** input 的 type */
  type?: string;
  role?: string;
  contentEditable: boolean;
  /** 在代码编辑器里 */
  inEditor: boolean;
  /** 在终端里 */
  inTerminal: boolean;
  /** 在弹窗、抽屉或菜单、下拉、气泡等浮层里 */
  inOverlay: boolean;
}

/** 按钮类控件自己要用的激活键 */
const ACTIVATION_KEYS = [" ", "Enter"];

/** 组合控件在内部移动用的键 */
const NAVIGATION_KEYS = ["ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown", "Home", "End", "PageUp", "PageDown"];

const EDITABLE_ROLES = ["textbox", "searchbox", "combobox"];
const COMPOSITE_ROLES = [
  "tab",
  "tablist",
  "radio",
  "radiogroup",
  "slider",
  "spinbutton",
  "listbox",
  "option",
  "menu",
  "menubar",
  "menuitem",
  "menuitemradio",
  "menuitemcheckbox",
  "grid",
  "gridcell",
  "tree",
  "treeitem",
];
const CONTROL_ROLES = ["button", "link", "checkbox", "switch"];
const COMPOSITE_INPUT_TYPES = ["range", "radio"];
const CONTROL_INPUT_TYPES = ["checkbox", "button", "submit", "reset", "file", "color", "image"];

/** 特殊键的显示名 */
const KEY_LABELS: Record<string, string> = {
  ArrowLeft: "←",
  ArrowRight: "→",
  ArrowUp: "↑",
  ArrowDown: "↓",
  " ": "空格",
  Escape: "Esc",
  Enter: "Enter",
};

/** classifyShortcutFocus 判断焦点所在的元素会占用哪些按键 */
export function classifyShortcutFocus(target: ShortcutTarget): ShortcutFocus {
  if (target.inOverlay || target.inTerminal) return "exclusive";
  if (target.inEditor || target.contentEditable) return "editable";
  const tag = target.tag.toUpperCase();
  if (tag === "TEXTAREA" || tag === "SELECT") return "editable";
  if (tag === "INPUT") {
    const type = (target.type ?? "text").toLowerCase();
    if (COMPOSITE_INPUT_TYPES.includes(type)) return "composite";
    if (CONTROL_INPUT_TYPES.includes(type)) return "control";
    return "editable";
  }
  const role = target.role?.toLowerCase();
  if (role !== undefined) {
    if (EDITABLE_ROLES.includes(role)) return "editable";
    if (COMPOSITE_ROLES.includes(role)) return "composite";
    if (CONTROL_ROLES.includes(role)) return "control";
  }
  if (tag === "BUTTON" || tag === "A" || tag === "SUMMARY") return "control";
  return "none";
}

/** focusOwnsKey 判断焦点所在的控件是否自己要用这个单键 */
function focusOwnsKey(focus: ShortcutFocus, key: string): boolean {
  if (focus === "editable") return true;
  if (focus === "none") return false;
  if (ACTIVATION_KEYS.includes(key)) return true;
  return focus === "composite" && NAVIGATION_KEYS.includes(key);
}

/** matchShortcut 判断一次按键是否就是这个快捷键(修饰键精确匹配,单键先让给焦点所在的控件) */
export function matchShortcut(shortcut: ShortcutKey, event: ShortcutKeyEvent): boolean {
  if (event.focus === "exclusive") return false;
  const wantsMod = shortcut.mod === true;
  if (wantsMod !== (event.ctrlKey || event.metaKey) || event.altKey) return false;
  if ((shortcut.shift === true) !== event.shiftKey) return false;
  if (event.key.toLowerCase() !== shortcut.key.toLowerCase()) return false;
  return wantsMod || !focusOwnsKey(event.focus, event.key);
}

/** 一次按键的处理结论:claimed 表示由工作台接管(拦下浏览器默认行为),run 是要执行的动作 */
export interface ShortcutResolution {
  claimed: boolean;
  run?: () => void;
}

/**
 * resolveShortcut 决定一次按键交给谁。
 * 命中即接管 —— 哪怕暂时不可用或是按住产生的重复,也不让 Ctrl/⌘ + S 落到浏览器的「另存网页」;
 * 执行的是第一个可用的命中项,按住不放时只执行声明了可连续执行的那些(推进、判分、提交只在按下时执行一次)。
 */
export function resolveShortcut(
  shortcuts: readonly WorkbenchShortcut[],
  event: ShortcutKeyEvent,
): ShortcutResolution {
  const hits = shortcuts.filter((item) => matchShortcut(item, event));
  const runnable = hits.find((item) => !item.disabled && (!event.repeat || item.repeat === true));
  return { claimed: hits.length > 0, run: runnable?.run };
}

/** shortcutLabel 快捷键的显示文字,如「Ctrl + Enter」「⌘ + Enter」「←」 */
export function shortcutLabel(shortcut: ShortcutKey, mac: boolean): string {
  const parts: string[] = [];
  if (shortcut.mod) parts.push(mac ? "⌘" : "Ctrl");
  if (shortcut.shift) parts.push("Shift");
  parts.push(KEY_LABELS[shortcut.key] ?? (shortcut.key.length === 1 ? shortcut.key.toUpperCase() : shortcut.key));
  return parts.join(" + ");
}

/** 快捷键登记处:工作台各部分按自己的编号登记,壳汇总后监听与列出 */
export interface ShortcutRegistry {
  /** set 写入(或替换)某一处的声明;替换时保留它在列表里的位置 */
  set(owner: string, shortcuts: readonly WorkbenchShortcut[]): void;
  remove(owner: string): void;
  /** all 当前全部声明,动作是最新的(按键时用) */
  all(): WorkbenchShortcut[];
  /** snapshot 供列表渲染的快照:只在键位、说明或可用性变化时换新,只换动作函数不换 */
  snapshot(): readonly WorkbenchShortcut[];
  subscribe(listener: () => void): () => void;
}

/** listingSignature 列表展示关心的字段;动作函数每次渲染都是新的,不算变化 */
function listingSignature(shortcuts: readonly WorkbenchShortcut[]): string {
  return JSON.stringify(
    shortcuts.map((item) => [item.key, item.mod === true, item.shift === true, item.description, item.disabled === true]),
  );
}

/** createShortcutRegistry 创建一个工作台的快捷键登记处 */
export function createShortcutRegistry(): ShortcutRegistry {
  const owners = new Map<string, readonly WorkbenchShortcut[]>();
  const listeners = new Set<() => void>();
  let listed: readonly WorkbenchShortcut[] = [];
  let signature = "";

  const all = () => [...owners.values()].flat();

  /** refresh 列表展示的内容变了才换快照并通知订阅方 */
  const refresh = () => {
    const current = all();
    const next = listingSignature(current);
    if (next === signature) return;
    signature = next;
    listed = current;
    listeners.forEach((listener) => listener());
  };

  return {
    set(owner, shortcuts) {
      owners.set(owner, shortcuts);
      refresh();
    },
    remove(owner) {
      if (owners.delete(owner)) refresh();
    },
    all,
    snapshot: () => listed,
    subscribe(listener) {
      listeners.add(listener);
      return () => {
        listeners.delete(listener);
      };
    },
  };
}
