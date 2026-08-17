/**
 * biz 出口:业务语义层(墨底教学舞台)统一导出。
 * 对外面两块:TeachingFrame 的三区渲染组件(阶段说明 / 主舞台 / 辅助段 / 事件流)+ 代码执行追踪面板,
 * 以及帧分区/视觉规则工具(供页面装配槽位)。单个模式渲染器不外露 —— 模式分派由 PatternView
 * 内部完成,页面只按 layout 用这些组件。
 */
export {
  TeachingFrameBrief,
  TeachingFrameStage,
  TeachingFrameAside,
  type TeachingFrameBriefProps,
  type TeachingFrameStageProps,
  type TeachingFrameAsideProps,
} from "./TeachingFrameStage";
export { TeachingFrameStream, type TeachingFrameStreamProps } from "./TeachingFrameStream";
export { CodeTracePanel, type CodeTracePanelProps } from "./CodeTracePanel";
export {
  asidePanels,
  frameHasAside,
  partitionAnnotations,
  pickPatterns,
  type AnnotationPartition,
  type AsidePanel,
} from "./frameLayout";
export {
  frameHasStream,
  frameStreamEntries,
  type FrameStreamEntry,
} from "./frameStream";
export {
  EMPHASIS_LABEL,
  LIFECYCLE_LABEL,
  matrixCellId,
  patternElementIds,
  progressPercent,
  resolveEmphasis,
  shortLabel,
  type FrameEmphasis,
  type PatternDensity,
} from "./frameVisual";
