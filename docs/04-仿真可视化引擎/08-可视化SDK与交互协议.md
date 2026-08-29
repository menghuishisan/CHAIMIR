# M4 仿真可视化引擎 — 可视化 SDK 与交互协议

> 本文是仿真包作者的开发契约 SSOT:仿真包怎么写、SDK 怎么用、交互怎么声明。
> 面向第三方/教师扩展开发者。
> 最后更新:2026-08-25

---

## 1. 仿真包总览

仿真包归档根目录必须包含 `sim-package.json`。若归档工具自动包了一层顶级目录,允许
`<top-level>/sim-package.json`;其他位置的同名文件不作为协议入口。后端上传审核只读取这
一个 manifest 生成交互白名单和审核摘要,运行时代码仍随 bundle 保存在对象存储。

**扩展包在后端隔离容器内执行**(见 07-架构设计.md §8):归档字节不下发浏览器,而是在会话建立时
经 k8s exec stdin 推入受限计算 Pod,容器内校验 sha256 后按 `meta.entry` 装配。
对作者而言执行位置是透明的 —— 下述协议与内置包完全一致,不为"跑在哪"写第二套实现。

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
  code: string;            // 唯一标识(命名空间前缀防冲突,由服务端按登录账号强制)
  name: string;
  category: string;        // 领域分类(仅用于检索/配色)
  version: string;         // semver
  entry: string;           // 归档内入口模块相对路径,供隔离容器装配(扩展包必填)
  scaleLimit: { nodes: number; maxTick: number; maxEvents: number };  // 性能边界
}
```

`compute` 不由作者声明:执行位置按代码来源派生(内置=浏览器、教师/第三方=隔离容器),
提交表单与 manifest 都不含该字段。作者写一次协议,平台决定在哪跑。

**硬约束**:
- `reducer` 必须纯函数:`reducer(s,e,t)` 同输入必同输出,禁用 `Date.now()`/`Math.random()`(随机走种子 PRNG)。
  隔离预览会同 seed 跑两遍逐帧比对,不纯即拒绝上架。
- `render` 只能返回 `TeachingFrame` 纯数据,不得自行操作 DOM、Canvas、网络或浏览器全局状态。
  容器内无 DOM、无网络出口,这些调用会直接失败。
- `interactions` 必须完整声明,运行时据此自动渲染控件并作为服务端事件白名单。
- `sim-package.json` 只描述 `meta`、`interactions`、`render.protocol`、`render.patterns`、`narrative`、`codeTrace` 与 `checkpoints`;
  后端不执行其中任何函数,只校验 TeachingFrame 协议版本、封闭模式、交互事件、代码追踪配置和检查点锚点。

### 1.1 `sim-package.json` 协议入口

```json
{
  "meta": {
    "code": "teacher_1001__pow-mining",
    "name": "PoW 挖矿与51%攻击",
    "category": "consensus",
    "version": "1.0.0",
    "entry": "dist/index.mjs",
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
- `meta` 必须与上传表单一致(`code`/`version`/`name`/`category`/`scale_limit`),防止 bundle 自描述与入库元数据分裂。
- `meta.entry` 必须是归档内真实存在的 `.mjs` 文件相对路径;不得为绝对路径、不得含 `..`、不得指向归档外。
  只接受 `.mjs`:归档内没有 `package.json` 时 Node 按 CJS 解析 `.js`,而扩展包必须默认导出 `SimPackage`,
  允许 `.js` 会让"能不能装配"取决于打包方式。缺失或非法即拒绝(`41002`)—— 容器需要它才能装配,没有它包就是不可运行的。
- `interactions[].emits` 生成 `sim_package.interaction_schema`,运行时只接受 manifest 声明过的事件和参数。
- `interactions[].label_tag` 只接受 `normal|recover|perturb|attack`(可缺省,缺省按 `normal` 处理),未知值即拒绝上架:
  标签决定按钮配色与攻击类的就地二次确认(§3.3),认不出的标签会让扩展包的破坏性操作画成普通推进色。
- `render.protocol` 必须是 `teaching-frame`。
- `render.patterns` 必须是 1~3 个封闭模式声明,每项必须有稳定 `id` 和 `mode`,`mode` 只能取 `graph|chain|tree|matrix|pipeline|lane|chart`。
- `render.patterns[].roles` 只能用于审核该模式可承担的教学区域,取值为 `primary|evidence|timeline|metrics|trace|checkpoints`,不得再使用旧版区域字段。
- `codeTrace` 使用 camelCase,与前端 TypeScript 协议一致;数据库字段 `code_trace` 只存不含源码正文的审核摘要。
- `checkpoints` 必须声明检查点 ID 与名称,隔离预览和后续 `/sessions/{id}/checkpoints` 上报均以这些锚点派生结果。
- manifest JSON 拒绝未知字段和尾随内容,避免同一协议出现兼容别名或灰色扩展。

### 1.2 官方前端 SDK 使用入口

前端官方 SDK 包为 `@chaimir/sim-sdk`,开发者新增仿真包必须从公开 API 开始,不得复制内置包、Worker 或渲染器实现。主入口导出:

- `SimPackage`、`SimState`、`SimEvent`、`TeachingFrame`、`VisualPattern` 等协议类型。
- `defineSimPackage(simPackage)`:定义仿真包并执行开发期协议校验。
- `createDeveloperTemplate(code)`:生成最小完整模板。
- `validateSimPackage(simPackage)`:上传前检查协议完整性。
- `createManifestSummary(simPackage)`:生成审核摘要。
- `createSimPackageManifest(simPackage, entry)`:生成上传归档中的 `sim-package.json` 内容(`entry` 为归档内入口模块路径)。
- `SimWorkerClient`:仅供平台页面装配内置包 Worker 运行时,仿真包作者不应直接复刻运行时。

`@chaimir/sim-sdk` 不导出任何 React 视图组件(包内无 react 依赖)。`TeachingFrame` 的渲染由 `@chaimir/ui` 的 `biz/TeachingFrameStage` 负责,页面级装配在 `apps/web` 完成。扩展包在容器内运行时同样只产出 `TeachingFrame` 纯数据,由同一个 `TeachingFrameStage` 绘制 —— 全平台只有一套渲染实现。

内置仿真包 registry 不从 `@chaimir/sim-sdk` 主入口导出。内置包由平台内部装配,第三方/教师包与内置包使用同一套 `SimPackage` 协议。

最小开发流程:

1. 使用 `createDeveloperTemplate(code)` 或 `defineSimPackage({...})` 创建包。
2. 完整实现 `meta`、`initState`、`reducer`、`interactions`、`render`、`narrative`、`codeTrace`、`checkpoints`。
3. 本地执行 `validateSimPackage(simPackage)`,确认无协议问题。
4. 把包打成 ESM 模块(默认导出该 `SimPackage`),记下归档内相对路径作为 `meta.entry`。
5. 使用 `createSimPackageManifest(simPackage, entry)` 生成 `sim-package.json`。
6. 把 `sim-package.json` 与入口模块一起打成 ZIP/TAR 归档,提交后端 M4 审核。
   平台会在隔离容器内装配并同 seed 跑两遍比对确定性、渲出样例教学帧,再由平台管理员判定后上架。

---

### 1.3 `codeTrace` 代码追踪协议

仿真包可选声明 `codeTrace`,把"可视化现象 ↔ 代码逻辑"建立因果锚点(需求见 `01-需求规格.md` §1)。协议类型与 `SimPackage` 同一定义:

```typescript
interface CodeTraceDef {
  sourceCode: string;         // 源代码(Solidity/Rust/Go/JavaScript/伪代码)
  language: "solidity" | "rust" | "go" | "javascript" | "pseudocode";
  lineMapping: LineMapping[]; // 行号 → 状态条件映射
  variableWatch?: VariableWatchDef[];
}

interface LineMapping {
  line: number;               // 代码行号(1-based)
  triggerCondition: string;   // 触发条件表达式(对仿真状态的求值)
  annotation?: string;        // 该行的教学注释
  highlightStyle?: "normal" | "success" | "error";
}

interface VariableWatchDef {
  name: string;               // 变量名(展示用)
  extract: string;            // 从状态提取的表达式
  format?: "hex" | "number" | "string" | "bool";
}
```

`reducer` 返回的状态需携带 `_trace` 字段,运行时读取后在代码面板高亮对应行并展示变量监视:

```typescript
interface State {
  // ... 仿真自定义字段
  _trace?: {
    triggeredLines: number[];            // 当前触发的代码行
    variables: Record<string, any>;      // 当前变量值
    executionPath?: string;              // 可选:执行路径描述
  };
}
```

- 前端运行时自动读取 `state._trace`:高亮 `triggeredLines` 中的行、展示 `variables` 的值、按 `lineMapping` 显示教学注释。
- 设计边界(只允许单向映射):状态 → 代码行高亮、变量监视、教学注释;**禁止**代码行 → 状态的反向修改(这不是调试器)、禁止断点/单步执行、禁止修改源码后重新执行。
- `codeTrace` 仅声明教学追踪配置;后端 `Package` 只持久化审核摘要(`CodeTraceAudit`:语言、行数、映射数、变量数,字段定义见 `02-数据模型.md`),源码正文仍随 bundle 保存在对象存储,不得在领域模型上复制第二套传输结构。
- 代码追踪如何参与仿真会话与渲染,见 `05-业务流程与状态机.md` §9。

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
  label_tag?: "normal" | "recover" | "perturb" | "attack";  // 仅视觉(配色)
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
| normal | 玉实底(常规推进) | 普通 |
| recover | 玉实底 + 盾形图标(修复/防护) | 普通 |
| perturb | 描边 + 需注意色(扰动) | 普通 |
| attack | 朱砂实底 + 就地二次确认 | 普通(仅多一步确认) |

> 攻击注入 = `label_tag:"attack"` 的普通交互,无任何特殊机制。仿真可声明任意攻击/操作。
> `recover` 与 `attack` 必须分开:把防护动作画成攻击色是反向教学暗示(前端规范 §7.2 B)。
> 二次确认做在按钮上(点一次转为「确认攻击 / 取消」),不弹模态:攻击在仿真里是常规教学动作,
> 每次弹窗会打断「做一步 → 看结论」的观察节奏,而就地确认同样拦住了误触。

---

## 4. 可视化模式(替代旧版图元 SDK)

> 详见 07-架构设计.md §5。作者**不写渲染代码**,只把状态映射到平台提供的封闭模式集(7 种),由模式引擎负责布局/绘制/动画。两条红线:① 封闭模式集 + 评审闸门(无自定义渲染器后门);② 作者只给语义数据。

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
  // 只有两段(前端规范 §7.2 B):focus 已用舞台高亮指出该看哪个元素,
  // 再写一段「看哪里」只是复述状态摘要。
  explanation: {
    what: string;
    why: string;
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
  meta: { code: "teacher_1001__pow-mining", name: "PoW 挖矿与51%攻击",
          category: "consensus", version: "1.0.0", entry: "dist/index.mjs",
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
        why: "最长链选择决定节点最终接受哪条历史。"
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

---

## 8. 内置仿真包标准库

> 本文约束前端 `@chaimir/sim-sdk` 内置仿真包标准库。内置包必须和第三方/教师包使用同一套 M4 协议,不得绕过确定性回放规则、通用交互渲染器或封闭可视化模式。
> 内置包在浏览器 Worker 内运行(它就是随平台版本交付的平台代码),扩展包在后端隔离容器内运行同一协议 —— 执行位置按代码来源分流,见 07-架构设计.md §8。

### 8.1 分类口径

内置仿真包按教学主题分类,不是按可视化模式分类。`graph`、`chain`、`tree`、`matrix`、`pipeline`、`lane`、`chart` 只是每个仿真按教学需要组合使用的封闭表现形式。

| 分类 | 说明 |
| --- | --- |
| `consensus` | 共识算法与最终性,例如 PBFT、PoW、Raft、PoS、HotStuff。 |
| `cryptography` | 密码学概念,例如哈希、签名、Merkle 证明、零知识流程、门限签名。 |
| `network` | 网络传播与拓扑,例如 P2P、Gossip、DHT、分区、延迟丢包。 |
| `data-structure` | 链上数据结构,例如区块链、Merkle Tree、Patricia Trie、UTXO、状态快照。 |
| `contract-security` | 合约安全与攻防路径,例如重入、授权缺陷、预言机操纵、闪电贷。 |
| `transaction-runtime` | 交易与执行运行时,例如生命周期、Nonce、Gas、EVM 调用栈、区块验证。 |
| `cross-chain-system` | 跨链与系统机制,例如跨链消息、桥验证、多签委员会、最终性确认、重放防护。 |

### 8.2 完整性要求

内置标准库当前固定为 **7 类共 41 个** 仿真包。新增、替换或下架内置包时,必须同步更新本表、`frontend/packages/sim-sdk/src/builtin/catalog.ts` 和对应分类 `index.ts`,并重新通过 `@chaimir/sim-sdk` 构建、lint 与 41 包协议审查。

**改完必须重跑清单导出**:内置包要能在生产被检索到,得先进 `sim_package` 表。入库清单由
`node scripts/codegen/export-sim-builtin-catalog.mjs` 从本目录的 `package.ts` 求值导出为
`backend/internal/modules/sim/builtin_catalog.json`,后端 `go:embed` 后在 `migrate-and-seed`
阶段幂等入库(流程与三条硬约束见 05-业务流程与状态机.md §4.1)。产物已提交入库,
`scripts/audit/sim-catalog-drift.mjs` 校验它与源码一致 —— 漏跑脚本会让新包在生产取不到,
而 `interactions` 漂移会让学生的合法操作被后端白名单拒绝。

| 分类 | code | 名称 | 前端实现入口 |
| --- | --- | --- | --- |
| `consensus` | `builtin__pbft-consensus` | PBFT 三阶段共识推演 | `consensus/pbft/package.ts` |
| `consensus` | `builtin__pow-longest-chain` | PoW 最长链共识推演 | `consensus/pow/package.ts` |
| `consensus` | `builtin__raft-log-replication` | Raft 选举与日志复制推演 | `consensus/raft/package.ts` |
| `consensus` | `builtin__pos-finality` | PoS 权益证明与最终性推演 | `consensus/pos/package.ts` |
| `consensus` | `builtin__consensus-ethereum-pos-finality` | Ethereum PoS 链头选择与最终性推演 | `consensus/ethereum-pos-finality/package.ts` |
| `consensus` | `builtin__consensus-tendermint-rounds` | Tendermint 轮次锁定与提交推演 | `consensus/tendermint-rounds/package.ts` |
| `consensus` | `builtin__hotstuff-chained-bft` | HotStuff 链式 BFT 推演 | `consensus/hotstuff/package.ts` |
| `cryptography` | `builtin__crypto-digital-signature` | 数字签名与重放防护推演 | `cryptography/digital-signature/package.ts` |
| `cryptography` | `builtin__crypto-hash-chain` | 哈希链篡改扩散推演 | `cryptography/hash-chain/package.ts` |
| `cryptography` | `builtin__crypto-merkle-proof` | Merkle 证明路径推演 | `cryptography/merkle-proof/package.ts` |
| `cryptography` | `builtin__crypto-threshold-signature` | 门限签名聚合推演 | `cryptography/threshold-signature/package.ts` |
| `cryptography` | `builtin__crypto-zk-proof` | 零知识证明交互流程推演 | `cryptography/zk-proof/package.ts` |
| `network` | `builtin__network-p2p-discovery` | P2P 节点发现推演 | `network/p2p-discovery/package.ts` |
| `network` | `builtin__network-gossip-propagation` | Gossip 消息传播推演 | `network/gossip-propagation/package.ts` |
| `network` | `builtin__network-dht-routing` | DHT 异或路由推演 | `network/dht-routing/package.ts` |
| `network` | `builtin__network-partition-recovery` | 网络分区与恢复推演 | `network/network-partition/package.ts` |
| `network` | `builtin__network-latency-loss` | 延迟丢包与重传推演 | `network/latency-loss/package.ts` |
| `data-structure` | `builtin__data-blockchain-link` | 区块链父哈希结构推演 | `data-structure/blockchain-link/package.ts` |
| `data-structure` | `builtin__data-merkle-tree-structure` | Merkle Tree 构建更新推演 | `data-structure/merkle-tree-structure/package.ts` |
| `data-structure` | `builtin__data-patricia-trie` | Patricia Trie 状态树推演 | `data-structure/patricia-trie/package.ts` |
| `data-structure` | `builtin__data-utxo-set` | UTXO 集合更新推演 | `data-structure/utxo-set/package.ts` |
| `data-structure` | `builtin__data-state-snapshot` | 状态快照与回滚推演 | `data-structure/state-snapshot/package.ts` |
| `contract-security` | `builtin__security-reentrancy` | 重入攻击与防护推演 | `contract-security/reentrancy/package.ts` |
| `contract-security` | `builtin__security-access-control` | 授权缺陷与最小权限推演 | `contract-security/access-control/package.ts` |
| `contract-security` | `builtin__security-oracle-manipulation` | 预言机操纵防护推演 | `contract-security/oracle-manipulation/package.ts` |
| `contract-security` | `builtin__security-flash-loan` | 闪电贷组合攻击推演 | `contract-security/flash-loan/package.ts` |
| `contract-security` | `builtin__security-integer-boundary` | 整数边界与 checked 运算推演 | `contract-security/integer-boundary/package.ts` |
| `transaction-runtime` | `builtin__runtime-transaction-lifecycle` | 交易生命周期推演 | `transaction-runtime/transaction-lifecycle/package.ts` |
| `transaction-runtime` | `builtin__runtime-nonce-ordering` | Nonce 顺序与替换交易推演 | `transaction-runtime/nonce-ordering/package.ts` |
| `transaction-runtime` | `builtin__runtime-mempool-replacement` | Mempool 替换交易与 Nonce 队列推演 | `transaction-runtime/mempool-replacement/package.ts` |
| `transaction-runtime` | `builtin__runtime-gas-metering` | Gas 计量与回滚推演 | `transaction-runtime/gas-metering/package.ts` |
| `transaction-runtime` | `builtin__runtime-eip1559-fee-market` | EIP-1559 费用市场推演 | `transaction-runtime/eip1559-fee-market/package.ts` |
| `transaction-runtime` | `builtin__runtime-evm-call-stack` | EVM 调用栈与 revert 推演 | `transaction-runtime/evm-call-stack/package.ts` |
| `transaction-runtime` | `builtin__runtime-block-validation` | 区块验证与拒绝推演 | `transaction-runtime/block-validation/package.ts` |
| `cross-chain-system` | `builtin__cross-message-lifecycle` | 跨链消息生命周期推演 | `cross-chain-system/cross-chain-message/package.ts` |
| `cross-chain-system` | `builtin__cross-bridge-validation` | 跨链桥证明验证推演 | `cross-chain-system/bridge-validation/package.ts` |
| `cross-chain-system` | `builtin__cross-optimistic-rollup-fraud-proof` | Optimistic Rollup 欺诈证明推演 | `cross-chain-system/optimistic-rollup-fraud-proof/package.ts` |
| `cross-chain-system` | `builtin__cross-zk-rollup-proof-verification` | ZK Rollup 批次证明与验证推演 | `cross-chain-system/zk-rollup-proof-verification/package.ts` |
| `cross-chain-system` | `builtin__cross-finality-confirmation` | 跨链最终性确认推演 | `cross-chain-system/finality-confirmation/package.ts` |
| `cross-chain-system` | `builtin__cross-multisig-committee` | 跨链多签委员会推演 | `cross-chain-system/multisig-committee/package.ts` |
| `cross-chain-system` | `builtin__cross-replay-protection` | 跨链消息重放防护推演 | `cross-chain-system/replay-protection/package.ts` |

每个内置仿真包必须包含以下能力:

- 自描述 `meta`,且 `code` 使用 `builtin__` 前缀。
- 纯函数 `initState` 与 `reducer`,所有随机和阶段推进必须可由 `seed + events` 复现。
- 声明式 `interactions`,至少包含一个可改变走势的用户操作;攻击类操作只通过 `labelTag: "attack"` 表示,机制仍是普通交互。
- `render` 必须输出 `TeachingFrame`,包含 `summary`、`phase`、`focus`、`layout` 与 1 到 3 个封闭模式语义数据,不得自定义 DOM、Canvas 或外部渲染器。
- 每个内置包必须按算法真实过程输出元素生命周期,不能把所有历史消息、边、区块或步骤无差别堆到主视图。
- `narrative` 必须解释每个关键阶段"发生了什么"和"为什么重要"。
- `codeTrace` 必须给出教学代码片段、行映射和变量监视。
- `checkpoints` 必须产生可供 M3 仿真检查点判题器使用的结果快照。
- `scaleLimit` 必须限制节点数、最大 tick 和最大事件数。
- 以 `emits: "attack"` 注入异常、以 `emits: "recover"` 修复的内置流程必须先发生攻击再允许恢复,恢复完成后再次恢复必须被拒绝;需要其他顺序的扩展包必须通过自身 `availableWhen` 明确声明。

### 8.3 联动边界

允许:

- 同一仿真包内部的多模式联动,例如 PBFT 同时输出图网络、时序泳道与投票矩阵。
- 对照推演:同一包起两个独立会话,同 seed 同参数,各自持有自己的 events,不共享状态(见 07-架构设计.md §7)。
- 阶段式实验在 `initState` 时接收前置检查点注入的参数。

禁止:

- 旧版跨画布 owner 共享状态。
- 运行时跨阶段实时通信。
- 仿真包直接操作主页面 DOM、Token、网络或对象存储。
- 为内置包单独开后门,使其绕过第三方包必须遵守的协议。

### 8.4 开发者扩展要求

`@chaimir/sim-sdk` 必须导出开发者可复用的类型、模板和 manifest 摘要工具。教师或第三方新增仿真包时,应从这些公开 API 开始,而不是复制内部 Worker 或渲染器实现。

### 8.5 TeachingFrame 质量要求

- `layout.primary` 必须是该算法最能解释当前阶段的主模式,不是固定使用第一个模式。
- `evidence/timeline/metrics` 只放辅助判断所需内容;右侧信息可折叠,不得抢占主可视化。
- `focus.primary` 必须对应当前阶段核心对象,例如 PBFT 的当前消息、PoW 的竞争链尖、Merkle 的证明路径、EVM 的活跃栈帧。
- `muted` 与元素 `meta.lifecycle` 必须用于历史淡化和离场,避免可视化过程不断叠加。
- 同一分类内可以共享模式组合,但每个算法必须有自己的过程表达和知识点焦点,不得只套通用模板。
```
