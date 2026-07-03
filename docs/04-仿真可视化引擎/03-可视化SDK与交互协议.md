# M4 仿真可视化引擎 — 可视化 SDK 与交互协议

> 本文是仿真包作者的开发契约 SSOT:仿真包怎么写、SDK 怎么用、交互怎么声明。
> 面向第三方/教师扩展开发者。
> 最后更新:2026-05-29

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
  render: (state) => ViewSpec;               // 用 SDK 描述画面
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
- `render` 只能用 SDK 能力,不得自行操作 DOM。
- `interactions` 必须完整声明,运行时据此自动渲染控件。
- `sim-package.json` 只描述 `meta`、`interactions`、`render.patterns`、`narrative`、`codeTrace` 与 `checkpoints`;
  后端不执行其中任何函数,只校验封闭模式、交互事件、代码追踪配置和检查点锚点。

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
    "patterns": [
      { "mode": "graph", "region": "main", "config": { "layout": "force" } },
      { "mode": "chain", "region": "bottom" }
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
- `render.patterns` 必须是 1~3 个封闭模式,`mode` 只能取 `graph|chain|tree|matrix|pipeline|lane|chart`。
- `codeTrace` 使用 camelCase,与前端 TypeScript 协议一致;数据库字段 `code_trace` 只存不含源码正文的审核摘要。
- `checkpoints` 必须声明检查点 ID 与名称,受控预览和后续 `/sessions/{id}/checkpoints` 上报均以这些锚点派生结果。
- manifest JSON 拒绝未知字段和尾随内容,避免同一协议出现兼容别名或灰色扩展。

### 1.2 官方前端 SDK 使用入口

前端官方 SDK 包为 `@chaimir/sim-sdk`,开发者新增仿真包必须从公开 API 开始,不得复制内置包、Worker 或渲染器实现。主入口导出:

- `SimPackage`、`SimState`、`SimEvent`、`ViewSpec` 等协议类型。
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

### 3.2 label_tag 的视觉差异(机制统一)

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

### 4.2 模式映射(作者声明,非绘制)

```typescript
interface ViewSpec {
  patterns: PatternBinding[];   // 一个仿真组合 1~3 个模式
}
interface PatternBinding {
  mode: "graph"|"chain"|"tree"|"matrix"|"pipeline"|"lane"|"chart";
  region?: "main"|"side"|"bottom";   // 放主画布/侧栏/底部
  bind: (state) => PatternData;      // 把仿真状态映射为该模式的语义数据
  config?: object;                    // 模式参数(如 graph 的 layout:'ring')
}
```

作者只实现 `bind`(状态 → 语义数据),**绝不接触坐标/canvas/DOM**。

### 4.3 新增模式的闸门(防碎片化)

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
  render: (s) => ({ patterns: [
    { mode: "graph", region: "main", config: { layout: "force" },
      bind: st => ({ nodes: st.miners, edges: st.gossip }) },     // 矿工网络
    { mode: "chain", region: "bottom",
      bind: st => ({ blocks: st.chain, forks: st.forks, longest: st.mainChain }) }, // 链+分叉
  ]}),
  narrative: [
    { id: "s1", trigger: { at_tick: 0 }, explain: "正常情况下最长链由诚实算力主导" },
    { id: "s2", trigger: { on_event: "launch-51" },
      question: { prompt: "攻击者算力超50%后,双花能否成功?", options: ["能","不能"], answer: "能",
                  checkpoint_id: "cp-51-success" } },
  ],
};
```
