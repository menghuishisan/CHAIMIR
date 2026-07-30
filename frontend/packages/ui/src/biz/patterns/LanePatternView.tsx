/**
 * LanePatternView:时序泳道视图(lane 模式)。
 * 宽舞台:参与者为纵向泳道,时间自上而下,消息为跨泳道的箭头,当前时间用横向标记线;
 * 窄栏:退化为按时间排序的消息清单(288px 放不下多泳道画布)。两种形态信息等价。
 * 带 ProcessSpan 的消息在清单里用静态进度条表达送达进度,不做飞行动画。
 */
import { useId } from "react";
import { cn } from "../../lib/cn";
import { EMPHASIS_MARK, resolveEmphasis, shortLabel } from "../frameVisual";
import { TONE_DASH, TONE_TEXT, type DarkTone } from "./darkTone";
import { ElementList, type ElementListItem } from "./ElementList";
import type { PatternViewProps } from "./types";
import type { LaneMessage, LanePattern } from "@chaimir/sim-sdk";

/** 消息状态 → 墨底色阶 */
const MESSAGE_TONE: Record<LaneMessage["status"], DarkTone> = {
  sent: "active",
  delivered: "success",
  dropped: "danger",
};

/** 消息状态词 */
const MESSAGE_STATUS_TEXT: Record<LaneMessage["status"], string> = {
  sent: "已发出",
  delivered: "已送达",
  dropped: "已丢失",
};

/** 画布尺寸:宽度固定,高度按时间跨度伸缩 */
const VIEW_W = 320;
const HEADER_H = 22;
const ROW_H = 26;

export function LanePatternView({
  pattern,
  focus,
  density,
  selectedElementId,
  onSelectElement,
}: PatternViewProps<LanePattern>) {
  const markerBase = useId().replace(/:/g, "");
  const { actors, messages, currentTime } = pattern.data;
  // 消息按时间升序展示:时序图的可读性来自时间单调
  const ordered = [...messages].sort((left, right) => left.at - right.at);

  const items: ElementListItem[] = ordered.map((message) => ({
    id: message.id,
    label: `t${message.at} ${message.from} → ${message.to}:${message.label}`,
    tone: MESSAGE_TONE[message.status],
    statusText: MESSAGE_STATUS_TEXT[message.status],
    detail: message.detail ?? message.meta?.explanation,
    emphasis: resolveEmphasis(message.id, focus, message.meta),
    progress: message.process?.progress,
    progressLabel: message.process?.label,
    lifecycle: message.meta?.lifecycle.state,
  }));

  const list = (
    <ElementList
      label={`${pattern.title} 消息`}
      items={items}
      selectedElementId={selectedElementId}
      onSelectElement={onSelectElement}
    />
  );

  if (density === "panel" || actors.length === 0) {
    return list;
  }

  // 泳道横坐标:消息收发方必是已声明参与方(sim-sdk 校验保证),按声明顺序均分画布宽度
  const laneX = (actor: string): number => ((actors.indexOf(actor) + 1) / (actors.length + 1)) * VIEW_W;
  const viewH = HEADER_H + Math.max(ordered.length, 1) * ROW_H + 12;
  const rowY = (index: number): number => HEADER_H + index * ROW_H + ROW_H / 2;

  return (
    <div className="flex flex-col gap-3">
      <svg
        viewBox={`0 0 ${VIEW_W} ${viewH}`}
        preserveAspectRatio="xMidYMin meet"
        role="presentation"
        className="w-full"
      >
        <defs>
          {(Object.keys(TONE_DASH) as DarkTone[]).map((tone) => (
            <marker
              key={tone}
              id={`${markerBase}-lane-arrow-${tone}`}
              viewBox="0 0 8 8"
              refX={7}
              refY={4}
              markerWidth={5}
              markerHeight={5}
              orient="auto-start-reverse"
            >
              <path d="M0,0 L8,4 L0,8 z" className={cn("fill-current", TONE_TEXT[tone])} />
            </marker>
          ))}
        </defs>

        {/* 泳道:参与者名 + 纵向生命线 */}
        {actors.map((actor) => (
          <g key={actor} className="text-on-dark-sub">
            <text x={laneX(actor)} y={12} textAnchor="middle" fontSize={10} fill="currentColor">
              {shortLabel(actor, 8)}
            </text>
            <line
              x1={laneX(actor)}
              y1={HEADER_H}
              x2={laneX(actor)}
              y2={viewH - 6}
              stroke="currentColor"
              strokeWidth={1}
              strokeDasharray="2 4"
            />
          </g>
        ))}

        {/* 消息箭头:每条消息占一行,y 表示时序位置 */}
        {ordered.map((message, index) => {
          const tone = MESSAGE_TONE[message.status];
          const emphasis = resolveEmphasis(message.id, focus, message.meta);
          const y = rowY(index);
          const x1 = laneX(message.from);
          const x2 = laneX(message.to);
          return (
            <g key={message.id} className={cn(TONE_TEXT[tone], EMPHASIS_MARK[emphasis])}>
              <line
                x1={x1}
                y1={y}
                x2={x2}
                y2={y}
                stroke="currentColor"
                strokeWidth={emphasis === "focus" ? 2 : 1.2}
                strokeDasharray={TONE_DASH[tone]}
                markerEnd={`url(#${markerBase}-lane-arrow-${tone})`}
              />
              <text
                x={(x1 + x2) / 2}
                y={y - 4}
                textAnchor="middle"
                fontSize={9}
                fill="currentColor"
              >
                {shortLabel(message.label, 12)}
              </text>
            </g>
          );
        })}

        {/* 当前时间标记:落在「已到时间的消息」之后,表示时间游标位置 */}
        {(() => {
          const cursorY = rowY(ordered.filter((message) => message.at <= currentTime).length) - ROW_H / 2;
          return (
            <g className="text-accent">
              <line x1={0} y1={cursorY} x2={VIEW_W} y2={cursorY} stroke="currentColor" strokeWidth={1} />
              <text x={2} y={cursorY - 3} fontSize={9} fill="currentColor">
                当前 t{currentTime}
              </text>
            </g>
          );
        })()}
      </svg>
      {list}
    </div>
  );
}
