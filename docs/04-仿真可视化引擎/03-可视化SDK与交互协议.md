# M4 仿真可视化引擎 — 可视化 SDK 与交互协议

> 本文是仿真包作者的开发契约 SSOT:仿真包怎么写、SDK 怎么用、交互怎么声明。
> 面向第三方/教师扩展开发者。
> 最后更新:2026-07-06

---

## 1. 仿真包总览

仿真包归档根目录必须包含 `sim-package.json`。若归档工具自动包了一层顶级目录,允许
`<top-level>/sim-package.json`;其他位置的同名文件不作为协议入口。后端上传审核只读取这
一个 manifest 生成交互白名单和审核摘要,运行时代码仍随 bundle 保存在对象存储。

```typescript
interface SimPackage {
  meta: SimMeta;
  initState: (params, seed) => State;        // 构造初始状态
  reducer: (state, event, tick) => State;    // 纯函数,确定性演化
  interactions: InteractionDef[];            // 声明式交互
  render: (state) => TeachingFrame;          // 声明当前教学画面
  narrative: NarrativeStep[];                // 教学叙事
  codeTrace: CodeTraceDef;                   // 代码追踪配置
  checkpoints: CheckpointDef[];              // 判题检查点
}

interface SimMeta {
  code: string;            // 唯一标识(命名空间前缀防冲突)
  name: string;
  category: string;        // 领域分类(仅用于检索/配色)
  version: string;         // semver
  compute: "frontend" | "backend";  // 默认 frontend
  scaleLimit: { nodes: number; maxTick: number; maxEvents: number };  // 性能边界
}
```

**硬约束**:
- `reducer` 必须纯函数:`reducer(s,e,t)` 同输入必同输出,禁用 `Date.now()`/`Math.random()`(随机走种子 PRNG)。
- `render` 只能返回 `TeachingFrame` 纯数据,不得自行操作 DOM、Canvas、网络或浏览器全局状态。
- `interactions` 必须完整声明,运行时据此自动渲染控件。
- `sim-package.json` 只描述 `meta`、`interactions`、`render.protocol`、`render.patterns`、`narrative`、`codeTrace` 与 `checkpoints`;
  后端不执行其中任何函数,只校验 TeachingFrame 协议版本、封闭模式、交互事件、代码追踪配置和检查点锚点。

### 1.1 `sim-package.json` 协议入口

```json
{
  "meta": {
    "code": "builtin__pow-mining",
    "name": "PoW 挖矿与51%攻击",
    "category": "consensus",
    "version": "1.0.0",
    "compute": "frontend",
    "scale_limit": { "nodes": 50, "max_tick": 5000, "max_events": 10000 }
  },
  "interactions": [
    {
      "id": "attack51",
      "kind": "button",
      "label": "发起51%攻击",
      "emits": "launch-51",
      "label_tag": "attack",
      "params": [{ "name": "blocks", "type": "number", "default": 6 }]
    }
  ],
  "render": {
    "protocol": "teaching-frame",
    "patterns": [
      { "id": "pow-network", "mode": "graph", "roles": ["primary", "evidence"] },
      { "id": "pow-chain", "mode": "chain", "roles": ["primary", "timeline"] }
    ]
  },
  "narrative": [
    { "id": "normal", "title": "观察挖矿传播", "highlight": ["pow-graph"], "explain": "先观察诚实节点如何扩展链。", "defaultDurationMs": 1200 }
  ],
  "codeTrace": {
    "sourceCode": "function mineStep(state) {\n  return validateAndAppend(state);\n}",
    "language": "pseudocode",
    "lineMapping": [
      { "line": 1, "triggerCondition": "tick", "annotation": "进入挖矿步骤" },
      { "line": 2, "triggerCondition": "append", "annotation": "校验并追加区块", "highlightStyle": "success" }
    ],
    "variableWatch": [{ "name": "height", "extract": "state.height", "format": "number" }]
  },
  "checkpoints": [{ "id": "cp-51-success", "label": "识别 51% 攻击结果" }]
}
```

后端校验规则:
- `meta` 必须与上传表单一致,防止 bundle 自描述与入库元数据分裂。
- `interactions[].emits` 生成 `sim_package.interaction_schema`,运行时只接受 manifest 声明过的事件和参数。
- `render.protocol` 必须是 `teaching-frame`。
- `render.patterns` 必须是 1~3 个封闭模式声明,每项必须有稳定 `id` 和 `mode`,`mode` 只能取 `graph|chain|tree|matrix|pipeline|lane|chart`。
- `render.patterns[].roles` 只能用于审核该模式可承担的教学区域,取值为 `primary|evidence|timeline|metrics|trace|checkpoints`,不得再使用旧版区域字段。
- `codeTrace` 使用 camelCase,与前端 TypeScript 协议一致;数据库字段 `code_trace` 只存不含源码正文的审核摘要。
- `checkpoints` 必须声明检查点 ID 与名称,受控预览和后续 `/sessions/{id}/checkpoints` 上报均以这些锚点派生结果。
- manifest JSON 拒绝未知字段和尾随内容,避免同一协议出现兼容别名或灰色扩展。

### 1.2 官方前端 SDK 使用入口

前端官方 SDK 包为 `@chaimir/sim-sdk`,开发者新增仿真包必须从公开 API 开始,不得复制内置包、Worker 或渲染器实现。主入口导出:

- `SimPackage`、`SimState`、`SimEvent`、`TeachingFrame`、`VisualPattern` 等协议类型。
- `defineSimPackage(simPackage)`:定义仿真包并执行开发期协议校验。
- `createDeveloperTemplate(code)`:生成最小完整模板。
- `validateSimPackageManifest(simPackage)`:上传前检查协议完整性。
- `createManifestSummary(simPackage)`:生成审核摘要。
- `createSimPackageManifest(simPackage)`:生成上传归档中的 `sim-package.json` 内容。
- `SimulationWorkbench`、`SimWorkerClient`、`PatternRenderer`:仅供平台页面装配,仿真包作者不应直接复刻运行时。

内置仿真包 registry 不从 `@chaimir/sim-sdk` 主入口导出。内置包由平台内部装配,第三方/教师包与内置包使用同一套 `SimPackage` 协议。

最小开发流程:

1. 使用 `createDeveloperTemplate(code)` 或 `defineSimPackage({...})` 创建包。
2. 完整实现 `meta`、`initState`、`reducer`、`interactions`、`render`、`narrative`、`codeTrace`、`checkpoints`。
3. 本地执行 `validateSimPackageManifest(simPackage)`,确认无协议问题。
4. 使用 `createSimPackageManifest(simPackage)` 生成 `sim-package.json`。
5. 将 `sim-package.json` 与 bundle 一起提交后端 M4 审核。

---

## 2. 状态与事件

```typescript
type State = Record<string, any>;   // 仿真自定义结构

interface SimEvent {
  type: string;            // 事件类型(与 InteractionDef.emits 对应)
  payload?: any;           // 参数值
  target?: string;         // 作用的元素 id(target=element 时)
  source: "tick" | "user"; // 来源
}
```

- tick 推进自动产生 `{type:"tick", source:"tick"}`。
- 用户交互产生 `{type, payload, target, source:"user"}`。
- reducer 统一消费,产出新状态。

---

## 3. 交互声明协议(核心)

```typescript
interface InteractionDef {
  id: string;
  kind: "button" | "slider" | "hold" | "select-element" | "drag" | "form";
  label: string;
  emits: string;                 // 触发后注入的事件 type
  params?: FieldDef[];           // 参数字段
  target?: "global" | "element"; // 默认 global
  element_filter?: string;       // target=element 时,可选元素类型过滤
  available_when?: Condition;    // 可用条件(状态/阶段表达式)
  label_tag?: "normal" | "perturb" | "attack";  // 仅视觉
  cooldown_ms?: number;
}

interface FieldDef {
  name: string;
  type: "number" | "string" | "boolean" | "select" | "range";
  default?: any;
  min?: number; max?: number; step?: number;   // number/range
  options?: { label: string; value: any }[];   // select
  required?: boolean;
}
```

### 3.1 kind → 控件映射(平台自动渲染)

| kind | 控件 | 行为 |
| --- | --- | --- |
| `button` | 按钮 | 点击 emit(params 为空)或展开表单后 emit |
| `slider` | 滑块 + 当前值 | 拖动实时 emit(配 range FieldDef) |
| `hold` | 按住按钮 | 按住期间持续 emit,松开停 |
| `select-element` | 画布选中模式 | 先点画布元素 → 再 emit(带 target) |
| `drag` | 拖拽手柄 | 拖拽产生 emit(含起止) |
| `form` | 字段表单 | 填字段 → 提交 emit |

### 3.2 平台保留 payload 字段

`params` 只声明仿真算法自定义参数。通用交互渲染器会为部分 `kind`
自动生成平台字段,这些字段不得在 `params` 中重复声明:

| 字段 | 来源 | 说明 |
| --- | --- | --- |
| `target` | `target:"element"` / `select-element` | 选中元素 ID,用于后端操作日志白名单校验;Worker 内部事件也保留顶层 `target` 供 reducer 使用。 |
| `active` | `hold` | 按住开始/持续为 `true`,释放为 `false`。 |
| `phase` | `drag` | 拖拽阶段:`start` / `move` / `end`。 |
| `startX` / `startY` | `drag` | 拖拽起点坐标。 |
| `currentX` / `currentY` | `drag` | 当前或结束坐标。 |
| `deltaX` / `deltaY` | `drag` | 相对起点的位移。 |

后端 `interaction_schema` 只接受 manifest `params` 与上述平台保留字段;
未声明字段一律拒绝。字段名必须满足操作日志 key 规则
`[A-Za-z][A-Za-z0-9_.:-]{0,63}`,不得使用下划线字段名。

### 3.3 label_tag 的视觉差异(机制统一)

| tag | 视觉 | 机制 |
| --- | --- | --- |
| normal | 常规色 | 普通 |
| perturb | 橙色 | 普通 |
| attack | 红色 + 二次确认弹窗 | 普通(仅多一步确认) |

> 攻击注入 = `label_tag:"attack"` 的普通交互,无任何特殊机制。仿真可声明任意攻击/操作。

---

## 4. 可视化模式(替代旧版图元 SDK)

> 详见《架构设计》§5。作者**不写渲染代码**,只把状态映射到平台提供的封闭模式集(7 种),由模式引擎负责布局/绘制/动画。两条红线:① 封闭模式集 + 评审闸门(无自定义渲染器后门);② 作者只给语义数据。

### 4.1 封闭模式集(7 种,平台维护)

| 模式 | key | 输入语义数据 | 引擎负责 |
| --- | --- | --- | --- |
| 图网络 | `graph` | nodes[]、edges/消息事件、layout(ring/force/grid) | 节点布局、消息飞线动画、状态着色 |
| 链式 | `chain` | blocks[]、分叉关系、最长链标记 | 块序列排布、分叉绘制、高亮 |
| 树形 | `tree` | 树节点、验证路径 | 树布局、路径高亮、构建动画 |
| 矩阵 | `matrix` | cells[][]、值/状态 | 网格、单元着色、变化闪烁 |
| 流水线 | `pipeline` | steps[]、当前步、数据值 | 分步流动、寄存器变化动画 |
| 时序泳道 | `lane` | stages[]、current、各方消息 | 阶段进度、消息时序 |
| 图表 | `chart` | 数据序列、类型(line/bar/pie) | 坐标轴、曲线、动态更新 |

### 4.2 TeachingFrame 映射(作者声明,非绘制)

```typescript
interface TeachingFrame {
  summary: string;
  phase: FramePhase;
  focus: FrameFocus;
  layout: FrameLayout;
  patterns: VisualPattern[];    // 一个教学画面组合 1~3 个封闭模式
  annotations?: FrameAnnotation[];
}

interface FramePhase {
  id: string;
  title: string;
  intent: "observe" | "compare" | "verify" | "debug" | "attack" | "recover" | "replay";
  explanation: {
    what: string;
    why: string;
    watch: string;
  };
}

interface FrameFocus {
  primary: string[];
  secondary?: string[];
  muted?: string[];
}

interface FrameLayout {
  primary: string;
  evidence?: string[];
  timeline?: string;
  metrics?: string[];
  trace?: string;
  checkpoints?: string[];
}

interface VisualPattern {
  id: string;
  mode: "graph"|"chain"|"tree"|"matrix"|"pipeline"|"lane"|"chart";
  title: string;
  data: PatternData;      // 把仿真状态映射为该模式的语义数据
}
```

作者只实现 `render(state)` 中的状态到教学语义映射,**绝不接触坐标/canvas/DOM**。`layout.primary` 必须指向 `patterns` 中存在的模式 ID;右侧证据、时间线、指标等也只能引用同一帧内的模式 ID。

### 4.3 元素生命周期与焦点

封闭模式内部的节点、边、区块、树节点、矩阵单元、流水线步骤、泳道消息都可以携带统一 `meta`:

```typescript
interface VisualElementMeta {
  id: string;
  label: string;
  role?: string;
  lifecycle: {
    state: "entering" | "active" | "settled" | "leaving" | "archived";
    fromTick: number;
    toTick?: number;
  };
  emphasis: "focus" | "context" | "history" | "ghost";
  explanation?: string;
}
```

规则:

- 当前阶段关键元素必须出现在 `focus.primary` 或元素 `meta.emphasis="focus"` 中。
- 历史元素必须显式标为 `history` 或 `ghost`,由平台统一淡化、折叠或只保留摘要。
- 算法不得把无限增长的历史消息全部作为普通活跃边/消息输出。
- 选择、键盘焦点、读屏文本都以元素 `id/label/role` 为准。

### 4.4 新增模式的闸门(防碎片化)

- 仿真作者**不能自带渲染器**。现有 7 种模式无法表达的全新隐喻(罕见),提交**平台级模式评审**。
- 评审通过 → 新模式进入封闭集,成为**所有仿真的公共资产**,后续复用。
- 这是与旧版"L3 自定义渲染器敞开后门"的根本区别:扩展加固主干,而非分裂枝杈。

---

## 5. 教学叙事协议

```typescript
interface NarrativeStep {
  id: string;
  trigger: { at_tick?: number; on_state?: Condition; on_event?: string };
  highlight?: string[];      // 高亮状态元素 id
  explain?: string;          // 解说"为什么"
  question?: {
    prompt: string;
    options?: string[];      // 选择题;无则开放预测
    answer?: any;            // 揭晓答案(供 M3 检查点)
    checkpoint_id?: string;  // M3 判分锚点
  };
}
```

- 叙事按 trigger 推进;`question` 结果可上报 M3 作为仿真检查点判分。
- 叙事与仿真解耦,同仿真可挂多套叙事。

---

## 6. 无障碍与响应式约束

- 所有交互控件键盘可达(Tab/Enter/方向键),焦点可见。
- 画布提供文本化状态摘要(供读屏与无法看动画时理解)。
- 布局响应式:复合组件自适应容器尺寸,低层绘制用逻辑坐标缩放。
- 颜色不作唯一信息载体(配图标/文字),保证色弱可用。

---

## 7. 仿真包示例(PoW + 51% 攻击,节选)

```typescript
const PoWSim: SimPackage = {
  meta: { code: "builtin__pow-mining", name: "PoW 挖矿与51%攻击",
          category: "consensus", version: "1.0.0", compute: "frontend",
          scaleLimit: { nodes: 50, maxTick: 5000, maxEvents: 10000 } },
  initState: (p, seed) => ({ miners: mkMiners(p.minerCount, seed), chain: [genesis], ... }),
  reducer: (s, e, t) => {
    switch (e.type) {
      case "tick": return mineStep(s);
      case "set-hashrate": return setHashrate(s, e.target, e.payload.value);
      case "launch-51": return forkAttack(s, e.payload);
      ...
    }
  },
  interactions: [
    { id: "hashrate", kind: "slider", label: "调矿工算力", emits: "set-hashrate",
      target: "element", element_filter: "miner",
      params: [{ name: "value", type: "range", min: 0, max: 100, default: 30 }] },
    { id: "attack51", kind: "button", label: "发起51%攻击", emits: "launch-51",
      label_tag: "attack",
      params: [{ name: "blocks", type: "number", default: 6 }] },
  ],
  render: (s) => ({
    summary: `当前高度 ${s.height},主链候选 ${s.mainChainTip}`,
    phase: {
      id: s.attackActive ? "fork-race" : "honest-mining",
      title: s.attackActive ? "分叉竞争" : "诚实出块",
      intent: s.attackActive ? "attack" : "observe",
      explanation: {
        what: s.attackActive ? "攻击者私链正在追赶公开链。" : "矿工按算力竞争新区块。",
        why: "最长链选择决定节点最终接受哪条历史。",
        watch: "观察主舞台链尖是否被攻击者分叉赶上。"
      }
    },
    focus: {
      primary: [s.mainChainTip, s.attackerTip].filter(Boolean),
      secondary: s.miners.filter(m => m.active).map(m => m.id),
      muted: s.gossip.filter(msg => msg.settled).map(msg => msg.id)
    },
    layout: {
      primary: "pow-chain",
      evidence: ["pow-network"],
      metrics: ["pow-hashrate"]
    },
    patterns: [
      { id: "pow-chain", mode: "chain", title: "主链与攻击分叉",
        data: { blocks: s.chain, forks: s.forks, canonicalTip: s.mainChainTip } },
      { id: "pow-network", mode: "graph", title: "矿工传播网络",
        data: { layout: "ring", nodes: s.miners, edges: s.gossip } },
      { id: "pow-hashrate", mode: "chart", title: "算力占比趋势",
        data: { series: s.hashrateSeries, unit: "%" } }
    ]
  }),
  narrative: [
    { id: "s1", trigger: { at_tick: 0 }, explain: "正常情况下最长链由诚实算力主导" },
    { id: "s2", trigger: { on_event: "launch-51" },
      question: { prompt: "攻击者算力超50%后,双花能否成功?", options: ["能","不能"], answer: "能",
                  checkpoint_id: "cp-51-success" } },
  ],
};
```
