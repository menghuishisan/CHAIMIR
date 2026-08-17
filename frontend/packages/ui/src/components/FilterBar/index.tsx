/**
 * FilterBar:资源页筛选区的唯一形态(规范 §6.5.2)。
 *
 * 为什么需要它:此前筛选控件都塞在 `PageSection actions` 里,分段控件、下拉、日期、搜索框、按钮
 * 各有各的高度与基线,被挤成一条长横条,窄屏还与分组标题抢空间 —— 这是「布局很乱」的主要来源。
 * FilterBar 把筛选从分组标题里搬出来,统一成「每项一个标签在控件上方、控件等高、按需折行」。
 *
 * 两类字段共用同一条:即时生效的(下拉/分段,自己的 onChange 就是筛选)与需要确认的
 * (搜索词、日期区间)。后者传 onSubmit,组件整体渲染为 form 并在末位给出提交按钮 ——
 * 于是在输入框里按回车就等于点按钮,不必每个页面自己再包一层 form。
 *
 * 层次上它是抬起片内部的凹陷井(§6.5.1 第 2 级),不画边框。
 */
import type { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import { Search } from "lucide-react";
import { cn } from "../../lib/cn";
import { Button } from "../Button";

/**
 * 一个筛选项的公共属性。
 * 单个表单控件(Select / Input / date)用 `htmlFor` 绑定 `label`;
 * 一组按钮(SegmentedControl)用 `group` 改走 fieldset/legend ——
 * `label[for]` 指向 radiogroup 这类非可标注元素是无效标记,读屏也不一定关联得上。
 */
export type FilterFieldProps = {
  /** 筛选项标签(用户向,说明这一项在筛什么) */
  label: string;
  children: ReactNode;
  className?: string;
} & ({ htmlFor: string; group?: false } | { group: true; htmlFor?: never });

/** 筛选项外壳:标签在上、控件在下,窄屏可折行 */
const FIELD_CLASS = "flex min-w-0 flex-col gap-1";
/** 标签排版:比正文小一档,不与控件抢注意力 */
const LABEL_CLASS = "text-xs text-ink-sub";

/**
 * FilterField:一个筛选项 —— 标签在上、控件在下。
 * 标签始终可见:只给分段控件配 aria-label 会让视觉用户看不出这排按钮在筛什么。
 */
export function FilterField({ label, htmlFor, group, children, className }: FilterFieldProps) {
  if (group) {
    return (
      <fieldset className={cn(FIELD_CLASS, className)}>
        <legend className={LABEL_CLASS}>{label}</legend>
        {children}
      </fieldset>
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

/** 筛选条内部排布:控件底对齐、按需折行 */
const BAR_CLASS = "well flex flex-wrap items-end gap-x-4 gap-y-3 p-3";

/**
 * FilterBar 渲染一条筛选区。
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
      <section aria-label={label} className={cn(BAR_CLASS, className)}>
        {children}
        {tail}
      </section>
    );
  }

  return (
    <form
      aria-label={label}
      className={cn(BAR_CLASS, className)}
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
