# @chaimir/sim-sdk 开发者指南

本包是 M4 仿真可视化引擎的官方前端 SDK。第三方或教师扩展仿真包时,只能通过这里导出的协议类型、authoring 工具和封闭可视化模式编写状态机,不能复制内置包、Worker 或渲染器实现。

## 公开能力

- `SimPackage`、`SimState`、`SimEvent`、`ViewSpec` 等协议类型。
- `defineSimPackage` 用于定义并校验仿真包。
- `createDeveloperTemplate` 用于生成最小完整示例。
- `createSimPackageManifest` 用于生成上传审核使用的 `sim-package.json` 内容。
- `createManifestSummary` 和 `validateSimPackageManifest` 用于本地检查协议字段。
- `SimWorkerClient`、`SimulationWorkbench`、`PatternRenderer` 供平台页面装配使用。

内置仿真包 registry 不从主入口导出。内置包只由平台内部装配,开发者新增仿真应使用同一套公开协议,不依赖内置实现。

## 最小仿真包

```typescript
import {
  defineSimPackage,
  createSimPackageManifest,
  type SimPackage,
  type SimState,
} from '@chaimir/sim-sdk';

interface DemoState extends SimState {
  phaseIndex: number;
}

export const simPackage: SimPackage<DemoState> = defineSimPackage({
  meta: {
    code: 'teacher_1001__demo-pipeline',
    name: '演示流水线',
    category: 'consensus',
    version: '1.0.0',
    compute: 'frontend',
    summary: '演示如何按 M4 协议实现确定性仿真包。',
    learningObjectives: ['理解 initState/reducer/render 的职责'],
    scaleLimit: { nodes: 16, maxTick: 120, maxEvents: 240 },
  },
  initState: () => ({
    tick: 0,
    phase: '准备',
    phaseIndex: 0,
    explanation: {
      title: '准备',
      effect: '初始化教学状态。',
      reason: '所有仿真都必须可由 seed 和事件日志重放。',
      defaultDurationMs: 1200,
    },
    metrics: { progress: 0 },
    checkpointValues: { done: false },
    _trace: { triggeredLines: [1], variables: { progress: 0 } },
  }),
  reducer: (state, event) => {
    const done = event.type === 'advance' || event.type === 'tick';
    return {
      ...state,
      tick: event.source === 'tick' ? state.tick + 1 : state.tick,
      phase: done ? '完成' : state.phase,
      phaseIndex: done ? 1 : state.phaseIndex,
      metrics: { progress: done ? 100 : 0 },
      checkpointValues: { done },
      _trace: { triggeredLines: done ? [2] : [1], variables: { progress: done ? 100 : 0 } },
    };
  },
  interactions: [
    {
      id: 'advance',
      kind: 'button',
      label: '推进阶段',
      description: '推进一次仿真状态。',
      emits: 'advance',
      labelTag: 'normal',
    },
  ],
  render: (state) => ({
    summary: `当前阶段:${state.phase}`,
    patterns: [
      {
        id: 'demo-pipeline',
        mode: 'pipeline',
        title: '演示流程',
        region: 'main',
        data: {
          currentStepId: state.phaseIndex === 1 ? 'done' : 'ready',
          steps: [
            { id: 'ready', label: '准备', status: state.phaseIndex === 1 ? 'complete' : 'running', detail: '初始化状态' },
            { id: 'done', label: '完成', status: state.phaseIndex === 1 ? 'complete' : 'pending', detail: '状态已推进' },
          ],
        },
      },
    ],
  }),
  narrative: [
    {
      id: 'ready',
      title: '观察流程',
      trigger: () => true,
      highlight: ['demo-pipeline'],
      explain: '观察 reducer 如何把事件转换为下一份状态。',
      defaultDurationMs: 1200,
    },
  ],
  codeTrace: {
    sourceCode: ['function reducer(state, event) {', '  return nextState;', '}'].join('\n'),
    language: 'pseudocode',
    lineMapping: [
      { line: 1, triggerCondition: 'init', annotation: '进入 reducer' },
      { line: 2, triggerCondition: 'advance', annotation: '生成下一份状态' },
    ],
    variableWatch: [{ name: 'progress', extract: 'state._trace.variables.progress', format: 'number' }],
  },
  checkpoints: [
    {
      id: 'demo-done',
      label: '完成演示流程',
      evaluate: (state) => ({
        achieved: state.checkpointValues.done === true,
        answer: state.checkpointValues.done,
        explanation: state.checkpointValues.done === true ? '流程已完成。' : '流程尚未完成。',
      }),
    },
  ],
});

export const manifest = createSimPackageManifest(simPackage);
```

`createSimPackageManifest` 输出的是后端 M4 上传审核读取的 `sim-package.json`:其中 `meta` 只包含 `code`、`name`、`category`、`version`、`compute`、`scale_limit`,交互参数不包含前端控件文案,`codeTrace` 保留源码和行映射供后端生成不含源码正文的审核摘要。

## 开发流程

1. 用 `createDeveloperTemplate(code)` 生成最小包,或按上面的结构从 `defineSimPackage` 开始。
2. 只实现 `initState`、`reducer`、`render`、`narrative`、`codeTrace`、`checkpoints`。
3. `reducer` 必须是纯函数,不能使用真实时间、网络、DOM、全局随机源或外部副作用。
4. `render` 只能返回 `graph`、`chain`、`tree`、`matrix`、`pipeline`、`lane`、`chart` 中的 1 到 3 个模式。
5. 运行 `validateSimPackageManifest(simPackage)` 检查协议字段。
6. 用 `createSimPackageManifest(simPackage)` 生成 `sim-package.json` 内容,和 bundle 一起提交审核。

## 运行边界

- 仿真包代码只在 Worker 中执行。
- 主线程只接收 `RuntimeSnapshot` 纯数据。
- 交互控件由 `interactions` 自动生成。
- 代码追踪由 `state._trace.triggeredLines` 和 `state._trace.variables` 驱动。
- 检查点只读状态派生结果,不能向外部服务写入数据。

## 本地校验

```bash
pnpm --filter @chaimir/sim-sdk build
pnpm --filter @chaimir/sim-sdk lint
```
