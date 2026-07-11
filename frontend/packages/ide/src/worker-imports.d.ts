// worker-imports 声明 Vite Worker 构造器模块,供 Monaco 独立 Worker 分块使用。

declare module '*?worker' {
  const WorkerConstructor: {
    new (): Worker
  }

  export default WorkerConstructor
}
