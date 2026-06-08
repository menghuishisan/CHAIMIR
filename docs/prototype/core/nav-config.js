/* ============================================================
   core/nav-config.js — 四端信息架构(导航 + 角色元数据)
   ------------------------------------------------------------
   职责:定义四个角色端的侧栏菜单分组与路由映射、默认落点、
        以及登录前页面与沉浸式工作台的清单(供原型导航与外壳使用)。
   说明:登录后按角色直达侧栏第一个功能页(总纲约定 §8)。
        私有化部署无 platform 端(此处保留,原型可整端预览)。
   菜单项格式:[routeKey, lucideIcon, label, badgeCount?]
   ============================================================ */
(function () {
  const Chaimir = window.Chaimir = window.Chaimir || {};

  /* 角色元数据 */
  Chaimir.roles = {
    student: { label: '学生', icon: 'graduation-cap', home: 'student/courses' },
    teacher: { label: '教师', icon: 'presentation', home: 'teacher/courses' },
    'school-admin': { label: '学校管理员', icon: 'building', home: 'school-admin/dashboard' },
    'platform-admin': { label: '平台管理员', icon: 'shield', home: 'platform-admin/tenants' },
  };

  /* 四端侧栏导航 */
  Chaimir.nav = {
    student: [
      { group: '学习', items: [
        ['student/courses', 'book-open', '我的课程'],
        ['student/experiments', 'flask-conical', '我的实验'],
        ['student/contests', 'trophy', '我的竞赛'],
        ['student/sim-lib', 'activity', '仿真实验室'],
      ]},
      { group: '成绩', items: [
        ['student/grades', 'bar-chart-3', '我的成绩'],
        ['student/warnings', 'alert-triangle', '学业预警', 1],
      ]},
      { group: '账户', items: [
        ['student/profile', 'user', '个人中心'],
      ]},
    ],
    teacher: [
      { group: '教学', items: [
        ['teacher/courses', 'book-open', '课程管理'],
        ['teacher/grading', 'check-square', '批改中心', 5],
      ]},
      { group: '实践', items: [
        ['teacher/experiments', 'flask-conical', '实验管理'],
        ['teacher/contests', 'trophy', '竞赛管理'],
        ['teacher/monitor', 'monitor', '实时监控'],
      ]},
      { group: '资源', items: [
        ['teacher/content', 'library', '题库'],
        ['teacher/papers', 'file-stack', '组卷'],
        ['teacher/sim-packages', 'activity', '仿真场景'],
        ['teacher/shared-lib', 'share-2', '共享库'],
      ]},
      { group: '成绩', items: [
        ['teacher/grade-submit', 'send', '成绩报送'],
      ]},
      { group: '组织', items: [
        ['teacher/org', 'network', '组织架构'],
      ]},
      { group: '账户', items: [
        ['teacher/profile', 'user', '个人中心'],
      ]},
    ],
    'school-admin': [
      { group: '概览', items: [
        ['school-admin/dashboard', 'layout-dashboard', '数据看板'],
        ['school-admin/statistics', 'line-chart', '运营统计'],
      ]},
      { group: '用户与组织', items: [
        ['school-admin/accounts', 'users', '用户管理'],
        ['school-admin/org', 'network', '组织架构'],
        ['school-admin/import-batches', 'file-up', '导入记录'],
      ]},
      { group: '成绩', items: [
        ['school-admin/grade-reviews', 'clipboard-check', '成绩审核', 4],
        ['school-admin/appeals', 'gavel', '申诉处理', 2],
        ['school-admin/warnings', 'alert-triangle', '学业预警'],
        ['school-admin/grade-config', 'settings-2', '成绩配置'],
      ]},
      { group: '系统', items: [
        ['school-admin/config', 'settings', '学校配置'],
        ['school-admin/sso', 'key-round', '认证配置'],
        ['school-admin/audit', 'scroll-text', '审计日志'],
        ['school-admin/alerts', 'bell-ring', '告警'],
      ]},
      { group: '账户', items: [
        ['school-admin/profile', 'user', '个人中心'],
      ]},
    ],
    'platform-admin': [
      { group: '租户', items: [
        ['platform-admin/tenants', 'building-2', '学校管理'],
        ['platform-admin/applications', 'clipboard-check', '入驻审核', 3],
      ]},
      { group: '概览', items: [
        ['platform-admin/dashboard', 'layout-dashboard', '平台看板'],
        ['platform-admin/statistics', 'line-chart', '平台统计'],
      ]},
      { group: '引擎', items: [
        ['platform-admin/runtimes', 'boxes', '运行时管理'],
        ['platform-admin/tools', 'wrench', '工具管理'],
        ['platform-admin/judgers', 'scale', '判题器管理'],
        ['platform-admin/sim-review', 'activity', '仿真包审核', 2],
        ['platform-admin/quota', 'gauge', '配额管理'],
      ]},
      { group: '运维', items: [
        ['platform-admin/config', 'settings', '系统配置'],
        ['platform-admin/alerts', 'bell-ring', '告警'],
        ['platform-admin/audit', 'scroll-text', '审计中心'],
        ['platform-admin/monitoring', 'radar', '基础监控'],
        ['platform-admin/backups', 'database-backup', '备份记录'],
      ]},
      { group: '账户', items: [
        ['platform-admin/profile', 'user', '个人中心'],
      ]},
    ],
  };

  /* 登录前页面(原型导航用) */
  Chaimir.authPages = [
    ['auth/login', 'log-in', '统一登录', '手机号/学号/短信智能识别 + 选学校'],
    ['auth/forgot', 'key-round', '找回密码', '短信验证码重置'],
    ['auth/sso', 'building-2', '学校 SSO', 'CAS / LDAP 统一身份'],
    ['auth/apply', 'file-plus-2', '学校入驻申请', '仅 SaaS,提交后平台审核'],
    ['auth/activate', 'badge-check', '激活账号', '激活码 + 自设密码'],
    ['auth/platform-login', 'shield', '平台管理员登录', '独立入口,私有化关闭'],
  ];

  /* 沉浸式工作台(全屏,原型导航用) */
  Chaimir.immersivePages = [
    ['immersive/exp-ide', 'code-2', '代码实验工作台', 'Monaco + 终端 + 检查点判分 + 链上操作'],
    ['immersive/sim', 'activity', '仿真可视化工作台', '图网络/时序泳道/投票矩阵 + 播放控制'],
    ['immersive/battle-replay', 'swords', '对抗赛对局回放', '攻防拓扑 + 日志流 + 时间轴回溯'],
    ['immersive/solve', 'terminal', '解题赛答题', '题面 + 沙箱环境 + 提交判定'],
  ];
})();
