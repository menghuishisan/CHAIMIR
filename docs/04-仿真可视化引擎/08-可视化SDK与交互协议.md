# M4 仿真可视化引擎 — 可视化 SDK 与交互协议

> 本文是仿真包作者的开发契约 SSOT:仿真包怎么写、SDK 怎么用、交互怎么声明。
> 面向第三方/教师扩展开发者。
> 最后更新:2026-11-27

---

## 1. 仿真包总览

仿真包归档根目录必须包含 `sim-package.json`。若归档工具自动包了一层顶级目录,允许
`<top-level>/sim-package.json`;其他位置的同名文件不作为协议入口。后端上传审核只读取这
一个 manifest 生成交互白名单和审核摘要,运行时代码仍随 bundle 保存在对象存储。

**扩展包在后端隔离容器内执行**(见 07-架构设计.md §8):归档字节不下发浏览器,而是在会话建立时
经 k8s exec stdin 推入受限计算 Pod,容器内校验 sha256 后按 `meta.entry` 装配。
对作者而言执行位置是透明的 —— 下述协议与内置包完全一致,不为"跑在哪"写第二套实现。

运行契约由 SDK 的[公开类型](../../frontend/packages/sim-sdk/src/types.ts)定义，入口导出 `SimPackage`、`SimMeta`、`SimState`、`SimEvent`、`ReducerContext`、`InteractionDef`、`TeachingFrame` 等类型。本文件维护语义、安全与教学约束，不复制完整类型定义。

一个包包含：

- `meta`：稳定编码、名称、分类、版本、简介、学习目标与规模上限；扩展包还须有真实归档入口。
- `initState(params, seed)` 与 `reducer(state, event, context)`：构造初始状态并确定性演化；随机数使用 `context.random`。
- `interactions`：交互声明；`render(state)`：返回教学画面。
- `narrative`、`codeTrace` 与 `checkpoints`：教学叙事、代码追踪和可评测的检查点。自由探索可以不触发叙事，但包描述须满足统一协议的完整性要求。

`compute` 不由作者声明:执行位置按代码来源派生(内置=浏览器、教师/第三方=隔离容器),
提交表单与 manifest 都不含该字段。作者写一次协议,平台决定在哪跑。

**硬约束**:
- `reducer` 必须纯函数:`reducer(state,event,context)` 同输入必同输出,禁用 `Date.now()`/`Math.random()`(随机走种子 PRNG)。
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
- 归档可直接包含根清单和入口，也可整体包在唯一的一层目录内；入口相对该包根解析。根清单优先，嵌套清单不覆盖它。没有根清单时，普通文件必须全部位于唯一顶层目录内，禁止混合包根或借用目录外入口；上传与容器装配执行相同规则。
- 普通成员名先按统一归档原语去除两端空白、统一分隔符并规范化相对路径，再校验重复、越界和包根；运行器使用同一规范化规则。入口声明本身仍禁止 `..`，不能借归一化放宽声明约束。
  只接受 `.mjs`:归档内没有 `package.json` 时 Node 按 CJS 解析 `.js`,而扩展包必须默认导出 `SimPackage`,
  允许 `.js` 会让"能不能装配"取决于打包方式。缺失或非法即拒绝(`41002`)—— 容器需要它才能装配,没有它包就是不可运行的。
- `interactions[].emits` 生成 `sim_package.interaction_schema`,运行时只接受 manifest 声明过的事件和参数。
- `interactions[].label_tag` 只接受 `normal|recover|perturb|attack`(可缺省,缺省按 `normal` 处理),未知值即拒绝上架:
  标签决定按钮配色与攻击类的就地二次确认(§3.3),认不出的标签会让扩展包的破坏性操作画成普通推进色。
- `render.protocol` 必须是 `teaching-frame`。
- `render.patterns` 必须是 1~3 个封闭模式声明,每项必须有稳定 `id` 和 `mode`,`mode` 只能取 `graph|chain|tree|matrix|pipeline|lane|chart`。
- `render.patterns[].roles` 只能用于审核该模式可承担的教学区域,取值为 `primary|evidence|timeline|metrics|trace|checkpoints`,不得在模式数据中另行声明区域布局。
- `codeTrace` 使用 camelCase,与前端 TypeScript 协议一致;数据库字段 `code_trace` 只存不含源码正文的审核摘要。
- `checkpoints` 必须声明检查点 ID 与名称,隔离预览及运行快照中的 `checkpoint_results` 均以这些锚点派生结果；不另设检查点上报接口。
- manifest JSON 拒绝未知字段和尾随内容,避免同一协议出现兼容别名或灰色扩展。

### 1.2 官方前端 SDK 使用入口

前端官方 SDK 包为 `@chaimir/sim-sdk`,开发者新增仿真包必须从公开 API 开始,不得复制内置包、Worker 或渲染器实现。主入口导出:

- `SimPackage`、`SimState`、`SimEvent`、`TeachingFrame`、`PatternBinding` 等协议类型。
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

`codeTrace` 将可视化现象与教学代码建立单向关联。配置使用公开 `CodeTraceDef`、`LineMapping` 与 `VariableWatchDef`：源码和语言、从 1 开始的行映射、触发条件、注释及变量提取。运行状态的可选 `_trace` 使用 `TraceInfo`，携带本帧触发行号、JSON 变量值及执行路径。类型见[公开协议](../../frontend/packages/sim-sdk/src/types.ts)。

- 前端运行时自动读取 `state._trace`:高亮 `triggeredLines` 中的行、展示 `variables` 的值、按 `lineMapping` 显示教学注释。
- 设计边界(只允许单向映射):状态 → 代码行高亮、变量监视、教学注释;**禁止**代码行 → 状态的反向修改(这不是调试器)、禁止断点/单步执行、禁止修改源码后重新执行。
- `codeTrace` 仅声明教学追踪配置;后端 `Package` 只持久化审核摘要(`CodeTraceAudit`:语言、行数、映射数、变量数,字段定义见 `02-数据模型.md`),源码正文仍随 bundle 保存在对象存储,不得在领域模型上复制第二套传输结构。
- 代码追踪如何参与仿真会话与渲染,见 `05-业务流程与状态机.md` §9。

---

## 2. 状态与事件

状态以 `SimState` 为基础，包含 tick、阶段、步骤解释、指标和检查点值，包可扩展自己的算法数据。事件使用 `SimEvent`，包含 `type`、`source`、`atTick`、`seq`、JSON `payload` 和可选目标元素；来源为 `tick`、`user` 或 `system`。运行时负责分配时间与序号。

`reducer` 接收 `ReducerContext` 中的种子、tick、序号和确定性随机数。tick 推进、用户交互和系统事件共用该入口；同种子、参数与事件序列必须得到相同状态。持久化及隔离流的字段投影由[会话接口](03-接口设计.md#32-上报操作-用户)定义，不把传输字段直接当作 SDK 运行类型。

---

## 3. 交互声明协议(核心)

SDK 运行类型使用 `InteractionDef`：`id/kind/label/description/emits` 定义交互，`params` 使用 `FieldDef` 描述字段标签、类型、默认值、范围、选项和必填约束；`target` 与 `elementFilter` 约束目标，`availableWhen(state)` 判定当前是否可用，`labelTag` 与 `cooldownMs` 定义视觉标签和冷却。

上传 manifest 由 `createSimPackageManifest` 导出，使用 `element_filter`、`label_tag`、`cooldown_ms` 等归档字段，不携带可执行函数。运行包不得照抄 manifest 字段名，也不维护别名兼容；具体序列化结构见[manifest 导出入口](../../frontend/packages/sim-sdk/src/authoring/manifest.ts)。

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

## 4. 可视化模式

> 架构职责见[架构设计](07-架构设计.md#5-teachingframe-教学画面协议核心)。字段和语义由本节统一定义。作者**不写渲染代码**,只把状态映射到平台提供的封闭模式集(7 种),由模式引擎负责布局/绘制/动画。两条红线:① 封闭模式集 + 评审闸门(无自定义渲染器后门);② 作者只给语义数据。

### 4.1 封闭模式集(7 种,平台维护)

| 模式 | key | 输入语义数据 | 引擎负责 |
| --- | --- | --- | --- |
| 图网络 | `graph` | 节点、连线与 ring/grid/layered 布局 | 节点布局、消息飞线动画、状态着色 |
| 链式 | `chain` | blocks[]、分叉关系、最长链标记 | 块序列排布、分叉绘制、高亮 |
| 树形 | `tree` | 树节点、验证路径 | 树布局、路径高亮、构建动画 |
| 矩阵 | `matrix` | cells[][]、值/状态 | 网格、单元着色、变化闪烁 |
| 流水线 | `pipeline` | steps[]、当前步、数据值 | 分步流动、寄存器变化动画 |
| 时序泳道 | `lane` | 参与者、带时间的消息与当前时间 | 阶段进度、消息时序 |
| 图表 | `chart` | 带横纵坐标点的数值序列与单位 | 坐标轴、曲线、动态更新 |

### 4.2 TeachingFrame 映射(作者声明,非绘制)

`TeachingFrame` 包含 `summary`、`phase`、`focus`、`layout`、1–3 个 `patterns` 和可选 `annotations`。阶段通过 `what/why` 解释发生了什么及意义，`focus` 指明主次和淡化对象；布局引用画面内的模式 ID。模式使用 `PatternBinding` 的判别联合，各模式有自己的完整数据结构，不以 `unknown` 或空对象替代。类型与结构上限由[公开协议](../../frontend/packages/sim-sdk/src/types.ts)及其生成 schema 定义。

作者只实现 `render(state)` 中的状态到教学语义映射,**绝不接触坐标/canvas/DOM**。`layout.primary` 必须指向 `patterns` 中存在的模式 ID;右侧证据、时间线、指标等也只能引用同一帧内的模式 ID。

运行数据结构以 `sim-sdk/src/types.ts` 为准。`node scripts/codegen/sim-runtime-protocol.mjs` 从类型生成 JSON Schema 和无运行依赖的校验器；`--check` 只检查产物漂移。后端复用已有 jsonschema，SDK 使用 Ajv 预编译产物；浏览器在隔离流入口校验描述和完整快照。两端都必须拒绝缺少阶段释义、图形正文、必填数组或无效枚举的数据，不补空值掩盖错误。模式数量、结构和固定上限由同一 schema 约束，布局引用及按包声明的节点、连线、矩阵和序列容量另做语义校验；失败不进入渲染或保存为审核样例。

### 4.3 元素生命周期与焦点

封闭模式内部的节点、边、区块、树节点、矩阵单元、流水线步骤、泳道消息都可以携带统一 `meta`:

元素元数据使用 `VisualElementMeta`：稳定 `id/label/role`、带起止 tick 的生命周期、视觉权重与可选解释。生命周期为 `entering/active/settled/leaving/archived`，权重为 `focus/context/history/ghost`，平台统一解释。

规则:

- 当前阶段关键元素必须出现在 `focus.primary` 或元素 `meta.emphasis="focus"` 中。
- 历史元素必须显式标为 `history` 或 `ghost`,由平台统一淡化、折叠或只保留摘要。
- 算法不得把无限增长的历史消息全部作为普通活跃边/消息输出。
- 选择、键盘焦点、读屏文本都以元素 `id/label/role` 为准。

### 4.4 新增模式的闸门(防碎片化)

- 仿真作者**不能自带渲染器**。现有 7 种模式无法表达的全新隐喻(罕见),提交**平台级模式评审**。
- 评审通过 → 新模式进入封闭集,成为**所有仿真的公共资产**,后续复用。
- 扩展只组合平台已有模式，不开放任意自定义渲染器。

---

## 5. 教学叙事协议

运行包使用 `NarrativeStep`，以 `trigger(state, event?)` 函数判定触发，提供标题、高亮、解释、默认时长及可选问题。问题的提示、选项、答案与 `checkpointId` 由 `QuestionCheckpoint` 定义；检查点评测仍由 `CheckpointDef.evaluate` 返回明确结果。

归档与运行描述只携带可序列化的叙事字段，触发函数留在执行入口中。叙事与状态机解耦，同一仿真可挂不同叙事；设问结果可供 M3 读取检查点判分，不把叙事描述当作已完成结果。

---

## 6. 无障碍与响应式约束

- 所有交互控件键盘可达(Tab/Enter/方向键),焦点可见。
- 画布提供文本化状态摘要(供读屏与无法看动画时理解)。
- 布局响应式:复合组件自适应容器尺寸,低层绘制用逻辑坐标缩放。
- 颜色不作唯一信息载体(配图标/文字),保证色弱可用。

---

## 7. 开发模板与归档清单

以下通过公开 API 创建模板并导出上传清单。作者须用真实算法替换模板的状态、交互、叙事、代码追踪、检查点和教学画面，再按 §1.2 打包：

```typescript
import { createDeveloperTemplate, createSimPackageManifest } from "@chaimir/sim-sdk";

const simPackage = createDeveloperTemplate("teacher_1001__pow-mining", "dist/index.mjs");
export const manifest = createSimPackageManifest(simPackage, "dist/index.mjs");
export default simPackage;
```

`manifest` 序列化为归档根的 `sim-package.json`，ESM 默认导出打包到 `dist/index.mjs`。模板与导出器共用公开校验，不在文档维护另一套可执行包。完整模板见[authoring/templates.ts](../../frontend/packages/sim-sdk/src/authoring/templates.ts)，教学算法参考本文标准库入口。

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

内置包通过 `sim_package` 目录供业务检索。入库清单由 `node scripts/codegen/export-sim-builtin-catalog.mjs` 从 `frontend/packages/sim-sdk/src/builtin/` 下的包入口求值，生成 `backend/internal/modules/sim/builtin_catalog.json`；后端嵌入后在 `migrate-and-seed` 阶段幂等同步。流程见[业务流程](05-业务流程与状态机.md#41-平台内置包不走审核流程部署期入库)。源码或交互声明变更时重新导出，`scripts/audit/sim-catalog-drift.mjs` 校验产物与源码一致；未同步的交互不能进入操作白名单。

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

- 跨画布 owner 共享状态。
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
- 正在发生的消息、投票、区块和证明路径使用 `active/focus`，已完成且仍有教学价值的对象使用 `settled/history`，不再参与当前判断的对象使用 `archived/ghost`。

完成状态必须来自算法实际完成条件，摘要、检查点和流程进度保持一致。例如 Gas 计量在指令执行、退款计算和费用结算全部完成后才达成结算检查点；终态流程标记完成且进度 100%。回退至退款阶段或重置后撤销完成状态，预算不足的失败路径不能判为成功。
