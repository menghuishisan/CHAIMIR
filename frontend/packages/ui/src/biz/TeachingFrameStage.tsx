/**
 * TeachingFrameStage:把 TeachingFrame 按 layout 分区渲染成三区骨架内容(规范 §7.1)。
 * 三个导出对应工作台壳的三个槽位,共用同一份帧数据:
 * TeachingFrameBrief(左辅助区:这一步做了什么/为什么重要)、
 * TeachingFrameStage(中主舞台:layout.primary,永不滚动)、
 * TeachingFrameAside(左辅助区其余段:证据/指标/追踪/检查点)。
 * 消息/调用这类无界元素不在这三处出现 —— 它们归右侧事件流(见 frameStream / TeachingFrameStream)。
 * 全部为纯受控展示组件:不取数、不持有仿真运行时,选中态与选择回调由页面从仿真状态传入。
 */
import { cn } from "../lib/cn";
import { Icon } from "../lib/icon";
import { asidePanels, partitionAnnotations } from "./frameLayout";
import { TONE_ICON, TONE_TEXT, type DarkTone } from "./patterns/darkTone";
import { PatternView } from "./patterns/PatternView";
import type { FrameAnnotation, FrameIntent, PatternBinding, TeachingFrame } from "@chaimir/sim-sdk";

/** 帧意图的用户向说法:让学习者知道这一屏在干什么(观察/对比/验证/…) */
const INTENT_LABEL: Record<FrameIntent, string> = {
  observe: "观察",
  compare: "对比",
  verify: "验证",
  debug: "排查",
  attack: "攻击",
  recover: "恢复",
  replay: "回放",
};

/** 标注语气 → 墨底色阶:info 走「进行中」蓝,其余同名对应 */
const ANNOTATION_TONE: Record<FrameAnnotation["tone"], DarkTone> = {
  info: "active",
  success: "success",
  warning: "warning",
  danger: "danger",
};

/* ---------------------------------------------------------------- 标注 */

interface AnnotationListProps {
  /** 清单标题(读屏用) */
  label: string;
  annotations: FrameAnnotation[];
}

/** AnnotationList:标注条目 —— 图标 + 文字,语气只影响图标与文字色,不改变版式 */
function AnnotationList({ label, annotations }: AnnotationListProps) {
  return (
    <ul aria-label={label} className="flex flex-col gap-1">
      {annotations.map((annotation) => {
        const tone = ANNOTATION_TONE[annotation.tone];
        return (
          <li key={annotation.id} className="flex items-start gap-2">
            <Icon icon={TONE_ICON[tone]} size="xs" className={cn("mt-0.5", TONE_TEXT[tone])} />
            <span className={cn("min-w-0 text-xs", TONE_TEXT[tone])}>{annotation.text}</span>
          </li>
        );
      })}
    </ul>
  );
}

/* ---------------------------------------------------------------- 左辅助区:阶段说明 */

export interface TeachingFrameBriefProps {
  frame: TeachingFrame;
  className?: string;
}

/**
 * TeachingFrameBrief:阶段说明区(§7.2 B「这一步做了什么 + 为什么重要」)。
 * 只有两段:不做「看哪里」—— frame.focus 已经用舞台高亮指出该看哪个元素,
 * 再用一段文字复述状态摘要只是重复。标题旁标注本屏意图;summary 作为整帧一句话结论收尾。
 */
export function TeachingFrameBrief({ frame, className }: TeachingFrameBriefProps) {
  const { phase, summary } = frame;
  const sections: Array<[string, string]> = [
    ["这一步做了什么", phase.explanation.what],
    ["为什么重要", phase.explanation.why],
  ];

  return (
    <div className={cn("flex flex-col gap-3 p-4", className)}>
      <div>
        <div className="flex items-baseline gap-2">
          <h2 className="min-w-0 text-sm font-semibold text-on-dark">{phase.title}</h2>
          <span className="shrink-0 text-xs text-accent">{INTENT_LABEL[phase.intent]}</span>
        </div>
        <p className="mt-1 text-xs text-on-dark-sub">{summary}</p>
      </div>
      {sections.map(([title, text]) => (
        <div key={title}>
          <h3 className="text-xs font-medium text-on-dark">{title}</h3>
          <p className="mt-0.5 text-xs leading-relaxed text-on-dark-sub">{text}</p>
        </div>
      ))}
    </div>
  );
}

/* ---------------------------------------------------------------- 中主舞台 */

export interface TeachingFrameStageProps {
  frame: TeachingFrame;
  /** 当前选中的元素编号(来自仿真状态) */
  selectedElementId?: string;
  /** 元素选择回调;不传则舞台只读(如公开回放) */
  onSelectElement?: (elementId: string) => void;
  className?: string;
}

/**
 * TeachingFrameStage:主舞台 —— 渲染 layout.primary 指向的模式。
 * 舞台永不滚动(§7.1):自身撑满槽位高度,图形取剩余高度按 viewBox 缩放,
 * 清单超长由清单自身滚动(高度分配落在 PatternFrame)。
 * 该模式上的标注紧随视图展示;target 未指向本帧任何模式或元素的标注也在此统一列出,
 * 保证作者写下的标注不会因为编号写错而被静默丢弃。
 */
export function TeachingFrameStage({
  frame,
  selectedElementId,
  onSelectElement,
  className,
}: TeachingFrameStageProps) {
  const primary = frame.patterns.find((pattern) => pattern.id === frame.layout.primary);
  const { byPattern, unassigned } = partitionAnnotations(frame);
  const primaryAnnotations = primary ? byPattern.get(primary.id) : undefined;

  return (
    <div className={cn("flex h-full min-h-0 flex-col gap-3 p-4", className)}>
      {primary && (
        <>
          <h2 className="shrink-0 text-sm font-semibold text-on-dark">{primary.title}</h2>
          <div className="min-h-0 flex-1">
            <PatternView
              pattern={primary}
              focus={frame.focus}
              density="stage"
              selectedElementId={selectedElementId}
              onSelectElement={onSelectElement}
            />
          </div>
        </>
      )}
      {primaryAnnotations && (
        <div className="shrink-0">
          <AnnotationList label="本视图标注" annotations={primaryAnnotations} />
        </div>
      )}
      {unassigned.length > 0 && (
        <div className="shrink-0 rounded-md border border-dark-line bg-dark-elevated p-2">
          <h3 className="mb-1 text-xs font-medium text-on-dark">其他说明</h3>
          <AnnotationList label="其他说明" annotations={unassigned} />
        </div>
      )}
    </div>
  );
}

/* ---------------------------------------------------------------- 左辅助区:模式段 */

export interface TeachingFrameAsideProps {
  frame: TeachingFrame;
  selectedElementId?: string;
  onSelectElement?: (elementId: string) => void;
  className?: string;
}

/**
 * TeachingFrameAside:左辅助区的模式段 —— 按证据 → 指标 → 执行追踪 → 检查点顺序排面板。
 * 每块面板标出职责与视图标题,内容用 panel 密度渲染(约 300px 窄栏可读);
 * 主舞台已渲染的模式与时间线泳道不在此重复(见 frameLayout.asidePanels)。
 */
export function TeachingFrameAside({
  frame,
  selectedElementId,
  onSelectElement,
  className,
}: TeachingFrameAsideProps) {
  const panels = asidePanels(frame);
  const { byPattern } = partitionAnnotations(frame);

  return (
    <div className={cn("flex flex-col", className)}>
      {panels.map(({ region, pattern }) => (
        <AsideSection
          key={pattern.id}
          region={region}
          pattern={pattern}
          focus={frame.focus}
          annotations={byPattern.get(pattern.id)}
          selectedElementId={selectedElementId}
          onSelectElement={onSelectElement}
        />
      ))}
    </div>
  );
}

interface AsideSectionProps {
  region: string;
  pattern: PatternBinding;
  focus: TeachingFrame["focus"];
  annotations?: FrameAnnotation[];
  selectedElementId?: string;
  onSelectElement?: (elementId: string) => void;
}

/** AsideSection(内部):一块辅助面板 —— 职责标签 + 视图标题 + 视图 + 该视图的标注 */
function AsideSection({
  region,
  pattern,
  focus,
  annotations,
  selectedElementId,
  onSelectElement,
}: AsideSectionProps) {
  return (
    <section className="flex flex-col gap-2 border-b border-dark-line p-3 last:border-b-0">
      <div className="flex items-baseline justify-between gap-2">
        <h3 className="min-w-0 truncate text-xs font-medium text-on-dark">{pattern.title}</h3>
        <span className="shrink-0 text-xs text-on-dark-sub">{region}</span>
      </div>
      <PatternView
        pattern={pattern}
        focus={focus}
        density="panel"
        selectedElementId={selectedElementId}
        onSelectElement={onSelectElement}
      />
      {annotations && <AnnotationList label={`${pattern.title} 标注`} annotations={annotations} />}
    </section>
  );
}
