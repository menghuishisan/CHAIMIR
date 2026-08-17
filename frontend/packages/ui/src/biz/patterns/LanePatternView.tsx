/**
 * LanePatternView:时序泳道视图(lane 模式)。
 * 参与者为纵向泳道,时间自上而下,消息为跨泳道的箭头,当前时间用横向标记线。
 *
 * 只画最近一窗消息:泳道高度原本随消息条数线性增长,消息一多整张图就被拉成长条,
 * 舞台放不下也读不清。完整的消息历史在右侧事件流里逐条给全(§7.1 状态与事件分流),
 * 故这里的窗口不造成信息丢失 —— 泳道负责「此刻谁在跟谁说话」,事件流负责「一共说过什么」。
 */
import { useId } from "react";
import { cn } from "../../lib/cn";
import { EMPHASIS_MARK, resolveEmphasis, shortLabel } from "../frameVisual";
import { TONE_DASH, TONE_TEXT, type DarkTone } from "./darkTone";
import { MESSAGE_TONE } from "./elementStatus";
import { PatternFrame } from "./PatternFrame";
import type { PatternViewProps } from "./types";
import type { LanePattern } from "@chaimir/sim-sdk";

/** 画布尺寸:宽度固定,高度按窗口内消息条数伸缩 */
const VIEW_W = 320;
const HEADER_H = 22;
const ROW_H = 26;

/** 泳道窗口:最多同时画这么多条消息(超出部分由右侧事件流承载) */
const LANE_WINDOW = 12;

export function LanePatternView({
  pattern,
  focus,
  density,
  selectedElementId,
}: PatternViewProps<LanePattern>) {
  const markerBase = useId().replace(/:/g, "");
  const { actors, messages, currentTime } = pattern.data;
  // 消息按时间升序:时序图的可读性来自时间单调
  const ordered = [...messages].sort((left, right) => left.at - right.at);
  // 窗口取最新一段:学生盯的是刚发生的事,历史在事件流里回看
  const windowed = ordered.slice(Math.max(0, ordered.length - LANE_WINDOW));
  // 参与方缺声明(或本帧还没有消息)时不画空泳道,信息由事件流给全
  if (actors.length === 0 || windowed.length === 0) return null;

  // 泳道横坐标:消息收发方必是已声明参与方(sim-sdk 校验保证),按声明顺序均分画布宽度
  const laneX = (actor: string): number => ((actors.indexOf(actor) + 1) / (actors.length + 1)) * VIEW_W;
  const viewH = HEADER_H + windowed.length * ROW_H + 12;
  const rowY = (index: number): number => HEADER_H + index * ROW_H + ROW_H / 2;
  const hidden = ordered.length - windowed.length;

  return (
    <PatternFrame
      density={density}
      canvas={
        <svg
          viewBox={`0 0 ${VIEW_W} ${viewH}`}
          preserveAspectRatio="xMidYMin meet"
          role="presentation"
          className={cn("w-full", density === "panel" ? undefined : "min-h-0 flex-1")}
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
          {windowed.map((message, index) => {
            const tone = MESSAGE_TONE[message.status];
            const emphasis = resolveEmphasis(message.id, focus, message.meta);
            const y = rowY(index);
            const x1 = laneX(message.from);
            const x2 = laneX(message.to);
            const selected = message.id === selectedElementId;
            return (
              <g key={message.id} className={cn(TONE_TEXT[tone], EMPHASIS_MARK[emphasis])}>
                <line
                  x1={x1}
                  y1={y}
                  x2={x2}
                  y2={y}
                  stroke="currentColor"
                  strokeWidth={selected || emphasis === "focus" ? 2 : 1.2}
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

          {/* 当前时间标记:落在「窗口内已到时间的消息」之后,表示时间游标位置 */}
          {(() => {
            const cursorY = rowY(windowed.filter((message) => message.at <= currentTime).length) - ROW_H / 2;
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
      }
    >
      {hidden > 0 ? (
        <p className="text-xs text-on-dark-sub">
          只画最近 {windowed.length} 条,更早的 {hidden} 条在右侧消息流里按时刻回看。
        </p>
      ) : null}
    </PatternFrame>
  );
}
