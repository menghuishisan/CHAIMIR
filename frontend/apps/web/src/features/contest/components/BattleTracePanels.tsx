// 对抗赛回放台的攻防拓扑与链上日志流(规范 §7.2 C 创新点②)。
//
// 两者同源于一份回放归档:双方参战物的攻防角色只在归档的 initial_state 里
// —— 对局列表接口不带它,本队参战记录也只有本队自己的角色。故「读取轨迹」之前
// 页面如实说明拓扑画不出来,不用猜出来的角色先画一个。
//
// 分区遵守规范 §7.1 的状态与事件分流:拓扑与逐帧游标是有界状态,归中舞台;
// 链上动作是无界序列,归右侧事件流。日志按游标增长(拖到第 n 步就只显示前 n 步),
// 这是「日志流随时间轴构建」的字面实现 —— 序列本身是后端记录的事实,不补帧、不插值。

import { useEffect, useMemo, useRef } from 'react'
import { ShieldCheck, Swords, TriangleAlert } from 'lucide-react'
import { BattleRole, type BattleReplayArchive, type BattleReplayAction } from '@chaimir/api-client'
import { Badge, Button, cn, useReducedMotion } from '@chaimir/ui'
import { battleChainOpLabel, battleRoleLabel } from '../../../utils/labels/contest'

/** 拓扑画布的逻辑坐标系:等比缩放到容器宽度,不写死像素。 */
const CANVAS = { width: 520, height: 160 } as const
const NODE = { radius: 34, leftX: 110, rightX: 410, y: 78 } as const

export interface AttackDefenseTopologyProps {
  archive: BattleReplayArchive
  /** 本队在这一局是 A 方还是 B 方;分不出时两侧都按「参战物」呈现 */
  mySide?: 'a' | 'b'
  /** 逐帧游标:已执行到第几步(0 表示这一局还没开始) */
  step: number
}

/**
 * AttackDefenseTopology 画一局的攻防关系:守方玉、攻方 crimson,箭头指向被攻击的一侧。
 * 节点上的版本号与摘要都来自归档,箭头上的进度是「已执行到第几步」的事实,不是动画。
 */
export function AttackDefenseTopology({ archive, mySide, step }: AttackDefenseTopologyProps) {
  const total = archive.actions.length
  const attackerIsA = archive.initial_state.entry_a.role === BattleRole.ATTACK
  const left = attackerIsA ? archive.initial_state.entry_a : archive.initial_state.entry_b
  const right = attackerIsA ? archive.initial_state.entry_b : archive.initial_state.entry_a
  const leftIsMine = mySide === (attackerIsA ? 'a' : 'b')
  const executed = Math.min(step, total)
  const lastOp = executed > 0 ? archive.actions[executed - 1].op : undefined

  return (
    <section className="flex shrink-0 flex-col gap-2" aria-label="攻防拓扑">
      <h2 className="text-sm font-medium text-on-dark">攻防拓扑</h2>
      <svg
        viewBox={`0 0 ${CANVAS.width} ${CANVAS.height}`}
        className="w-full"
        role="img"
        aria-label={topologySummary(archive, executed, total)}
      >
        {/* 连线:从攻方指向守方。已执行步数 > 0 时实线,否则虚线表示尚未开打 */}
        <line
          x1={NODE.leftX + NODE.radius}
          y1={NODE.y}
          x2={NODE.rightX - NODE.radius - 10}
          y2={NODE.y}
          strokeWidth={2}
          strokeDasharray={executed > 0 ? undefined : '6 5'}
          className={executed > 0 ? 'stroke-on-dark-danger' : 'stroke-on-dark-faint'}
        />
        <polygon
          points={`${NODE.rightX - NODE.radius - 10},${NODE.y - 5} ${NODE.rightX - NODE.radius},${NODE.y} ${NODE.rightX - NODE.radius - 10},${NODE.y + 5}`}
          className={executed > 0 ? 'fill-on-dark-danger' : 'fill-on-dark-faint'}
        />
        <text
          x={(NODE.leftX + NODE.rightX) / 2}
          y={NODE.y - 14}
          textAnchor="middle"
          className="fill-on-dark-sub text-xs"
        >
          {executed > 0 ? `已执行 ${executed}/${total} 步` : `共 ${total} 步待回放`}
        </text>
        {lastOp ? (
          <text
            x={(NODE.leftX + NODE.rightX) / 2}
            y={NODE.y + 24}
            textAnchor="middle"
            className="fill-on-dark text-xs"
          >
            {battleChainOpLabel(lastOp)}
          </text>
        ) : null}

        <TopologyNode
          cx={NODE.leftX}
          role={left.role}
          versionNo={left.version_no}
          mine={leftIsMine}
        />
        <TopologyNode
          cx={NODE.rightX}
          role={right.role}
          versionNo={right.version_no}
          mine={mySide !== undefined && !leftIsMine}
        />
      </svg>
      <p className="text-xs text-on-dark-faint">
        角色与版本取自这一局的轨迹归档;箭头从攻方指向守方,线上的步数是归档里已执行到的位置。
      </p>
    </section>
  )
}

interface TopologyNodeProps {
  cx: number
  role: BattleRole
  versionNo: number
  mine: boolean
}

/** TopologyNode 画一侧参战物:攻方 crimson、守方玉,颜色之外用角色文字与图标双通道表达。 */
function TopologyNode({ cx, role, versionNo, mine }: TopologyNodeProps) {
  const attacking = role === BattleRole.ATTACK
  const toneClass = attacking ? 'text-on-dark-danger' : 'text-accent'
  return (
    <g className={toneClass}>
      <circle
        cx={cx}
        cy={NODE.y}
        r={NODE.radius}
        className="fill-dark-elevated"
        stroke="currentColor"
        strokeWidth={mine ? 2.5 : 1.4}
      />
      <text x={cx} y={NODE.y - 4} textAnchor="middle" fill="currentColor" className="text-xs">
        {battleRoleLabel(role)}
      </text>
      <text x={cx} y={NODE.y + 12} textAnchor="middle" className="fill-on-dark-sub text-xs">
        第 {versionNo} 版
      </text>
      {mine ? (
        <text x={cx} y={NODE.y + NODE.radius + 14} textAnchor="middle" className="fill-on-dark text-xs">
          本队
        </text>
      ) : null}
    </g>
  )
}

/** topologySummary 给读屏一句把拓扑说清的话:谁攻谁守、本队在哪一侧、执行到哪。 */
function topologySummary(archive: BattleReplayArchive, executed: number, total: number): string {
  const roleA = battleRoleLabel(archive.initial_state.entry_a.role)
  const roleB = battleRoleLabel(archive.initial_state.entry_b.role)
  return `这一局由${roleA}与${roleB}对阵,链上动作共 ${total} 步,当前回放到第 ${executed} 步。`
}

export interface ChainLogStreamProps {
  archive?: BattleReplayArchive
  /** 逐帧游标:只显示前 step 步,拖动即让日志随时间轴增长 */
  step: number
  /** 读取轨迹的动作;未读取时右栏只给这一个入口 */
  onLoad: () => void
  loading: boolean
  /** 用户向失败文案 */
  error?: string
  /** 这一局有没有归档;没有归档时不给读取入口 */
  available: boolean
}

/**
 * ChainLogStream 渲染链上日志流:按执行顺序逐条列出动作,跟随游标滚到最新一条。
 * 无界序列只在这一栏出现(规范 §7.1 状态与事件分流),舞台不再重复一份。
 */
export function ChainLogStream({
  archive,
  step,
  onLoad,
  loading,
  error,
  available,
}: ChainLogStreamProps) {
  const reducedMotion = useReducedMotion()
  const listRef = useRef<HTMLOListElement>(null)
  const visible = useMemo(
    () => (archive ? archive.actions.slice(0, Math.min(step, archive.actions.length)) : []),
    [archive, step],
  )

  // 跟随最新:游标前进时把最后一条滚进视口;减弱动效偏好下不做平滑滚动
  useEffect(() => {
    const node = listRef.current
    if (!node) return
    node.scrollTo({ top: node.scrollHeight, behavior: reducedMotion ? 'auto' : 'smooth' })
  }, [reducedMotion, visible.length])

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex shrink-0 flex-col gap-2 border-b border-dark-line p-3">
        <div className="flex items-center justify-between gap-2">
          <h2 className="text-sm font-medium text-on-dark">链上日志流</h2>
          {archive ? (
            <Badge onDark tone="neutral">
              {visible.length}/{archive.actions.length} 步
            </Badge>
          ) : null}
        </div>
        {!available ? (
          <p className="text-xs text-on-dark-sub">这一局没有留下轨迹记录,没有可回放的链上动作。</p>
        ) : archive ? (
          <p className="text-xs text-on-dark-faint">拖动中间的逐帧游标,日志按执行顺序增长。</p>
        ) : (
          <>
            <p className="text-xs text-on-dark-sub">
              轨迹归档需要单独取件:每次取件都重新签发一次性授权,不把存储地址交给浏览器。
            </p>
            <Button variant="on-dark" size="sm" loading={loading} onClick={onLoad}>
              读取这一局的轨迹
            </Button>
          </>
        )}
        {error ? <p className="text-xs text-on-dark-danger">{error}</p> : null}
      </div>

      <ol ref={listRef} className="flex min-h-0 flex-1 flex-col overflow-y-auto p-3">
        {visible.map((action) => (
          <li key={action.seq} className="border-b border-dark-line py-2 last:border-b-0">
            <ChainLogRow action={action} />
          </li>
        ))}
        {archive && visible.length === 0 ? (
          <li className="text-xs text-on-dark-sub">游标在这一局开始之前,往右拖就能看到链上动作。</li>
        ) : null}
      </ol>
    </div>
  )
}

/** ChainLogRow 渲染一条链上动作:步号、操作名与归档里带的入参/结果键值。 */
function ChainLogRow({ action }: { action: BattleReplayAction }) {
  const payload = readableEntries(action.payload)
  const output = readableEntries(action.output)
  return (
    <div className="flex flex-col gap-1">
      <div className="flex items-center gap-2">
        <span className="font-mono text-xs tabular-nums text-on-dark-faint">
          第 {action.seq} 步
        </span>
        <span className="text-sm text-on-dark">{battleChainOpLabel(action.op)}</span>
      </div>
      {payload.length > 0 ? <KeyValueLine label="入参" entries={payload} /> : null}
      {output.length > 0 ? <KeyValueLine label="结果" entries={output} /> : null}
    </div>
  )
}

/** KeyValueLine 渲染一行键值:归档里的键由运行时决定,故原样显示键名并标明这是链上记录。 */
function KeyValueLine({ label, entries }: { label: string; entries: Array<[string, string]> }) {
  return (
    <dl className="flex flex-wrap gap-x-3 gap-y-0.5">
      <dt className="text-xs text-on-dark-sub">{label}</dt>
      {entries.map(([key, value]) => (
        <dd key={key} className="font-mono text-xs text-on-dark-sub">
          {key}={value}
        </dd>
      ))}
    </dl>
  )
}

/**
 * readableEntries 把归档里的开放对象压成可读键值。
 * 只展开 JSON 基础类型:嵌套对象/数组的形状由运行时决定,展开它等于替运行时编排界面。
 */
function readableEntries(source?: Record<string, unknown>): Array<[string, string]> {
  if (!source) return []
  const out: Array<[string, string]> = []
  for (const [key, value] of Object.entries(source)) {
    if (typeof value === 'string') out.push([key, truncate(value)])
    else if (typeof value === 'number' || typeof value === 'boolean') out.push([key, String(value)])
  }
  return out
}

/** truncate 截断过长的哈希/地址,避免一行铺满整栏;完整值在归档文件里。 */
function truncate(value: string): string {
  return value.length > 24 ? `${value.slice(0, 10)}…${value.slice(-6)}` : value
}

export interface TraceAssertionsProps {
  archive: BattleReplayArchive
}

/**
 * TraceAssertions 渲染这一局的断言结论。
 * 它是有界状态(条数固定),故留在舞台而不进事件流;失败断言配图标与状态词,不只靠颜色。
 */
export function TraceAssertions({ archive }: TraceAssertionsProps) {
  const { details, passed, score, max_score: maxScore } = archive.result
  if (details.length === 0) {
    return (
      <p className="text-xs text-on-dark-sub">
        这一局的归档没有断言明细,判定结论为{passed ? '通过' : '未通过'}。
      </p>
    )
  }
  return (
    <section className="flex flex-col gap-2" aria-label="断言结论">
      <div className="flex flex-wrap items-center gap-2">
        <h3 className="text-xs font-medium text-on-dark-sub">断言结论</h3>
        <Badge onDark tone={passed ? 'success' : 'danger'}>
          {passed ? '判定通过' : '判定未通过'}
        </Badge>
        <span className="font-mono text-xs tabular-nums text-on-dark-sub">
          {score}/{maxScore} 分
        </span>
      </div>
      <ul className="flex flex-col gap-1">
        {details.map((detail, index) => (
          <li
            key={`${index}-${detail.case}`}
            className={cn(
              'flex flex-wrap items-center gap-2 rounded-md px-2 py-1.5',
              detail.passed ? 'bg-dark-surface' : 'bg-dark-elevated',
            )}
          >
            {detail.passed ? (
              <ShieldCheck aria-hidden="true" className="size-3.5 shrink-0 text-accent" />
            ) : (
              <TriangleAlert aria-hidden="true" className="size-3.5 shrink-0 text-on-dark-danger" />
            )}
            <span className="min-w-0 flex-1 truncate text-xs text-on-dark">
              {detail.case || `第 ${index + 1} 条断言`}
            </span>
            <span className="text-xs text-on-dark-sub">{detail.passed ? '守住' : '被打穿'}</span>
            {detail.hint ? (
              <span className="w-full text-xs text-on-dark-faint">{detail.hint}</span>
            ) : null}
          </li>
        ))}
      </ul>
    </section>
  )
}

/** TraceLegend 在舞台上说明拓扑配色,保证颜色不是唯一信息载体。 */
export function TraceLegend() {
  return (
    <div className="flex flex-wrap items-center gap-2 text-xs text-on-dark-sub">
      <span className="inline-flex items-center gap-1">
        <ShieldCheck aria-hidden="true" className="size-3.5 text-accent" />
        守方
      </span>
      <span className="inline-flex items-center gap-1">
        <Swords aria-hidden="true" className="size-3.5 text-on-dark-danger" />
        攻方
      </span>
      <span>粗描边的一侧是本队。</span>
    </div>
  )
}
