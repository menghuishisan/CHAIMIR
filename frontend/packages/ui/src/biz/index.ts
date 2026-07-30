/**
 * biz 出口:业务语义层(墨底教学舞台)统一导出。
 * 唯一对外面:TeachingFrame 的三栏渲染组件 + 帧分区/视觉规则工具(供页面装配槽位)。
 * 单个模式渲染器不外露 —— 模式分派由 PatternView 内部完成,页面只按 layout 用三栏组件。
 */
export {
  TeachingFrameBrief,
  TeachingFrameStage,
  TeachingFrameAside,
  type TeachingFrameBriefProps,
  type TeachingFrameStageProps,
  type TeachingFrameAsideProps,
} from "./TeachingFrameStage";
export {
  asidePanels,
  frameHasAside,
  partitionAnnotations,
  pickPatterns,
  type AnnotationPartition,
  type AsidePanel,
} from "./frameLayout";
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
