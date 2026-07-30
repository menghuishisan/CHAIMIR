// login-intro 是登录页左区的平台介绍(桌面宽屏可见,窄屏折叠为顶部品牌行)。
// 这一区不参与登录流程,只回答访客的第一个问题:这是什么平台、能在上面做什么。
// 文案即信息(不是排版素材),故与登录表单分开维护:改介绍不必碰登录状态机。
// 入场动效为低频页编排(规范 §4.5):印章落下 → 标题显影 → 段落与能力点阶梯浮现,总时长 ≤1.1s。

import { BookOpen, Braces, Trophy } from 'lucide-react'
import { Icon } from '@chaimir/ui'
import { AuthBrandMark } from './auth-ui'

/** 平台能力点:三条对应三位一体(教学 · 实验 · 竞赛),与首页宣传口径同源 */
const FEATURES = [
  { icon: BookOpen, title: '体系化课程', description: '随堂练习与课程内容一体,学练不分离' },
  { icon: Braces, title: '真实实验环境', description: '打开即用的合约开发与共识仿真沙箱' },
  { icon: Trophy, title: '竞赛与对抗', description: '解题赛、链上攻防赛,实战检验学习成果' },
] as const

/**
 * LoginIntro 渲染品牌章与平台介绍;内容自足,不接受任何入参。
 */
export function LoginIntro() {
  return (
    <section className="relative hidden w-3/5 flex-col px-14 pb-16 pt-12 lg:flex">
      <AuthBrandMark large />
      <div className="mt-auto max-w-lg">
        <h2
          className="font-display text-4xl font-normal leading-tight text-on-dark animate-develop"
          style={{ animationDelay: 'calc(var(--t-stagger) * 1)' }}
        >
          从课堂,
          <br />
          到<span className="text-accent">链上实战</span>
        </h2>
        <p
          className="mt-4 max-w-md text-md text-on-dark-sub animate-rise"
          style={{ animationDelay: 'calc(var(--t-stagger) * 3)' }}
        >
          面向高校的区块链一体化教学平台。课程、实验环境、竞赛对抗全部在浏览器中完成,无需任何本地配置。
        </p>
        <ul className="mt-8 flex flex-col gap-4">
          {FEATURES.map((feature, index) => (
            <li
              key={feature.title}
              className="flex items-start gap-3.5 animate-rise"
              style={{ animationDelay: `calc(var(--t-stagger) * ${index + 4})` }}
            >
              <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border border-jade-400/20 bg-jade-400/10">
                <Icon icon={feature.icon} size="sm" className="text-accent" />
              </span>
              <span>
                <span className="block text-base font-semibold text-on-dark">{feature.title}</span>
                <span className="block text-sm text-on-dark-sub">{feature.description}</span>
              </span>
            </li>
          ))}
        </ul>
      </div>
    </section>
  )
}
