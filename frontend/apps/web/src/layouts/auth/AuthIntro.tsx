// AuthIntro 是认证域左区的平台介绍(桌面宽屏可见,窄屏折叠为顶部品牌行)。
// 这一区对所有认证页可见(登录/忘记密码/入驻申请/激活/SSO 共享),
// 不参与具体认证流程,只回答访客的第一个问题:这是什么平台、能在上面做什么。
// 文案即信息(不是排版素材),故与具体认证页分开维护:改介绍不必碰各页状态机。
// 入场动效为低频页编排(规范 §4.5):品牌显影 → 标题显影 → 段落与能力点阶梯浮现,总时长 ≤1.1s。
// 品牌区不用「落印」:那是品牌章的动效,而章表达「已确权、不可逆」,认证页只是入口,
// 章与落印留给提交成功回执(AuthSuccess)。
// 底层添加玉色径向光晕(规范 §1.2:用光晕、密度、暗化渐变过渡,不用边线/色块分隔)。

import { BookOpen, Braces, Trophy } from 'lucide-react'
import { useTenantBrand } from '../../features/identity/useTenantBrand'
import { AuthBrandMark, SchoolBrandLine } from '../../features/identity/pages/auth/auth-ui'

/** 平台能力点:三条对应三位一体(教学 · 实验 · 竞赛),与首页宣传口径同源 */
const FEATURES = [
  { icon: BookOpen, title: '体系化课程', description: '随堂练习与课程内容一体' },
  { icon: Braces, title: '真实链环境', description: '零配置合约开发与共识仿真沙箱' },
  { icon: Trophy, title: '实战检验', description: '解题赛、链上攻防赛验证学习成果' },
] as const

/**
 * AuthIntro 渲染品牌锁定组合与平台介绍;内容自足,不接受任何入参。
 * 对所有认证页可见(登录/忘记密码/入驻申请/激活/SSO 共享),路由切换时不重新渲染。
 * 学校私有部署会在品牌区下方多一行学校身份 —— 那台部署只服务这一所学校。
 * 与右区表单垂直居中对齐(justify-center),建立视觉锚点。
 */
export function AuthIntro() {
  const brand = useTenantBrand()

  return (
    <section className="auth-intro-glow relative hidden basis-3/5 flex-col justify-center px-16 pb-16 pt-12 lg:flex">
      {/* 品牌锁定组合(规范 §1.3:移到内容区最上方,与整体垂直居中配合) */}
      <div className="mb-12 space-y-5">
        <AuthBrandMark large />
        {brand ? (
          <div className="animate-develop" style={{ animationDelay: 'calc(var(--t-stagger) * 1)' }}>
            <SchoolBrandLine name={brand.display_name} logoSrc={brand.logo_image} />
          </div>
        ) : null}
      </div>

      {/* 大标题 + 价值主张:合并为一个视觉单元,垂直间距收紧 */}
      <div className="max-w-2xl space-y-6">
        <h1
          className="font-display text-6xl font-normal leading-tight text-on-dark animate-develop"
          style={{ animationDelay: 'calc(var(--t-stagger) * 2)' }}
        >
          从课堂，
          <br />
          到链上实战
        </h1>
        <p
          className="max-w-xl text-lg leading-relaxed text-on-dark-sub animate-rise"
          style={{ animationDelay: 'calc(var(--t-stagger) * 3)' }}
        >
          真实链环境 + 零配置实验 + 竞赛对抗，
          <br />
          一个浏览器完成区块链教学全流程
        </p>
      </div>

      {/* 三个特性:紧凑垂直列表,图标用玉色圆底点睛 */}
      <ul className="mt-10 max-w-lg space-y-4">
        {FEATURES.map((feature, index) => (
          <li
            key={feature.title}
            className="flex items-start gap-3 animate-rise"
            style={{ animationDelay: `calc(var(--t-stagger) * ${index + 4})` }}
          >
            <span className="mt-1 flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-accent/10">
              {/* 直接渲染 Lucide 图标，不用 Icon 包装，以完全控制尺寸 */}
              <feature.icon className="h-3.5 w-3.5 text-accent" strokeWidth={1.8} aria-hidden />
            </span>
            <div>
              <p className="text-base font-medium leading-snug text-on-dark">{feature.title}</p>
              <p className="mt-0.5 text-sm leading-relaxed text-on-dark-sub">{feature.description}</p>
            </div>
          </li>
        ))}
      </ul>
    </section>
  )
}
