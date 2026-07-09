// vite-env.d.ts 声明 Vite 与 CSS Module 类型,供应用层 TypeScript 编译识别样式导入。
/// <reference types="vite/client" />

declare module '*.module.css' {
  const classes: Record<string, string>
  export default classes
}
