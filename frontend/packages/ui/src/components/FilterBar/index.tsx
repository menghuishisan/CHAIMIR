/**
 * FilterBar:列表上方的一行工具条(规范 §6.5.2)。
 *
 * 状态分段、下拉、搜索框排成一行,窄屏折行;无底色 —— 表格型列表放进 DataPanel 的 filter 槽位
 * (槽位给内距与下分隔线),卡片网格型列表直接排在纸面上。
 * 每项的可见标签与控件同行排在左侧(FilterField),视觉用户与读屏都知道这一项在筛什么。
 *
 * 两类字段共用同一条:即时生效的(下拉/分段,自己的 onChange 就是筛选)与需要确认的
 * (搜索词、日期区间)。后者传 onSubmit,组件整体渲染为 form 并在末位给出提交按钮 ——
 * 于是在输入框里按回车就等于点按钮,不必每个页面自己再包一层 form。
 */
import { useId, type ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import { Search } from "lucide-react";
import { cn } from "../../lib/cn";
import { Button } from "../Button";

/**
 * 一个筛选项的公共属性。
 * 单个表单控件(Select / Input / date)用 `htmlFor` 绑定 `label`;
 * 一组按钮(SegmentedControl)用 `group` 改走 role="group" + aria-labelledby ——
 * `label[for]` 指向 radiogroup 这类非可标注元素是无效标记,读屏也不一定关联得上。
 */
export type FilterFieldProps = {
  /** 筛选项标签(用户向,说明这一项在筛什么) */
  label: string;
  children: ReactNode;
  className?: string;
} & ({ htmlFor: string; group?: false } | { group: true; htmlFor?: never });

/** 筛选项外壳:标签在左、控件在右,同一行 */
const FIELD_CLASS = "flex min-w-0 items-center gap-2";
/** 标签排版:次要色小一档,不与控件抢注意力 */
const LABEL_CLASS = "shrink-0 text-sm text-ink-sub";

/**
 * FilterField:一个筛选项 —— 标签在左、控件在右。
 * 标签始终可见:只给分段控件配 aria-label 会让视觉用户看不出这排按钮在筛什么。
 */
export function FilterField({ label, htmlFor, group, children, className }: FilterFieldProps) {
  const labelId = useId();
  if (group) {
    return (
      <div role="group" aria-labelledby={labelId} className={cn(FIELD_CLASS, className)}>
        <span id={labelId} className={LABEL_CLASS}>
          {label}
        </span>
        {children}
      </div>
    );
  }
  return (
    <div className={cn(FIELD_CLASS, className)}>
      <label htmlFor={htmlFor} className={LABEL_CLASS}>
        {label}
      </label>
      {children}
    </div>
  );
}

export interface FilterBarProps {
  /** 无障碍名称,如「账号筛选」;读屏据此说明这一组控件的用途 */
  label: string;
  /** 筛选项:用 FilterField 包裹的控件,按需折行 */
  children: ReactNode;
  /** 需要确认才生效的字段(搜索词/日期区间)传它:整条渲染为 form,末位出提交按钮,回车即提交 */
  onSubmit?: () => void;
  /** 提交按钮文案,默认「查询」 */
  submitLabel?: string;
  /** 提交按钮图标,默认放大镜 */
  submitIcon?: LucideIcon;
  /** 提交进行中 */
  submitting?: boolean;
  /** 重置(可选):排在提交之后,由页面决定清掉哪些状态 */
  onReset?: () => void;
  className?: string;
}

/** 工具条排布:一行、垂直居中、按需折行;无底色无内距(内距由所在容器给) */
const BAR_LAYOUT = "flex flex-wrap items-center gap-x-4 gap-y-3";

/**
 * FilterBar 渲染一行工具条。
 * 传 onSubmit 时是 form(回车提交),否则是普通分区 —— 没有需要确认的字段就不该出现提交按钮。
 */
export function FilterBar({
  label,
  children,
  onSubmit,
  submitLabel = "查询",
  submitIcon = Search,
  submitting = false,
  onReset,
  className,
}: FilterBarProps) {
  const barClass = cn(BAR_LAYOUT, className);
  const tail = (
    <>
      {onSubmit && (
        <Button type="submit" variant="outline" leftIcon={submitIcon} loading={submitting}>
          {submitLabel}
        </Button>
      )}
      {onReset && (
        <Button type="button" variant="ghost" onClick={onReset}>
          重置筛选
        </Button>
      )}
    </>
  );
  if (!onSubmit) {
    return (
      <section aria-label={label} className={barClass}>
        {children}
        {tail}
      </section>
    );
  }
  return (
    <form
      aria-label={label}
      className={barClass}
      onSubmit={(event) => {
        event.preventDefault();
        onSubmit();
      }}
    >
      {children}
      {tail}
    </form>
  );
}
