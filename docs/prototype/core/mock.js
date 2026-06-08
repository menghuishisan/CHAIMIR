/* ============================================================
   core/mock.js — 原型共享假数据
   ------------------------------------------------------------
   职责:为四端页面提供一致的演示数据(课程/实验/竞赛/账号/题库等)。
        仅用于原型展示,字段命名对齐 docs 数据模型,便于过渡到真实接口。
   ============================================================ */
(function () {
  const C = window.Chaimir = window.Chaimir || {};

  C.mock = {
    me: { name: '林同学', no: '2023210456', class: '区块链 2301 班', dept: '计算机学院', phone: '138****6677' },

    courses: [
      { id: 1, name: '区块链原理与智能合约开发', teacher: '李明远', members: 128, type: '混合', difficulty: '进阶',
        progress: 62, status: '进行中', credits: 3, semester: '2025-2026 春', cover: 'amber' },
      { id: 2, name: 'DeFi 协议开发与套利审计', teacher: '王思齐', members: 86, type: '实验', difficulty: '高级',
        progress: 0, status: '未开始', credits: 2, semester: '2025-2026 春', cover: 'purple' },
      { id: 3, name: '密码学基础与共识算法', teacher: '陈雪', members: 152, type: '理论', difficulty: '入门',
        progress: 100, status: '已结束', credits: 3, semester: '2024-2025 秋', cover: 'blue' },
    ],

    chapters: [
      { title: '第一章 · 区块链与分布式账本', lessons: [
        { title: '1.1 从比特币说起', type: 'video', status: 'done', dur: '18:24' },
        { title: '1.2 哈希与默克尔树', type: 'sim', status: 'done' },
        { title: '1.3 章节测验', type: 'assignment', status: 'done' } ] },
      { title: '第三章 · 共识算法与重入漏洞', lessons: [
        { title: '3.1 PoW 与 PBFT', type: 'video', status: 'doing', dur: '24:10' },
        { title: '3.2 PBFT 共识仿真', type: 'sim', status: 'todo' },
        { title: '3.3 重入漏洞代码实验', type: 'experiment', status: 'todo' },
        { title: '3.4 智能合约安全作业', type: 'assignment', status: 'todo' } ] },
    ],

    experiments: [
      { id: 1, name: 'PoW 挖矿与 51% 算力攻击', stack: 'EVM / Hardhat', kind: 'code', status: '进行中', score: null, checkpoints: 3, passed: 1 },
      { id: 2, name: 'PBFT 拜占庭容错共识交互', stack: '图形仿真', kind: 'sim', status: '已通过', score: 95, checkpoints: 4, passed: 4 },
      { id: 3, name: '重入漏洞利用与防护(CEI)', stack: 'EVM / Foundry', kind: 'code', status: '未开始', score: null, checkpoints: 5, passed: 0 },
      { id: 4, name: '默克尔树与轻节点验证', stack: '前端仿真', kind: 'sim', status: '已完成', score: 88, checkpoints: 3, passed: 3 },
    ],

    contests: [
      { id: 1, name: '「链上夺旗」金库重入渗透赛', mode: '对抗赛', team: '个人', status: '进行中', signup: '已报名', rank: 7, players: 64 },
      { id: 2, name: '智能合约 Gas 优化挑战赛', mode: '解题赛', team: '个人', status: '报名中', signup: '未报名', rank: null, players: 41 },
      { id: 3, name: '跨链桥安全攻防联赛', mode: '对抗赛', team: '团队', status: '已结束', signup: '已参赛', rank: 3, players: 120 },
    ],

    notifications: [
      { id: 1, type: '作业', title: '《智能合约安全作业》将于明天 23:59 截止', read: false, time: '10 分钟前', link: 'student/assignment' },
      { id: 2, type: '竞赛', title: '你在「链上夺旗」赛中的排名上升至第 7 名', read: false, time: '1 小时前', link: 'student/contests' },
      { id: 3, type: '成绩', title: '《PBFT 拜占庭容错共识交互》实验已评分:95 分', read: false, time: '3 小时前', link: 'student/grades' },
      { id: 4, type: '系统', title: '平台将于本周六 02:00-04:00 进行维护', read: true, time: '昨天', link: 'student/announcements' },
    ],

    students: [
      { id: 1, name: '林思远', no: '2023210456', class: '区块链 2301', state: 'ok', stateText: '通过检查点 2/3', cp: 2 },
      { id: 2, name: '赵雨桐', no: '2023210457', class: '区块链 2301', state: 'ok', stateText: '通过检查点 2/3', cp: 2 },
      { id: 3, name: '孙浩然', no: '2023210458', class: '区块链 2301', state: 'warn', stateText: '进度落后', cp: 1 },
      { id: 4, name: '周晓彤', no: '2023210459', class: '区块链 2301', state: 'err', stateText: '编译报错', cp: 1 },
      { id: 5, name: '吴俊杰', no: '2023210460', class: '区块链 2301', state: 'ok', stateText: '已完成', cp: 3 },
      { id: 6, name: '郑梓萱', no: '2023210461', class: '区块链 2301', state: 'ok', stateText: '通过检查点 1/3', cp: 1 },
    ],

    accounts: [
      { id: 1, name: '李明远', role: '教师', no: 'T2019033', dept: '计算机学院', status: '正常', login: '2 小时前' },
      { id: 2, name: '王思齐', role: '教师', no: 'T2020118', dept: '网络空间安全学院', status: '正常', login: '昨天' },
      { id: 3, name: '林思远', role: '学生', no: '2023210456', dept: '区块链 2301', status: '正常', login: '10 分钟前' },
      { id: 4, name: '赵雨桐', role: '学生', no: '2023210457', dept: '区块链 2301', status: '正常', login: '1 天前' },
      { id: 5, name: '陈旧账号', role: '学生', no: '2019210001', dept: '已毕业', status: '已归档', login: '180 天前' },
    ],

    tenants: [
      { id: 1, name: '示例大学', code: 'demo-univ', users: 3280, status: '正常', expire: '2027-08-31' },
      { id: 2, name: '滨海理工大学', code: 'bhit', users: 1560, status: '正常', expire: '2026-12-31' },
      { id: 3, name: '云岭师范学院', code: 'ylnu', users: 0, status: '停用', expire: '2026-06-30' },
    ],

    applications: [
      { id: 1, school: '江南科技大学', type: '本科', contact: '教务处 / 周老师', phone: '139****2200', time: '2026-06-05', status: '待审' },
      { id: 2, school: '海川职业技术学院', type: '高职', contact: '信息中心 / 吴主任', phone: '137****8841', time: '2026-06-03', status: '待审' },
      { id: 3, school: '北辰大学', type: '本科', contact: '实验室 / 钱老师', phone: '135****0099', time: '2026-06-01', status: '待审' },
    ],

    runtimes: [
      { id: 1, code: 'evm-hardhat', name: 'EVM · Hardhat', selftest: '通过', img: 'v2.22.1', nodes: '6/6', def: true },
      { id: 2, code: 'evm-foundry', name: 'EVM · Foundry', selftest: '通过', img: 'v0.2.0', nodes: '6/6', def: false },
      { id: 3, code: 'fabric', name: 'Hyperledger Fabric', selftest: '预拉取中', img: 'v2.5.4', nodes: '3/6', def: false },
    ],

    content: [
      { id: 1, title: '重入漏洞利用与防护', type: '实验模板', difficulty: '进阶', author: '李明远', version: 'v1.3.0', status: '已发布', usage: 12 },
      { id: 2, title: '金库重入渗透 CTF', type: '竞赛题', difficulty: '高级', author: '系统入库', version: 'v1.0.0', status: '已发布', usage: 5 },
      { id: 3, title: 'PBFT 三阶段共识选择题组', type: '理论题', difficulty: '入门', author: '陈雪', version: 'v2.0.0', status: '草稿', usage: 0 },
    ],

    /* 对抗赛对局回放:交易序列 */
    battleLog: [
      { h: 1024, kind: 'deploy', text: '守方发布资产金库合约,开启 nonReentrant 重入防护排他锁。' },
      { h: 1025, kind: 'attack', text: '攻方存入 1 ETH,触发渗透函数,试图通过递归 receive 掏空金库。' },
      { h: 1026, kind: 'defend', text: '虚拟机捕捉到状态重入深度越界,执行强制 Revert 拦截!' },
      { h: 1027, kind: 'settle', text: '对局结束:攻方被完全拦截,守方资产零损耗,天梯结算完毕。' },
    ],
    ladder: [
      { rank: 1, name: '影梭战队', elo: 2148, win: 18, lose: 2 },
      { rank: 2, name: 'ZeroDay', elo: 2090, win: 16, lose: 4 },
      { rank: 3, name: '拜占庭幻象', elo: 2033, win: 15, lose: 5 },
      { rank: 7, name: '林思远(你)', elo: 1820, win: 9, lose: 6, me: true },
    ],
  };
})();
