/**
 * ProgressRing:环形进度(设计 §7.2 课程详情「下一步」卡片:已学完课时 / 全部课时)。
 * 中心放百分数(正文字体粗体),全量未知时不画弧、中心显示「—」;
 * 首次出现时弧线从 0 填充到当前值(设计 §6.10 的三处新增动效之一),减弱动效下直接到位。
 * 无障碍:role="progressbar" + aria-valuenow,配文字标签,不只靠图形。
 */
import { useEffect, useState } from "react";
import { cn } from "../../lib/cn";
import { useReducedMotion } from "../../hooks";

export interface ProgressRingProps {
  /** 已完成数 */
  done: number;
  /** 总数;为 0 或未知时不画弧 */
  total: number | undefined;
  /** 读屏标签,如「课时完成进度」 */
  label: string;
  className?: string;
}

/** SVG 画布与线宽(用户单位,随容器缩放) */
const VIEW = 80;
const STROKE = 7;
const RADIUS = (VIEW - STROKE) / 2;
const CIRCUMFERENCE = 2 * Math.PI * RADIUS;

/** ProgressRing 渲染环形进度;尺寸由 className 给(默认 76px) */
export function ProgressRing({ done, total, label, className }: ProgressRingProps) {
  const reducedMotion = useReducedMotion();
  const known = total !== undefined && total > 0;
  const ratio = known ? Math.min(1, Math.max(0, done / total)) : 0;
  const percent = Math.round(ratio * 100);
  // 首次出现从 0 填充:先渲染空环,下一帧再写入目标值,过渡由 CSS 完成
  const [shown, setShown] = useState(reducedMotion ? ratio : 0);
  useEffect(() => {
    if (reducedMotion) {
      setShown(ratio);
      return;
    }
    const frame = window.requestAnimationFrame(() => setShown(ratio));
    return () => window.cancelAnimationFrame(frame);
  }, [ratio, reducedMotion]);

  return (
    <div
      role="progressbar"
      aria-label={label}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={known ? percent : undefined}
      aria-valuetext={known ? `${done} / ${total}` : "进度未知"}
      className={cn("relative size-19 shrink-0", className)}
    >
      <svg viewBox={`0 0 ${VIEW} ${VIEW}`} className="size-full -rotate-90" aria-hidden="true">
        <circle cx={VIEW / 2} cy={VIEW / 2} r={RADIUS} fill="none" strokeWidth={STROKE} className="stroke-surface-sunken" />
        {known ? (
          <circle
            cx={VIEW / 2}
            cy={VIEW / 2}
            r={RADIUS}
            fill="none"
            strokeWidth={STROKE}
            strokeLinecap="round"
            strokeDasharray={CIRCUMFERENCE}
            strokeDashoffset={CIRCUMFERENCE * (1 - shown)}
            className="stroke-primary transition-ring duration-slow ease-out"
          />
        ) : null}
      </svg>
      <span className="absolute inset-0 flex items-center justify-center text-lg font-bold tabular-nums text-ink">
        {known ? `${percent}%` : "—"}
      </span>
    </div>
  );
}
