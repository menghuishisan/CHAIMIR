// authFooter.ts 决定认证页页脚显示什么(设计 §7.1):
// - SaaS 部署时右侧放「平台管理员登录」—— 此前全站没有任何页面链到平台管理员登录页;
//   放在页脚而不是登录卡片里,避免学校管理员误点;私有化部署没有平台管理员,不显示。
// - 在平台管理员登录页上,这个位置换成「学校账号登录」。
// - 左侧「遇到问题?请联系本校管理员」只在面向师生的页面显示;
//   入驻申请页和平台管理员登录页上的人没有「本校管理员」可以找。
// 纯函数,供 AuthLayout 与 node:test 使用。

export interface AuthFooterLink {
  to: string
  label: string
}

export interface AuthFooterContent {
  /** 是否显示「遇到问题?请联系本校管理员」 */
  showSchoolHelp: boolean
  /** 右侧入口;私有化部署为 null */
  link: AuthFooterLink | null
}

const SCHOOL_LOGIN_PATH = '/auth/login'
const PLATFORM_LOGIN_PATH = '/auth/platform-login'

/** 不面向师生的认证页:这些页面上的人没有「本校管理员」 */
const NON_SCHOOL_PATHS = new Set([PLATFORM_LOGIN_PATH, '/auth/apply'])

/** authFooterContent 按当前路径与部署形态给出页脚内容。 */
export function authFooterContent(pathname: string, platformEnabled: boolean): AuthFooterContent {
  const path = pathname.length > 1 ? pathname.replace(/\/+$/, '') : pathname
  const onPlatformLogin = path === PLATFORM_LOGIN_PATH
  let link: AuthFooterLink | null = null
  if (platformEnabled) {
    link = onPlatformLogin
      ? { to: SCHOOL_LOGIN_PATH, label: '学校账号登录' }
      : { to: PLATFORM_LOGIN_PATH, label: '平台管理员登录' }
  }
  return { showSchoolHelp: !NON_SCHOOL_PATHS.has(path), link }
}
