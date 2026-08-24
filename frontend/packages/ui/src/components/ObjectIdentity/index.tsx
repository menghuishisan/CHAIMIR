/**
 * ObjectIdentity:对象身份区(规范 §6.5.3 第 ④ 族「详情」)。
 *
 * 详情页开头不该是 KPI 指标带。读者进来是为了核对**一个对象**:它叫什么、处在什么状态、
 * 几个必须一眼看到的属性、以及能对它做什么。这四样是一体的,所以收在同一块抬起片里。
 *
 * 属性用横排而不是 Stat 卡:它们是**属性**不是指标 —— 「服务到期 长期有效」「部署形态 平台托管」
 * 没有量纲也不参与比较,套上 Display 字号的大卡等于用排数字的方式排正文(违反 §2.3)。
 */
import type { ReactNode } from "react";
import { cn } from "../../lib/cn";

export interface ObjectIdentityProperty {
  /** 属性名(用户向) */
  label: string;
  value: ReactNode;
}

export interface ObjectIdentityProps {
  /**
   * 对象名称。**它就是详情页的 `h1`** —— 所以详情族的 `PageHeader` 只出面包屑、不再出标题
   * (§6.5.0 通则 1:不重复说同一件事;一页也只该有一个 h1)。
   */
  name: string;
  /** 状态徽标槽位:传 Badge —— 状态必须带文字,不靠颜色单一传达(FE-2) */
  status?: ReactNode;
  /** 名称下方的次要标识,如短码、编号、所属层级 */
  subtitle?: ReactNode;
  /** 主操作槽位:该对象级别的动作(停用/调整配额/导出) */
  actions?: ReactNode;
  /**
   * 关键属性横排。只放「不看就没法判断」的几项,其余属性进下方分区的 DescriptionList；
   * 超过 6 项时读者已经在扫而不是看,应当下沉。
   */
  properties?: ObjectIdentityProperty[];
  className?: string;
}

export function ObjectIdentity({
  name,
  status,
  subtitle,
  actions,
  properties,
  className,
}: ObjectIdentityProps) {
  return (
    // 抬起片(§6.5.1 第 1 级);对象身份是这一族的主体,给它一块自己的纸
    <section
      className={cn("flex min-w-0 flex-col gap-4 rounded-lg bg-surface p-5 shadow-xs", className)}
    >
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2.5">
            <h1 className="font-display text-2xl text-ink">{name}</h1>
            {status}
          </div>
          {subtitle && <div className="mt-1 text-sm text-ink-sub">{subtitle}</div>}
        </div>
        {actions && <div className="flex shrink-0 flex-wrap items-center gap-2">{actions}</div>}
      </div>

      {properties && properties.length > 0 && (
        // 上边线把身份与属性分层 —— border 用于分隔而非圈盒(§6.5.1)
        <dl className="grid grid-cols-2 gap-x-6 gap-y-3 border-t border-line pt-4 sm:grid-cols-3 lg:grid-cols-5">
          {properties.map((property) => (
            <div key={property.label} className="min-w-0">
              <dt className="text-xs text-ink-sub">{property.label}</dt>
              <dd className="mt-0.5 truncate text-sm font-medium text-ink">{property.value}</dd>
            </div>
          ))}
        </dl>
      )}
    </section>
  );
}
