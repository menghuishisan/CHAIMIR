// linkActivation.test.ts 验证整行、整卡链接的点击判定:普通左键在本页跳转,修饰键与中键在新标签页打开,其余不处理。
import { test } from "node:test";
import assert from "node:assert/strict";
import { linkActivation } from "./linkActivation.ts";

const plain = { button: 0, ctrlKey: false, metaKey: false, shiftKey: false, altKey: false };

test("普通左键在本页跳转", () => {
  assert.equal(linkActivation(plain), "same-tab");
});

test("Ctrl 或 ⌘ 加左键、Shift 加左键、中键都在新标签页打开", () => {
  assert.equal(linkActivation({ ...plain, ctrlKey: true }), "new-tab");
  assert.equal(linkActivation({ ...plain, metaKey: true }), "new-tab");
  assert.equal(linkActivation({ ...plain, shiftKey: true }), "new-tab");
  assert.equal(linkActivation({ ...plain, button: 1 }), "new-tab");
});

test("Alt 加左键与右键不处理,交给浏览器", () => {
  assert.equal(linkActivation({ ...plain, altKey: true }), "ignore");
  assert.equal(linkActivation({ ...plain, button: 2 }), "ignore");
});
