/* ============================================================
   Chaimir 原型 — 应用核心 (SPA 外壳 + 路由 + 页面)
   ============================================================ */

const qs = new URLSearchParams(location.search);
let ROLE = qs.get('role') || 'student';

// ---------- 角色配置 ----------
const ROLES = {
  student:  { name:'张同学', short:'张', label:'学生 · 北京大学', home:'courses' },
  teacher:  { name:'李老师', short:'李', label:'教师 · 北京大学', home:'t-courses' },
  school:   { name:'李老师', short:'李', label:'学校管理员 · 北京大学', home:'s-users' },
  platform: { name:'王管理', short:'王', label:'平台管理员', home:'p-schools' },
};

// 侧栏菜单(按角色)
const MENUS = {
  student: [
    { group:'学习', items:[
      ['courses','book-open','我的课程'],
      ['experiments','flask-conical','我的实验'],
      ['contests','trophy','我的竞赛'],
      ['grades','award','我的成绩'],
    ]},
    { group:'账户', items:[ ['profile','user','个人中心'] ]},
  ],
  teacher: [
    { group:'教学', items:[
      ['t-courses','book-open','课程管理'],
      ['t-grade-review','clipboard-check','批改中心'],
    ]},
    { group:'实践', items:[
      ['t-experiments','flask-conical','实验管理'],
      ['t-contests','trophy','竞赛管理'],
      ['t-monitor','monitor-dot','学生监控'],
    ]},
    { group:'资源', items:[
      ['t-content','library','题库'],
      ['t-sim','box','仿真场景'],
    ]},
  ],
  school: [
    { group:'管理', items:[
      ['s-users','users','用户管理'],
      ['s-org','network','组织架构'],
      ['s-grade-audit','file-check','成绩审核'],
    ]},
    { group:'配置', items:[
      ['s-config','settings','学校配置'],
      ['s-dashboard','layout-dashboard','数据看板'],
    ]},
  ],
  platform: [
    { group:'租户', items:[
      ['p-schools','building-2','学校管理'],
      ['p-applications','inbox','入驻审核'],
    ]},
    { group:'引擎', items:[
      ['p-runtimes','cpu','运行时管理'],
      ['p-judgers','scale','判题器管理'],
      ['p-sim-lib','box','仿真场景库'],
      ['p-vuln','shield-alert','漏洞源'],
    ]},
    { group:'运维', items:[
      ['p-config','settings','系统配置'],
      ['p-alerts','bell-ring','告警'],
      ['p-audit','scroll-text','审计中心'],
      ['p-dashboard','layout-dashboard','数据看板'],
    ]},
  ],
};

let currentPage = null;

// ---------- 初始化 ----------
function init(){
  const r = ROLES[ROLE];
  document.getElementById('avaText').textContent = r.short;
  document.getElementById('avaName').textContent = r.name;
  document.getElementById('dropName').textContent = r.name;
  document.getElementById('dropRole').textContent = r.label;
  renderRoleSwitch();
  renderSidebar();
  navigate(r.home);
  if(qs.get('immersive')) enterImmersive(qs.get('immersive'));
  lucide.createIcons();
}

// 角色切换胶囊(演示用:教师可切学校管理员;此处提供全角色快速切换)
function renderRoleSwitch(){
  const map = { student:'学生', teacher:'教师', school:'校管', platform:'平台' };
  document.getElementById('roleSwitch').innerHTML = Object.keys(map).map(k=>
    `<button class="${k===ROLE?'active':''}" onclick="switchRole('${k}')">${map[k]}</button>`
  ).join('');
}
function switchRole(k){ location.href = 'app.html?role='+k; }

// ---------- 侧栏 ----------
function renderSidebar(){
  const menu = MENUS[ROLE];
  document.getElementById('sidebar').innerHTML = menu.map(g=>`
    <div class="side-group">
      <div class="side-group-title">${g.group}</div>
      ${g.items.map(([id,icon,label])=>`
        <a class="side-item" data-page="${id}" onclick="navigate('${id}')">
          <i data-lucide="${icon}"></i><span>${label}</span>
        </a>`).join('')}
    </div>`).join('');
  lucide.createIcons();
}

// ---------- 路由 ----------
function navigate(page){
  currentPage = page;
  document.querySelectorAll('.side-item').forEach(el=>
    el.classList.toggle('active', el.dataset.page===page));
  const fn = PAGES[page];
  document.getElementById('content').innerHTML = fn ? fn() : emptyPage(page);
  document.getElementById('content').scrollTop = 0;
  lucide.createIcons();
  bindPageEvents && bindPageEvents();
}
function emptyPage(p){ return `<div class="content-head"><div><div class="page-title">${p}</div><div class="page-sub">页面建设中</div></div></div>`; }

// ---------- 顶栏面板 ----------
function togglePanel(which){
  const np = document.getElementById('notifyPanel');
  const ad = document.getElementById('avatarDrop');
  if(which==='notify'){ np.classList.toggle('open'); ad.classList.remove('open'); }
  else { ad.classList.toggle('open'); np.classList.remove('open'); }
}
document.addEventListener('click', e=>{
  if(!e.target.closest('#bellBtn') && !e.target.closest('#notifyPanel')) document.getElementById('notifyPanel').classList.remove('open');
  if(!e.target.closest('#avatarDrop')) document.getElementById('avatarDrop').classList.remove('open');
});

// ---------- 沉浸模态 ----------
function enterImmersive(type){
  const ws = document.getElementById('workspace');
  const cfg = IMMERSIVE[type];
  if(!cfg) return;
  document.getElementById('imTaskName').textContent = cfg.title;
  document.getElementById('imChip').textContent = cfg.chip || '';
  document.getElementById('imChip').style.display = cfg.chip?'inline':'none';
  document.getElementById('imAction').textContent = cfg.action || '提交';
  ws.innerHTML = cfg.render();
  document.getElementById('immersive').classList.add('open');
  lucide.createIcons();
  cfg.after && cfg.after();
}
function exitImmersive(){ document.getElementById('immersive').classList.remove('open'); }

// 页面与沉浸内容容器(由后续脚本填充)
const PAGES = {};
const IMMERSIVE = {};
let bindPageEvents = null;

// 小工具
function head(title, sub, actions){
  return `<div class="content-head">
    <div><div class="breadcrumb">${sub||''}</div><div class="page-title">${title}</div></div>
    ${actions?`<div class="content-actions">${actions}</div>`:''}
  </div>`;
}
function statCard(icon, num, label, color){
  return `<div class="card card-pad flex items-center gap-4">
    <div class="stat-icon" style="background:${color}22"><i data-lucide="${icon}" style="color:${color};width:22px;height:22px"></i></div>
    <div class="stat"><span class="num">${num}</span><span class="label">${label}</span></div>
  </div>`;
}

/* ============================================================
   学生页面
   ============================================================ */

// 我的课程
PAGES['courses'] = () => head('我的课程','学习') + `
  <div class="card card-pad mb-4" style="background:linear-gradient(120deg,var(--slate-900),var(--slate-800));color:#fff;border:none">
    <div class="flex items-center justify-between">
      <div>
        <div class="text-sm" style="color:var(--amber-400)">继续学习</div>
        <div style="font-size:18px;font-weight:700;margin:4px 0">区块链原理与智能合约</div>
        <div class="text-sm" style="color:var(--slate-400)">第 3 章 · 共识机制 — 已完成 62%</div>
      </div>
      <button class="btn btn-primary" onclick="navigate('course-detail')">继续 <i data-lucide="arrow-right" style="width:16px;height:16px"></i></button>
    </div>
  </div>
  <div class="flex items-center justify-between mb-3"><div class="fw-600">全部课程</div>
    <div class="input-icon" style="width:220px"><i data-lucide="search"></i><input class="input" placeholder="搜索课程"></div></div>
  <div class="grid grid-3">
    ${[['区块链原理与智能合约','李老师','进行中','62'],['DeFi 协议开发实战','王老师','进行中','30'],['密码学基础','张老师','已结束','100'],['Hyperledger Fabric 联盟链','陈老师','进行中','15'],['Web3 前端开发','刘老师','未开始','0'],['智能合约安全审计','赵老师','进行中','45']]
      .map(c=>`<div class="card card-hover card-pad" onclick="navigate('course-detail')">
        <div class="flex items-center justify-between mb-3">
          <div class="stat-icon" style="background:var(--amber-100)"><i data-lucide="book-open" style="color:var(--amber-600);width:20px;height:20px"></i></div>
          <span class="badge ${c[2]==='进行中'?'badge-green':c[2]==='已结束'?'badge-gray':'badge-blue'}">${c[2]}</span>
        </div>
        <div class="fw-600" style="font-size:15px">${c[0]}</div>
        <div class="text-sm muted mt-2 flex items-center gap-2"><i data-lucide="user" style="width:14px;height:14px"></i>${c[1]}</div>
        <div class="progress mt-3"><span style="width:${c[3]}%"></span></div>
        <div class="text-xs muted mt-2">学习进度 ${c[3]}%</div>
      </div>`).join('')}
  </div>`;

// 课程详情
PAGES['course-detail'] = () => head('区块链原理与智能合约','<a onclick="navigate(\'courses\')">我的课程</a> <span class="sep">/</span> 课程详情') + `
  <div class="tabs">
    <div class="tab active">章节内容</div><div class="tab">作业测验</div><div class="tab">讨论区</div><div class="tab">公告</div><div class="tab">成绩</div>
  </div>
  <div class="grid" style="grid-template-columns:1fr 300px">
    <div>
      ${[['第一章 区块链概述',['什么是区块链','去中心化与共识','区块链发展史'],'done'],
         ['第二章 密码学基础',['哈希函数','非对称加密','默克尔树'],'done'],
         ['第三章 共识机制',['PoW 工作量证明','PoS 权益证明','PBFT 实战实验'],'doing']]
        .map((ch,ci)=>`<div class="card card-pad mb-3">
          <div class="fw-600 mb-3 flex items-center gap-2"><i data-lucide="${ch[2]==='done'?'check-circle-2':'circle-dot'}" style="width:18px;height:18px;color:${ch[2]==='done'?'var(--success)':'var(--amber)'}"></i>${ch[0]}</div>
          ${ch[1].map((l,li)=>{
            const isExp = l.includes('实验');
            return `<div class="flex items-center justify-between" style="padding:9px 0;border-top:1px solid var(--slate-100)">
              <div class="flex items-center gap-3 text-sm"><i data-lucide="${isExp?'flask-conical':'play-circle'}" style="width:17px;height:17px;color:var(--slate-400)"></i>${l}</div>
              ${isExp?`<button class="btn btn-primary btn-sm" onclick="enterImmersive('experiment')">开始实验</button>`
                     :`<span class="badge ${ci<2?'badge-green':'badge-gray'}">${ci<2?'已学':'未学'}</span>`}
            </div>`}).join('')}
        </div>`).join('')}
    </div>
    <div>
      <div class="card card-pad mb-3"><div class="fw-600 mb-3">课程信息</div>
        <div class="text-sm muted" style="line-height:2">授课教师:李老师<br>学分:3.0<br>学期:2025-2026 秋<br>已选 128 人</div></div>
      <div class="card card-pad"><div class="fw-600 mb-3">待办</div>
        <div class="cp-card pending mb-3" style="border-color:var(--amber)"><span class="cp-status" style="background:var(--amber-100);color:var(--amber-600)"><i data-lucide="clock" style="width:13px;height:13px"></i></span><div><div class="fw-600">作业3:实现ERC20</div><div class="text-xs muted">2小时后截止</div></div></div>
      </div>
    </div>
  </div>`;

// 我的实验
PAGES['experiments'] = () => head('我的实验','学习') + `
  <div class="grid grid-3 mb-4">
    ${statCard('flask-conical','12','总实验数','#F59E0B')}
    ${statCard('check-circle-2','8','已完成','#10B981')}
    ${statCard('loader','2','进行中','#3B82F6')}
  </div>
  <table class="table card" style="overflow:hidden">
    <thead><tr><th>实验名称</th><th>类型</th><th>所属课程</th><th>状态</th><th>得分</th><th></th></tr></thead>
    <tbody>
    ${[['PoW 挖矿与51%攻击','沙箱+仿真','区块链原理','进行中','—'],
       ['部署你的第一个 ERC20','沙箱','区块链原理','已完成','95'],
       ['默克尔树构建与验证','仿真','密码学基础','已完成','100'],
       ['Fabric 联盟链组网','多人沙箱','联盟链','进行中','—'],
       ['重入攻击复现','沙箱','安全审计','未开始','—']]
      .map(e=>`<tr>
        <td class="fw-600">${e[0]}</td>
        <td><span class="badge badge-purple">${e[2-1]}</span></td>
        <td class="muted">${e[2]}</td>
        <td><span class="badge ${e[3]==='已完成'?'badge-green':e[3]==='进行中'?'badge-blue':'badge-gray'}">${e[3]}</span></td>
        <td class="fw-600">${e[4]}</td>
        <td><button class="btn btn-outline btn-sm" onclick="enterImmersive('experiment')">${e[3]==='已完成'?'查看':'进入'}</button></td>
      </tr>`).join('')}
    </tbody>
  </table>`;

// 我的竞赛
PAGES['contests'] = () => head('我的竞赛','学习') + `
  <div class="tabs"><div class="tab active">进行中</div><div class="tab">可报名</div><div class="tab">已结束</div></div>
  <div class="grid grid-2">
    ${[['链上夺旗赛 2026','解题赛','进行中','还剩 2天3小时','第5名 / 320分'],
       ['智能合约攻防对抗','对抗赛','进行中','天梯赛进行中','ELO 1456 / 第12名']]
      .map(c=>`<div class="card card-pad">
        <div class="flex items-center justify-between mb-3">
          <span class="badge ${c[1]==='对抗赛'?'badge-red':'badge-amber'}">${c[1]}</span>
          <span class="badge badge-green">${c[2]}</span>
        </div>
        <div class="fw-700" style="font-size:17px">${c[0]}</div>
        <div class="text-sm muted mt-2 flex items-center gap-2"><i data-lucide="clock" style="width:15px;height:15px"></i>${c[3]}</div>
        <div class="flex items-center justify-between mt-3" style="padding-top:12px;border-top:1px solid var(--slate-100)">
          <div class="text-sm"><span class="muted">我的成绩</span> <span class="fw-700" style="color:var(--amber-600)">${c[4]}</span></div>
          <button class="btn btn-primary btn-sm" onclick="enterImmersive('${c[1]==='对抗赛'?'battle':'ctf'}')">进入竞赛</button>
        </div>
      </div>`).join('')}
  </div>
  <div class="fw-600 mt-4 mb-3">竞赛战绩</div>
  <table class="table card" style="overflow:hidden">
    <thead><tr><th>竞赛</th><th>类型</th><th>最终名次</th><th>得分/ELO</th><th>徽章</th></tr></thead>
    <tbody>
      <tr><td>新生区块链知识赛</td><td><span class="badge badge-gray">解题赛</span></td><td>3 / 156</td><td>880</td><td><span class="badge badge-amber">铜奖</span></td></tr>
      <tr><td>DeFi 安全挑战赛</td><td><span class="badge badge-gray">解题赛</span></td><td>8 / 92</td><td>1240</td><td>—</td></tr>
    </tbody>
  </table>`;

// 我的成绩
PAGES['grades'] = () => head('我的成绩','学习') + `
  <div class="grid grid-4 mb-4">
    ${statCard('award','3.72','当前 GPA','#F59E0B')}
    ${statCard('book-open','5','已修课程','#3B82F6')}
    ${statCard('check-circle-2','15','获得学分','#10B981')}
    ${statCard('trending-up','排名 12','专业排名','#8B5CF6')}
  </div>
  <div class="card" style="overflow:hidden">
    <div class="flex items-center justify-between" style="padding:14px 18px;border-bottom:1px solid var(--slate-200)">
      <span class="fw-600">课程成绩</span>
      <select class="select" style="width:160px"><option>2025-2026 秋</option><option>2024-2025 春</option></select>
    </div>
    <table class="table">
      <thead><tr><th>课程</th><th>学分</th><th>平时</th><th>实验</th><th>期末</th><th>总评</th><th>绩点</th></tr></thead>
      <tbody>
      ${[['区块链原理与智能合约','3.0','88','92','—','进行中','—'],
         ['密码学基础','2.0','90','95','88','91','4.0'],
         ['Web3 前端开发','2.0','85','—','82','83','3.3'],
         ['DeFi 协议开发','3.0','92','90','94','92','4.0']]
        .map(g=>`<tr><td class="fw-600">${g[0]}</td><td>${g[1]}</td><td>${g[2]}</td><td>${g[3]}</td><td>${g[4]}</td>
          <td><span class="${g[5]==='进行中'?'badge badge-blue':'fw-700'}">${g[5]}</span></td><td class="fw-600" style="color:var(--amber-600)">${g[6]}</td></tr>`).join('')}
      </tbody>
    </table>
  </div>
  <div class="text-sm muted mt-3"><i data-lucide="info" style="width:14px;height:14px;vertical-align:-2px"></i> 竞赛成绩独立计入战绩档案,不计入 GPA。如对成绩有异议可在课程结束 30 天内申诉。</div>`;

// 个人中心
PAGES['profile'] = () => head('个人中心','账户') + `
  <div class="grid" style="grid-template-columns:280px 1fr">
    <div class="card card-pad" style="text-align:center">
      <div class="ava" style="width:80px;height:80px;font-size:32px;margin:0 auto 14px;background:var(--amber);border-radius:50%;display:grid;place-items:center;color:#fff;font-weight:600">张</div>
      <div class="fw-700" style="font-size:18px">张同学</div>
      <div class="muted text-sm">学号 2023110325</div>
      <div class="badge badge-amber mt-3">学生</div>
    </div>
    <div class="card card-pad">
      <div class="fw-600 mb-4">基本信息</div>
      <div class="grid grid-2">
        <div class="field"><label>姓名</label><input class="input" value="张同学" disabled></div>
        <div class="field"><label>学号(不可改)</label><input class="input" value="2023110325" disabled></div>
        <div class="field"><label>学院 / 班级</label><input class="input" value="计算机学院 / 区块链2301" disabled></div>
        <div class="field"><label>手机号</label><div class="flex gap-2"><input class="input" value="138****5678"><button class="btn btn-outline">换绑</button></div></div>
      </div>
      <div class="fw-600 mb-3 mt-3">安全</div>
      <button class="btn btn-outline"><i data-lucide="key-round" style="width:16px;height:16px"></i> 修改密码</button>
      <div class="text-xs muted mt-3">学籍信息(学号/学院/班级)由学校管理员维护,不可自行修改。</div>
    </div>
  </div>`;

/* ============================================================
   沉浸工作模态
   ============================================================ */

// 实验工作台:左检查点/说明 + IDE + 终端
IMMERSIVE['experiment'] = {
  title:'实验:PoW 挖矿与51%攻击', chip:'检查点 2/5', action:'提交检查点',
  render(){ return `
    <div class="ws-panel" id="wsPanel">
      <div class="ws-panel-tabs">
        <div class="ws-panel-tab active" onclick="wsPanelTab(this,'cp')">检查点</div>
        <div class="ws-panel-tab" onclick="wsPanelTab(this,'desc')">实验说明</div>
        <div class="ws-panel-tab" onclick="wsPanelTab(this,'files')">文件</div>
      </div>
      <div class="ws-panel-body">
        <div id="wsp-cp">
          <div class="text-sm muted mb-3">完成下列检查点获得得分</div>
          <div class="cp-list">
            ${[['部署 PoW 链节点',1],['实现简单挖矿逻辑',1],['发起 51% 算力攻击',0],['观察链分叉与双花',0],['提交实验报告',0]]
              .map((c,i)=>`<div class="cp-card ${c[1]?'passed':'pending'}">
                <span class="cp-status"><i data-lucide="${c[1]?'check':'circle'}" style="width:13px;height:13px"></i></span>
                <div><div class="fw-600">检查点${i+1}</div><div class="text-xs muted">${c[0]}</div></div>
              </div>`).join('')}
          </div>
        </div>
        <div id="wsp-desc" style="display:none" class="text-sm" style="line-height:1.8">
          <div class="fw-600 mb-2">实验目标</div>
          <p class="muted">理解 PoW 共识与 51% 攻击原理。先部署一条 PoW 测试链,实现挖矿;再控制超过半数算力发起攻击,观察链重组与双花如何发生。</p>
          <div class="fw-600 mb-2 mt-3">环境</div>
          <p class="muted">EVM 运行时 · Hardhat · 已预置测试链。工具:代码编辑器 + 终端 + 区块链浏览器。</p>
        </div>
        <div id="wsp-files" style="display:none">
          <div class="text-sm" style="line-height:2">
            <div class="flex items-center gap-2"><i data-lucide="folder" style="width:15px;height:15px;color:var(--amber)"></i> contracts</div>
            <div style="padding-left:18px" class="flex items-center gap-2"><i data-lucide="file-code" style="width:14px;height:14px;color:var(--slate-400)"></i> Miner.sol</div>
            <div class="flex items-center gap-2"><i data-lucide="folder" style="width:15px;height:15px;color:var(--amber)"></i> scripts</div>
            <div style="padding-left:18px" class="flex items-center gap-2"><i data-lucide="file-code" style="width:14px;height:14px;color:var(--slate-400)"></i> attack.js</div>
          </div>
        </div>
      </div>
    </div>
    <div class="ws-main">
      <div class="ws-toolbar">
        <button class="btn-ghost" style="color:#94a3b8" onclick="togglePanel2()"><i data-lucide="panel-left" style="width:16px;height:16px"></i></button>
        <div class="ws-tool-tab active">Miner.sol</div>
        <div class="ws-tool-tab">attack.js</div>
        <div class="ws-tool-tab">浏览器</div>
        <div class="spacer" style="flex:1"></div>
        <button class="btn btn-primary btn-sm"><i data-lucide="play" style="width:14px;height:14px"></i> 运行</button>
      </div>
      <div class="ws-editor">// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

contract Miner {
    uint256 public difficulty = 4;
    bytes32 public lastHash;

    function mine(uint256 nonce) external {
        bytes32 h = keccak256(abi.encodePacked(block.number, nonce));
        require(uint256(h) < (2**256 - 1) >> difficulty, "not solved");
        lastHash = h;            // 出块成功
        emit BlockMined(msg.sender, nonce, h);
    }

    event BlockMined(address miner, uint256 nonce, bytes32 hash);
}</div>
      <div class="ws-terminal">
<span class="prompt">$</span> npx hardhat run scripts/deploy.js
<span class="ok">✓</span> Miner 部署成功: 0x5FbDB2315678afecb367f032d93F642f64180aa3
<span class="prompt">$</span> npx hardhat test
  PoW 挖矿
    <span class="ok">✓</span> 应能成功挖出区块 (412ms)
    <span class="ok">✓</span> 难度校验生效 (88ms)
<span class="ok">2 passing</span>
<span class="prompt">$</span> <span style="color:#fff">_</span></div>
    </div>`; },
  after(){}
};
function wsPanelTab(el, key){
  el.parentElement.querySelectorAll('.ws-panel-tab').forEach(t=>t.classList.remove('active'));
  el.classList.add('active');
  ['cp','desc','files'].forEach(k=>document.getElementById('wsp-'+k).style.display = k===key?'block':'none');
}
function togglePanel2(){ document.getElementById('wsPanel').classList.toggle('collapsed'); }

// 仿真工作台:左叙事+交互 + 仿真画布 + 时间控制
IMMERSIVE['sim'] = {
  title:'仿真:PBFT 共识与拜占庭攻击', chip:'', action:'达成目标',
  render(){ return `
    <div class="ws-panel">
      <div class="ws-panel-tabs"><div class="ws-panel-tab active">引导 & 交互</div><div class="ws-panel-tab">指标</div></div>
      <div class="ws-panel-body">
        <div class="narrative-bubble">
          <div class="fw-600 mb-2"><i data-lucide="lightbulb" style="width:15px;height:15px;vertical-align:-2px;color:var(--amber)"></i> 第 2 步</div>
          PBFT 需要 3f+1 个节点容忍 f 个拜占庭节点。现在尝试把 1 个节点设为拜占庭,观察共识是否仍能达成。
          <div class="mt-3"><b>思考:</b>4 个节点最多容忍几个作恶?</div>
        </div>
        <div class="fw-600 mb-3 text-sm">交互操作</div>
        <div class="interaction">
          <div class="it-head"><i data-lucide="sliders-horizontal" style="width:15px;height:15px;color:var(--amber)"></i> 节点总数</div>
          <input type="range" class="slider" min="4" max="16" value="4">
          <div class="text-xs muted mt-2">当前:4 个节点</div>
        </div>
        <div class="interaction attack">
          <div class="it-head"><i data-lucide="bug" style="width:15px;height:15px;color:var(--danger)"></i> 注入拜占庭节点 <span class="badge badge-red" style="margin-left:auto">攻击</span></div>
          <div class="text-xs muted mb-2">选择要作恶的节点,使其发送矛盾消息</div>
          <button class="btn btn-danger btn-sm w-full" style="justify-content:center">注入到节点 N2</button>
        </div>
        <div class="interaction">
          <div class="it-head"><i data-lucide="send" style="width:15px;height:15px;color:var(--amber)"></i> 发起共识提案</div>
          <button class="btn btn-primary btn-sm w-full" style="justify-content:center">由主节点发起</button>
        </div>
      </div>
    </div>
    <div class="ws-main">
      <div class="sim-canvas" id="simCanvas"></div>
      <div class="sim-controls">
        <button class="sim-ctrl-btn"><i data-lucide="skip-back" style="width:16px;height:16px"></i></button>
        <button class="sim-ctrl-btn"><i data-lucide="play" style="width:16px;height:16px"></i></button>
        <button class="sim-ctrl-btn"><i data-lucide="skip-forward" style="width:16px;height:16px"></i></button>
        <span class="text-sm" style="color:var(--slate-300);margin-left:8px">Tick 23 · Prepare 阶段</span>
        <div class="spacer" style="flex:1"></div>
        <span class="text-xs" style="color:var(--slate-400)">速度</span>
        <div class="sim-speed"><button>0.5×</button><button class="active">1×</button><button>2×</button><button>4×</button></div>
      </div>
    </div>`; },
  after(){
    const cv = document.getElementById('simCanvas');
    const pos=[[35,25],[65,25],[65,60],[35,60]];
    pos.forEach((p,i)=>{ const n=document.createElement('div'); n.className='sim-node';
      n.style.left=p[0]+'%'; n.style.top=p[1]+'%';
      n.innerHTML = i===1?'N2<br>拜占庭':'N'+(i+1);
      if(i===1){ n.style.borderColor='var(--danger)'; n.style.color='#fff'; n.style.boxShadow='0 0 20px rgba(239,68,68,.5)'; }
      cv.appendChild(n); });
  }
};

// 竞赛答题(解题赛)
IMMERSIVE['ctf'] = {
  title:'链上夺旗赛 · 题目:可重入的金库', chip:'剩余 2:58:12', action:'提交 Flag',
  render(){ return `
    <div class="ws-panel">
      <div class="ws-panel-tabs"><div class="ws-panel-tab active">题目</div><div class="ws-panel-tab">排行榜</div></div>
      <div class="ws-panel-body text-sm">
        <span class="badge badge-red mb-3">真实漏洞复现 · 500分</span>
        <div class="fw-600 mt-3 mb-2">可重入的金库</div>
        <p class="muted" style="line-height:1.8">下面的 Vault 合约存在重入漏洞。编写攻击合约掏空金库余额,使其 ETH 余额低于 1 ether,即可获得 flag。</p>
        <div class="fw-600 mt-3 mb-2">提交 Flag</div>
        <div class="input-icon mb-2"><i data-lucide="flag"></i><input class="input" placeholder="flag{...}"></div>
        <div class="text-xs muted"><i data-lucide="users" style="width:13px;height:13px;vertical-align:-2px"></i> 42 人已解出 · 当前动态分 460</div>
      </div>
    </div>
    <div class="ws-main">
      <div class="ws-toolbar"><div class="ws-tool-tab active">Attack.sol</div><div class="ws-tool-tab">Vault.sol(只读)</div><div class="spacer" style="flex:1"></div><button class="btn btn-primary btn-sm"><i data-lucide="zap" style="width:14px;height:14px"></i> 部署并攻击</button></div>
      <div class="ws-editor">contract Attack {
    Vault public vault;
    constructor(address _v) { vault = Vault(_v); }

    function attack() external payable {
        vault.deposit{value: 1 ether}();
        vault.withdraw();           // 触发重入
    }

    receive() external payable {
        if (address(vault).balance >= 1 ether) {
            vault.withdraw();       // 递归提款
        }
    }
}</div>
      <div class="ws-terminal"><span class="prompt">$</span> 部署攻击合约...
<span class="ok">✓</span> Attack 部署成功
<span class="prompt">$</span> 执行 attack()...
<span class="ok">✓</span> 金库余额: 0.0 ETH (已掏空)
<span class="ok">✓</span> 链上断言通过 → flag{re3ntr4ncy_dr41n3d}</div>
    </div>`; }
};

// 对抗赛对局回放
IMMERSIVE['battle'] = {
  title:'对抗赛 · 对局回放 #2847', chip:'你 vs 选手B', action:'提交新策略',
  render(){ return `
    <div class="ws-panel">
      <div class="ws-panel-body text-sm">
        <div class="fw-600 mb-3">对局信息</div>
        <div class="card card-pad mb-3" style="box-shadow:none">
          <div class="flex items-center justify-between"><span>你(守方)</span><span class="badge badge-green">防御合约</span></div>
          <div class="flex items-center justify-between mt-2"><span>选手B(攻方)</span><span class="badge badge-red">攻击脚本</span></div>
        </div>
        <div class="cp-card passed mb-3"><span class="cp-status"><i data-lucide="trophy" style="width:13px;height:13px"></i></span><div><div class="fw-600">结果:你胜</div><div class="text-xs muted">防御合约挡住了重入攻击 · ELO +18</div></div></div>
        <div class="fw-600 mb-2">回放进度</div>
        <input type="range" class="slider" value="60">
        <div class="text-xs muted mt-2">第 6 / 10 步交易</div>
      </div>
    </div>
    <div class="ws-main">
      <div class="ws-toolbar"><div class="ws-tool-tab active">链上交易回放</div><div class="spacer" style="flex:1"></div>
        <button class="btn-ghost" style="color:#94a3b8"><i data-lucide="play" style="width:15px;height:15px"></i></button></div>
      <div class="ws-terminal" style="height:auto;flex:1">
<span style="color:#94a3b8">[步骤 1]</span> 攻方部署 Attack 合约 → 0x9a3c...
<span style="color:#94a3b8">[步骤 2]</span> 攻方调用 attack() value=1 ETH
<span style="color:#94a3b8">[步骤 3]</span> 守方 Vault.deposit() 记账
<span style="color:#94a3b8">[步骤 4]</span> 守方 Vault.withdraw() 触发
<span style="color:#94a3b8">[步骤 5]</span> 攻方 receive() 尝试递归 withdraw
<span class="err">[步骤 6]</span> <span class="ok">✓ 守方 nonReentrant 修饰符拦截!revert</span>
<span style="color:#94a3b8">[步骤 7]</span> 攻击失败,金库余额保持 10 ETH
<span class="ok">═══ 守方防御成功,本局你胜 ═══</span></div>
    </div>`; }
};

/* ============================================================
   教师页面
   ============================================================ */

// 课程管理
PAGES['t-courses'] = () => head('课程管理','教学', `<button class="btn btn-primary" onclick="navigate('t-course-edit')"><i data-lucide="plus" style="width:16px;height:16px"></i> 新建课程</button>`) + `
  <div class="grid grid-4 mb-4">
    ${statCard('book-open','6','我的课程','#F59E0B')}
    ${statCard('users','312','学生总数','#3B82F6')}
    ${statCard('file-edit','18','待批改','#EF4444')}
    ${statCard('flask-conical','24','已布置实验','#8B5CF6')}
  </div>
  <table class="table card" style="overflow:hidden">
    <thead><tr><th>课程</th><th>学期</th><th>学生</th><th>状态</th><th>操作</th></tr></thead>
    <tbody>
    ${[['区块链原理与智能合约','2025秋','128','进行中'],['DeFi 协议开发实战','2025秋','86','进行中'],['密码学基础','2025春','98','已结束'],['智能合约安全审计','2025秋','45','草稿']]
      .map(c=>`<tr><td class="fw-600">${c[0]}</td><td class="muted">${c[1]}</td><td>${c[2]}</td>
        <td><span class="badge ${c[3]==='进行中'?'badge-green':c[3]==='草稿'?'badge-gray':'badge-blue'}">${c[3]}</span></td>
        <td class="flex gap-2"><button class="btn btn-outline btn-sm" onclick="navigate('t-course-edit')">编辑</button><button class="btn btn-ghost btn-sm">数据</button></td></tr>`).join('')}
    </tbody>
  </table>`;

// 课程编辑
PAGES['t-course-edit'] = () => head('编辑课程:区块链原理与智能合约','<a onclick="navigate(\'t-courses\')">课程管理</a> <span class="sep">/</span> 编辑',
  `<button class="btn btn-outline">保存草稿</button><button class="btn btn-primary">发布</button>`) + `
  <div class="tabs"><div class="tab active">基本信息</div><div class="tab">章节内容</div><div class="tab">作业</div><div class="tab">学生</div><div class="tab">成绩设置</div></div>
  <div class="grid" style="grid-template-columns:1fr 320px">
    <div class="card card-pad">
      <div class="grid grid-2">
        <div class="field"><label>课程名称</label><input class="input" value="区块链原理与智能合约"></div>
        <div class="field"><label>课程类型</label><select class="select"><option>混合课</option><option>理论课</option><option>实验课</option><option>项目实战</option></select></div>
        <div class="field"><label>学分</label><input class="input" value="3.0"></div>
        <div class="field"><label>学期</label><select class="select"><option>2025-2026 秋</option></select></div>
      </div>
      <div class="field"><label>课程简介</label><textarea class="textarea" rows="3">系统讲解区块链核心原理,结合 Solidity 智能合约开发实战...</textarea></div>
      <div class="field"><label>邀请码</label><div class="flex gap-2"><input class="input mono" value="BC2026X" style="width:160px"><button class="btn btn-outline"><i data-lucide="refresh-cw" style="width:14px;height:14px"></i> 刷新</button></div></div>
    </div>
    <div class="card card-pad" style="text-align:center;height:fit-content">
      <div style="height:120px;background:var(--slate-100);border-radius:8px;display:grid;place-items:center;margin-bottom:12px"><i data-lucide="image" style="width:32px;height:32px;color:var(--slate-400)"></i></div>
      <button class="btn btn-outline w-full" style="justify-content:center">上传封面</button>
    </div>
  </div>`;

// 实验管理
PAGES['t-experiments'] = () => head('实验管理','实践', `<button class="btn btn-primary" onclick="navigate('t-exp-wizard')"><i data-lucide="plus" style="width:16px;height:16px"></i> 新建实验</button>`) + `
  <table class="table card" style="overflow:hidden">
    <thead><tr><th>实验名称</th><th>组件</th><th>所属课程</th><th>提交/完成</th><th>状态</th><th></th></tr></thead>
    <tbody>
    ${[['PoW 挖矿与51%攻击','沙箱+仿真+3检查点','区块链原理','98/128','已发布'],
       ['部署 ERC20 合约','沙箱+2检查点','区块链原理','120/128','已发布'],
       ['默克尔树仿真','仅仿真','密码学','88/98','已发布'],
       ['Fabric 联盟链组网','多人沙箱','联盟链','12/45','草稿']]
      .map(e=>`<tr><td class="fw-600">${e[0]}</td><td><span class="badge badge-purple">${e[1]}</span></td><td class="muted">${e[2]}</td><td>${e[3]}</td>
        <td><span class="badge ${e[4]==='已发布'?'badge-green':'badge-gray'}">${e[4]}</span></td>
        <td class="flex gap-2"><button class="btn btn-outline btn-sm" onclick="navigate('t-exp-wizard')">编辑</button><button class="btn btn-ghost btn-sm" onclick="navigate('t-monitor')">监控</button></td></tr>`).join('')}
    </tbody>
  </table>`;

// ===== 实验编排向导(教师组装引擎,核心流程) =====
let wizStep = 1;
PAGES['t-exp-wizard'] = () => { wizStep = wizStep||1; return head('新建实验 · 编排向导','<a onclick="navigate(\'t-experiments\')">实验管理</a> <span class="sep">/</span> 编排向导') + renderWizard(); };

function renderWizard(){
  const steps = ['基本信息','环境组件','仿真组件','检查点','说明与协作','发布预览'];
  return `
  <div class="card card-pad mb-4">
    <div class="flex items-center" style="gap:0">
      ${steps.map((s,i)=>{const n=i+1; const st=n<wizStep?'done':n===wizStep?'active':'todo';
        return `<div class="flex items-center" style="${i<steps.length-1?'flex:1':''}">
          <div onclick="wizGo(${n})" style="cursor:pointer;display:flex;align-items:center;gap:8px">
            <span style="width:28px;height:28px;border-radius:50%;display:grid;place-items:center;font-size:13px;font-weight:600;
              ${st==='done'?'background:var(--success);color:#fff':st==='active'?'background:var(--amber);color:#fff':'background:var(--slate-200);color:var(--slate-500)'}">
              ${st==='done'?'<i data-lucide=check style="width:15px;height:15px"></i>':n}</span>
            <span class="text-sm ${st==='active'?'fw-600':'muted'}">${s}</span>
          </div>
          ${i<steps.length-1?`<div style="flex:1;height:2px;margin:0 12px;background:${n<wizStep?'var(--success)':'var(--slate-200)'}"></div>`:''}
        </div>`}).join('')}
    </div>
  </div>
  <div class="card card-pad" style="min-height:380px">${wizContent()}</div>
  <div class="flex justify-between mt-4">
    <button class="btn btn-outline" onclick="wizGo(${wizStep-1})" ${wizStep===1?'disabled':''}>上一步</button>
    ${wizStep<6?`<button class="btn btn-primary" onclick="wizGo(${wizStep+1})">下一步</button>`
              :`<button class="btn btn-primary" onclick="navigate('t-experiments')"><i data-lucide="check" style="width:16px;height:16px"></i> 发布实验</button>`}
  </div>`;
}
function wizGo(n){ if(n<1||n>6) return; wizStep=n; navigate('t-exp-wizard'); }

function wizContent(){
  if(wizStep===1) return `
    <div class="fw-600 mb-4">① 基本信息</div>
    <div class="grid grid-2">
      <div class="field"><label>实验名称</label><input class="input" value="PoW 挖矿与51%攻击"></div>
      <div class="field"><label>所属课程</label><select class="select"><option>区块链原理与智能合约</option></select></div>
      <div class="field"><label>基于模板</label><select class="select"><option>从空白创建</option><option>PoW 实验模板 v1.2</option></select></div>
      <div class="field"><label>难度</label><select class="select"><option>进阶</option></select></div>
    </div>
    <div class="text-sm muted"><i data-lucide="info" style="width:14px;height:14px;vertical-align:-2px"></i> 组件自由组合:本实验可包含 沙箱环境 + 仿真 + 检查点 的任意组合,无需预设固定类型。</div>`;

  if(wizStep===2) return `
    <div class="flex items-center justify-between mb-4"><div class="fw-600">② 环境组件(M2 沙箱)</div><button class="btn btn-outline btn-sm"><i data-lucide="plus" style="width:14px;height:14px"></i> 添加环境</button></div>
    <div class="card card-pad mb-3" style="box-shadow:none;border-style:dashed">
      <div class="grid grid-2">
        <div class="field"><label>运行时(链)</label><select class="select"><option>EVM · Hardhat</option><option>EVM · Foundry</option><option>Hyperledger Fabric</option><option>FISCO BCOS</option><option>长安链</option><option>Solana</option></select></div>
        <div class="field"><label>初始代码模板</label><select class="select"><option>PoW 起始代码</option><option>空白</option></select></div>
      </div>
      <div class="field"><label>工具集(可多选,前端动态渲染工作台)</label>
        <div class="flex gap-2 mt-2" style="flex-wrap:wrap">
          ${[['代码编辑器(Monaco)',1],['终端(K8s exec)',1],['区块链浏览器',1],['Remix IDE',0],['Jupyter',0],['图形桌面',0]]
            .map(t=>`<span class="badge ${t[1]?'badge-amber':'badge-gray'}" style="cursor:pointer;padding:6px 12px"><i data-lucide="${t[1]?'check':'plus'}" style="width:13px;height:13px"></i> ${t[0]}</span>`).join('')}
        </div>
      </div>
      <div class="text-xs muted mt-2">个性化初始化脚本:部署 PoW 合约 + 预置 3 个矿工账户(异步执行,学生秒进环境)</div>
    </div>`;

  if(wizStep===3) return `
    <div class="flex items-center justify-between mb-4"><div class="fw-600">③ 仿真组件(M4)</div><button class="btn btn-outline btn-sm"><i data-lucide="plus" style="width:14px;height:14px"></i> 添加仿真</button></div>
    <div class="card card-pad" style="box-shadow:none;border-style:dashed">
      <div class="field"><label>仿真包</label><select class="select"><option>PoW 挖矿与51%攻击仿真 v1.0</option><option>PBFT 共识仿真</option><option>默克尔树构建</option></select></div>
      <div class="grid grid-2">
        <div class="field"><label>初始矿工数</label><input class="input" value="4"></div>
        <div class="field"><label>初始难度</label><input class="input" value="4"></div>
      </div>
      <div class="field"><label>启用教学叙事</label>
        <div class="flex gap-2 mt-2"><span class="badge badge-amber" style="padding:6px 12px"><i data-lucide="check" style="width:13px;height:13px"></i> 分步引导</span><span class="badge badge-amber" style="padding:6px 12px"><i data-lucide="check" style="width:13px;height:13px"></i> 设问检查点</span></div>
      </div>
      <div class="text-xs muted mt-2"><i data-lucide="lightbulb" style="width:13px;height:13px;vertical-align:-2px"></i> 仿真在前端运行,学生注入攻击零延迟即时看到链分叉效果。</div>
    </div>`;

  if(wizStep===4) return `
    <div class="flex items-center justify-between mb-4"><div class="fw-600">④ 检查点(M3 判题,决定得分)</div><button class="btn btn-outline btn-sm"><i data-lucide="plus" style="width:14px;height:14px"></i> 添加检查点</button></div>
    ${[['部署 PoW 链节点','链上断言','env','30'],['实现挖矿逻辑','测试用例','env','30'],['成功发起51%攻击','仿真检查点','sim','40']]
      .map((c,i)=>`<div class="card card-pad mb-3" style="box-shadow:none">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-3"><span class="badge badge-gray">检查点${i+1}</span><span class="fw-600">${c[0]}</span></div>
          <div class="flex items-center gap-2"><span class="badge badge-purple">${c[1]}</span><span class="badge badge-amber">${c[3]}分</span><button class="btn-ghost"><i data-lucide="settings-2" style="width:16px;height:16px;color:var(--slate-400)"></i></button></div>
        </div>
      </div>`).join('')}
    <div class="text-sm muted mt-2"><i data-lucide="info" style="width:14px;height:14px;vertical-align:-2px"></i> 分值合计 100。判题在隔离 judge 沙箱执行,答案对学生黑盒。</div>`;

  if(wizStep===5) return `
    <div class="fw-600 mb-4">⑤ 说明与协作</div>
    <div class="field"><label>实验说明(支持 Markdown)</label><textarea class="textarea" rows="5">## 实验目标
理解 PoW 共识与 51% 攻击原理...

## 步骤
1. 部署 PoW 链
2. 实现挖矿
3. 发起攻击观察分叉</textarea></div>
    <div class="grid grid-2">
      <div class="field"><label>协作模式</label><select class="select"><option>单人实验</option><option>小组实验(共享环境)</option></select></div>
      <div class="field"><label>要求提交报告</label><select class="select"><option>是</option><option>否</option></select></div>
    </div>`;

  return `
    <div class="fw-600 mb-4">⑥ 发布预览</div>
    <div class="card card-pad mb-3" style="box-shadow:none;background:var(--slate-50)">
      <div class="flex items-center gap-2 mb-3" style="color:var(--success)"><i data-lucide="check-circle-2" style="width:18px;height:18px"></i> <span class="fw-600">发布前校验通过</span></div>
      <div class="text-sm" style="line-height:1.9">
        <div>✓ 依赖完整性:EVM 运行时可用</div>
        <div>✓ 组件可用性:仿真包 v1.0 已上架</div>
        <div>✓ 分值合理性:检查点合计 100 分</div>
        <div>✓ 连通性预检:沙箱启动正常</div>
      </div>
    </div>
    <div class="card card-pad" style="box-shadow:none">
      <div class="fw-600 mb-3">实验构成</div>
      <div class="flex gap-2" style="flex-wrap:wrap">
        <span class="badge badge-blue" style="padding:7px 14px"><i data-lucide="box" style="width:14px;height:14px"></i> 1个沙箱环境(EVM+3工具)</span>
        <span class="badge badge-purple" style="padding:7px 14px"><i data-lucide="activity" style="width:14px;height:14px"></i> 1个仿真</span>
        <span class="badge badge-amber" style="padding:7px 14px"><i data-lucide="check-square" style="width:14px;height:14px"></i> 3个检查点</span>
      </div>
    </div>`;
}

// 竞赛管理
PAGES['t-contests'] = () => head('竞赛管理','实践', `<button class="btn btn-primary" onclick="navigate('t-contest-edit')"><i data-lucide="plus" style="width:16px;height:16px"></i> 新建竞赛</button>`) + `
  <table class="table card" style="overflow:hidden">
    <thead><tr><th>竞赛名称</th><th>赛制</th><th>报名/参赛</th><th>时间</th><th>状态</th><th></th></tr></thead>
    <tbody>
    ${[['链上夺旗赛 2026','解题赛','320','进行中','进行中'],['智能合约攻防对抗','对抗赛(天梯)','86','进行中','进行中'],['新生区块链知识赛','解题赛','156','已结束','已结束'],['DeFi 安全月赛','对抗赛','—','下月','草稿']]
      .map(c=>`<tr><td class="fw-600">${c[0]}</td><td><span class="badge ${c[1].includes('对抗')?'badge-red':'badge-amber'}">${c[1]}</span></td><td>${c[2]}</td><td class="muted">${c[3]}</td>
        <td><span class="badge ${c[4]==='进行中'?'badge-green':c[4]==='草稿'?'badge-gray':'badge-blue'}">${c[4]}</span></td>
        <td class="flex gap-2"><button class="btn btn-outline btn-sm" onclick="navigate('t-contest-edit')">编辑</button><button class="btn btn-ghost btn-sm">榜单</button></td></tr>`).join('')}
    </tbody>
  </table>`;

// 竞赛配置
PAGES['t-contest-edit'] = () => head('新建竞赛','<a onclick="navigate(\'t-contests\')">竞赛管理</a> <span class="sep">/</span> 配置',
  `<button class="btn btn-outline">保存草稿</button><button class="btn btn-primary">发布</button>`) + `
  <div class="grid" style="grid-template-columns:1fr 320px">
    <div class="card card-pad">
      <div class="grid grid-2">
        <div class="field"><label>竞赛名称</label><input class="input" placeholder="如:链上夺旗赛 2026"></div>
        <div class="field"><label>赛制</label><select class="select" onchange="this.value"><option>解题赛(含理论题)</option><option>对抗赛 · 攻防</option><option>对抗赛 · 博弈</option></select></div>
        <div class="field"><label>组队模式</label><select class="select"><option>个人赛</option><option>团队赛(可跨校)</option></select></div>
        <div class="field"><label>撮合方式(对抗赛)</label><select class="select"><option>天梯 ELO</option><option>循环赛</option></select></div>
        <div class="field"><label>报名时间</label><input class="input" type="text" value="2026-06-01 ~ 06-10"></div>
        <div class="field"><label>比赛时间</label><input class="input" type="text" value="2026-06-15 ~ 06-17"></div>
        <div class="field"><label>封榜时长(分钟)</label><input class="input" value="60"></div>
      </div>
      <div class="fw-600 mb-3 mt-2">题目编排(引用题库)</div>
      ${[['可重入的金库','真实漏洞','500'],['整数溢出','模板题','300'],['PBFT 容错知识','理论题','100']]
        .map(p=>`<div class="flex items-center justify-between" style="padding:9px 0;border-top:1px solid var(--slate-100)">
          <div class="flex items-center gap-3"><i data-lucide="file-code" style="width:16px;height:16px;color:var(--slate-400)"></i>${p[0]} <span class="badge badge-purple">${p[1]}</span></div>
          <span class="badge badge-amber">${p[2]}分</span></div>`).join('')}
      <button class="btn btn-outline btn-sm mt-3"><i data-lucide="plus" style="width:14px;height:14px"></i> 从题库添加</button>
    </div>
    <div class="card card-pad" style="height:fit-content">
      <div class="fw-600 mb-3">赛制说明</div>
      <div class="text-sm muted" style="line-height:1.8">解题赛:选手独立解题自动判定+实时排行。对抗赛:提交合约/脚本,系统异步撮合对局,ELO天梯排名,支持战报回放。</div>
    </div>
  </div>`;

// 题库
PAGES['t-content'] = () => head('题库','资源', `<button class="btn btn-primary"><i data-lucide="plus" style="width:16px;height:16px"></i> 新建内容</button>`) + `
  <div class="flex gap-3 mb-4" style="flex-wrap:wrap">
    <select class="select" style="width:140px"><option>全部类型</option><option>实验模板</option><option>竞赛题</option><option>理论题</option></select>
    <select class="select" style="width:120px"><option>全部难度</option></select>
    <div class="input-icon" style="flex:1;max-width:280px"><i data-lucide="search"></i><input class="input" placeholder="搜索题目/知识点"></div>
    <button class="btn btn-outline"><i data-lucide="library" style="width:15px;height:15px"></i> 共享库</button>
  </div>
  <table class="table card" style="overflow:hidden">
    <thead><tr><th>标题</th><th>类型</th><th>难度</th><th>知识点</th><th>版本</th><th>引用</th><th></th></tr></thead>
    <tbody>
    ${[['部署ERC20合约','实验模板','进阶','智能合约','v1.2','24'],['可重入的金库','竞赛题','高级','安全/重入','v1.0','8'],['PBFT容错节点数','理论题','入门','共识','v1.0','56'],['默克尔树构建','实验模板','进阶','密码学','v2.0','12'],['51%攻击复现','竞赛题(真实漏洞)','高级','共识/攻击','v1.1','5']]
      .map(c=>`<tr><td class="fw-600">${c[0]}</td><td><span class="badge ${c[1].includes('实验')?'badge-blue':c[1].includes('竞赛')?'badge-red':'badge-gray'}">${c[1]}</span></td>
        <td>${c[2]}</td><td class="muted">${c[3]}</td><td class="mono text-xs">${c[4]}</td><td>${c[5]}</td>
        <td class="flex gap-2"><button class="btn btn-ghost btn-sm">编辑</button><button class="btn btn-ghost btn-sm">克隆</button></td></tr>`).join('')}
    </tbody>
  </table>`;

// 仿真场景
PAGES['t-sim'] = () => head('仿真场景','资源', `<button class="btn btn-primary"><i data-lucide="upload" style="width:16px;height:16px"></i> 上传自定义场景</button>`) + `
  <div class="text-sm muted mb-3">平台已上架的仿真场景,可在实验/课程中引用。教师可按 SDK 规范开发自定义场景上传审核。</div>
  <div class="grid grid-3">
    ${[['PoW 挖矿与51%攻击','共识','前端运行'],['PBFT 共识过程','共识','前端运行'],['默克尔树构建验证','密码学','前端运行'],['哈希函数原理','密码学','前端运行'],['P2P 网络传播','网络','前端运行'],['重入攻击演示','安全','前端运行']]
      .map(s=>`<div class="card card-hover card-pad">
        <div class="flex items-center justify-between mb-3">
          <div class="stat-icon" style="background:var(--slate-900)"><i data-lucide="activity" style="color:var(--amber);width:20px;height:20px"></i></div>
          <span class="badge badge-gray">${s[1]}</span>
        </div>
        <div class="fw-600">${s[0]}</div>
        <div class="text-xs muted mt-2 flex items-center gap-2"><i data-lucide="zap" style="width:13px;height:13px"></i>${s[2]} · 可交互</div>
        <button class="btn btn-outline btn-sm w-full mt-3" style="justify-content:center" onclick="enterImmersive('sim')">预览</button>
      </div>`).join('')}
  </div>`;

// 批改中心
PAGES['t-grade-review'] = () => head('批改中心','教学') + `
  <div class="tabs"><div class="tab active">待批改 (18)</div><div class="tab">实验报告 (6)</div><div class="tab">已批改</div></div>
  <table class="table card" style="overflow:hidden">
    <thead><tr><th>学生</th><th>作业/实验</th><th>题型</th><th>提交时间</th><th>自动分</th><th></th></tr></thead>
    <tbody>
    ${[['张同学','作业3:ERC20','编程题','2小时前','已判85'],['李同学','作业3:简答','主观题','3小时前','待批'],['王同学','实验:PoW报告','实验报告','5小时前','待批'],['赵同学','作业3:论述','主观题','1天前','待批']]
      .map(g=>`<tr><td class="fw-600">${g[0]}</td><td>${g[1]}</td><td><span class="badge badge-gray">${g[2]}</span></td><td class="muted">${g[3]}</td>
        <td>${g[4]}</td><td><button class="btn btn-primary btn-sm">批改</button></td></tr>`).join('')}
    </tbody>
  </table>`;

// 学生监控
PAGES['t-monitor'] = () => head('学生监控:PoW 挖矿实验','<a onclick="navigate(\'t-experiments\')">实验管理</a> <span class="sep">/</span> 实时监控') + `
  <div class="grid grid-4 mb-4">
    ${statCard('users','98','在线学生','#3B82F6')}
    ${statCard('check-circle-2','42','已完成','#10B981')}
    ${statCard('loader','51','进行中','#F59E0B')}
    ${statCard('alert-triangle','5','遇到困难','#EF4444')}
  </div>
  <div class="flex items-center justify-between mb-3"><div class="fw-600">学生实时进度</div>
    <div class="flex gap-2"><button class="btn btn-outline btn-sm"><i data-lucide="megaphone" style="width:14px;height:14px"></i> 广播</button><button class="btn btn-outline btn-sm"><i data-lucide="pause" style="width:14px;height:14px"></i> 集中暂停</button></div></div>
  <div class="grid grid-4">
    ${[['张同学','检查点3','正常','green'],['李同学','检查点2','正常','green'],['王同学','检查点1','落后','amber'],['赵同学','检查点4','正常','green'],['陈同学','检查点1','异常','red'],['刘同学','检查点3','正常','green'],['周同学','检查点2','正常','green'],['吴同学','检查点1','异常','red']]
      .map(s=>`<div class="card card-pad" style="${s[3]==='red'?'border-color:var(--danger)':s[3]==='amber'?'border-color:var(--amber)':''}">
        <div class="flex items-center justify-between mb-2"><span class="fw-600 text-sm">${s[0]}</span><span class="badge badge-${s[3]==='green'?'green':s[3]==='amber'?'amber':'red'}">${s[2]}</span></div>
        <div style="height:60px;background:var(--slate-900);border-radius:6px;display:grid;place-items:center;margin-bottom:8px"><i data-lucide="activity" style="width:20px;height:20px;color:var(--amber)"></i></div>
        <div class="text-xs muted">${s[1]} · Tick ${20+Math.floor(Math.random()*30)}</div>
      </div>`).join('')}
  </div>`;

/* ============================================================
   学校管理员页面
   ============================================================ */

// 用户管理
PAGES['s-users'] = () => head('用户管理','管理', `<button class="btn btn-outline"><i data-lucide="download" style="width:15px;height:15px"></i> 导出</button><button class="btn btn-primary"><i data-lucide="upload" style="width:15px;height:15px"></i> 批量导入</button>`) + `
  <div class="grid grid-4 mb-4">
    ${statCard('users','3,248','学生总数','#3B82F6')}
    ${statCard('presentation','156','教师总数','#F59E0B')}
    ${statCard('user-check','2,980','活跃账号','#10B981')}
    ${statCard('archive','268','已归档','#64748B')}
  </div>
  <div class="flex gap-3 mb-3" style="flex-wrap:wrap">
    <div class="tabs" style="border:none;margin:0"><div class="tab active">学生</div><div class="tab">教师</div></div>
    <div class="spacer" style="flex:1"></div>
    <select class="select" style="width:140px"><option>全部班级</option></select>
    <select class="select" style="width:120px"><option>全部状态</option><option>正常</option><option>停用</option><option>已归档</option></select>
    <div class="input-icon" style="width:220px"><i data-lucide="search"></i><input class="input" placeholder="学号/姓名/手机号"></div>
  </div>
  <table class="table card" style="overflow:hidden">
    <thead><tr><th><input type="checkbox"></th><th>姓名</th><th>学号</th><th>学院/班级</th><th>手机号</th><th>状态</th><th></th></tr></thead>
    <tbody>
    ${[['张同学','2023110325','计算机/区块链2301','138****5678','正常'],['李同学','2023110326','计算机/区块链2301','139****1234','正常'],['王同学','2023110327','计算机/区块链2302','137****8888','停用'],['赵同学','2022110201','计算机/区块链2201','136****6666','已归档']]
      .map(u=>`<tr><td><input type="checkbox"></td><td class="fw-600">${u[0]}</td><td class="mono text-xs">${u[1]}</td><td class="muted">${u[2]}</td><td class="mono text-xs">${u[3]}</td>
        <td><span class="badge ${u[4]==='正常'?'badge-green':u[4]==='停用'?'badge-gray':'badge-blue'}">${u[4]}</span></td>
        <td class="flex gap-2"><button class="btn btn-ghost btn-sm">编辑</button><button class="btn btn-ghost btn-sm">重置密码</button></td></tr>`).join('')}
    </tbody>
  </table>
  <div class="text-sm muted mt-3"><i data-lucide="info" style="width:14px;height:14px;vertical-align:-2px"></i> 禁止自助注册:师生账号仅由学校管理员导入。批量操作支持停用/归档/恢复。</div>`;

// 组织架构
PAGES['s-org'] = () => head('组织架构','管理', `<button class="btn btn-primary"><i data-lucide="plus" style="width:15px;height:15px"></i> 新增院系</button>`) + `
  <div class="grid" style="grid-template-columns:300px 1fr">
    <div class="card card-pad">
      <div class="fw-600 mb-3">院系 → 专业 → 班级</div>
      <div class="text-sm" style="line-height:2.2">
        <div class="flex items-center gap-2 fw-600"><i data-lucide="chevron-down" style="width:15px;height:15px"></i><i data-lucide="building" style="width:15px;height:15px;color:var(--amber)"></i> 计算机学院</div>
        <div style="padding-left:22px"><div class="flex items-center gap-2"><i data-lucide="chevron-down" style="width:14px;height:14px"></i><i data-lucide="folder" style="width:14px;height:14px;color:var(--slate-400)"></i> 区块链工程</div>
          <div style="padding-left:24px" class="muted"><div>区块链2301 (32人)</div><div>区块链2302 (30人)</div></div>
        </div>
        <div style="padding-left:22px"><div class="flex items-center gap-2"><i data-lucide="chevron-right" style="width:14px;height:14px"></i><i data-lucide="folder" style="width:14px;height:14px;color:var(--slate-400)"></i> 软件工程</div></div>
        <div class="flex items-center gap-2 fw-600 mt-2"><i data-lucide="chevron-right" style="width:15px;height:15px"></i><i data-lucide="building" style="width:15px;height:15px;color:var(--amber)"></i> 网络空间安全学院</div>
      </div>
    </div>
    <div class="card card-pad">
      <div class="fw-600 mb-3">区块链2301 班级详情</div>
      <div class="grid grid-2 mb-3">
        <div class="field"><label>班级名称</label><input class="input" value="区块链2301"></div>
        <div class="field"><label>入学年份</label><input class="input" value="2023"></div>
      </div>
      <div class="flex gap-2"><button class="btn btn-outline btn-sm"><i data-lucide="archive" style="width:14px;height:14px"></i> 按学年归档</button><button class="btn btn-outline btn-sm"><i data-lucide="arrow-up" style="width:14px;height:14px"></i> 班级升级</button></div>
    </div>
  </div>`;

// 成绩审核
PAGES['s-grade-audit'] = () => head('成绩审核','管理') + `
  <div class="tabs"><div class="tab active">待审核 (5)</div><div class="tab">已通过</div><div class="tab">申诉处理 (2)</div></div>
  <table class="table card" style="overflow:hidden">
    <thead><tr><th>课程</th><th>教师</th><th>学生数</th><th>提交时间</th><th>状态</th><th></th></tr></thead>
    <tbody>
    ${[['区块链原理与智能合约','李老师','128','1天前','待审核'],['密码学基础','张老师','98','2天前','待审核'],['DeFi协议开发','王老师','86','3天前','待审核']]
      .map(g=>`<tr><td class="fw-600">${g[0]}</td><td>${g[1]}</td><td>${g[2]}</td><td class="muted">${g[3]}</td>
        <td><span class="badge badge-amber">${g[4]}</span></td>
        <td class="flex gap-2"><button class="btn btn-primary btn-sm">审核</button><button class="btn btn-ghost btn-sm">查看</button></td></tr>`).join('')}
    </tbody>
  </table>
  <div class="text-sm muted mt-3"><i data-lucide="info" style="width:14px;height:14px;vertical-align:-2px"></i> 审核通过后成绩锁定;教师改分需走解锁/申诉流程。申诉改分自动触发 GPA 重算。</div>`;

// 学校配置
PAGES['s-config'] = () => head('学校配置','配置') + `
  <div class="grid" style="grid-template-columns:1fr 1fr">
    <div class="card card-pad">
      <div class="fw-600 mb-4">基本信息</div>
      <div class="field"><label>学校名称</label><input class="input" value="北京大学"></div>
      <div class="field"><label>学校短码</label><input class="input mono" value="pku" style="width:160px"></div>
      <div class="field"><label>校徽</label><button class="btn btn-outline"><i data-lucide="upload" style="width:14px;height:14px"></i> 上传</button></div>
    </div>
    <div class="card card-pad">
      <div class="fw-600 mb-4">SSO 统一认证</div>
      <div class="field"><label>认证协议</label><select class="select"><option>CAS</option><option>LDAP</option><option>不启用</option></select></div>
      <div class="field"><label>CAS 服务地址</label><input class="input" placeholder="https://sso.pku.edu.cn/cas"></div>
      <div class="field"><label>名单匹配字段</label><select class="select"><option>学号/工号</option><option>手机号</option></select></div>
      <button class="btn btn-outline"><i data-lucide="plug" style="width:14px;height:14px"></i> 测试连接</button>
      <div class="text-xs muted mt-3">SSO 仅核验身份,账号仍需先由管理员导入,匹配名单才放行。</div>
    </div>
  </div>`;

// 学校数据看板
PAGES['s-dashboard'] = () => head('数据看板','配置') + `
  <div class="grid grid-4 mb-4">
    ${statCard('users','3,404','师生总数','#3B82F6')}
    ${statCard('book-open','42','开设课程','#F59E0B')}
    ${statCard('flask-conical','156','活跃实验','#8B5CF6')}
    ${statCard('trophy','8','进行竞赛','#10B981')}
  </div>
  <div class="grid" style="grid-template-columns:2fr 1fr">
    <div class="card card-pad">
      <div class="fw-600 mb-3">平台活跃度趋势(近30天)</div>
      <div style="height:220px;display:flex;align-items:flex-end;gap:6px;padding:10px 0">
        ${Array.from({length:30}).map((_,i)=>{const h=30+Math.random()*150; return `<div style="flex:1;background:linear-gradient(var(--amber),var(--amber-100));border-radius:3px 3px 0 0;height:${h}px"></div>`}).join('')}
      </div>
    </div>
    <div class="card card-pad">
      <div class="fw-600 mb-3">资源用量</div>
      <div class="text-sm" style="line-height:1.6">
        <div class="flex justify-between mb-2"><span class="muted">沙箱并发</span><span class="fw-600">42 / 100</span></div>
        <div class="progress mb-3"><span style="width:42%"></span></div>
        <div class="flex justify-between mb-2"><span class="muted">CPU</span><span class="fw-600">58%</span></div>
        <div class="progress mb-3"><span style="width:58%"></span></div>
        <div class="flex justify-between mb-2"><span class="muted">存储</span><span class="fw-600">340 / 500 GB</span></div>
        <div class="progress"><span style="width:68%"></span></div>
      </div>
    </div>
  </div>`;

/* ============================================================
   平台管理员页面
   ============================================================ */

// 学校管理
PAGES['p-schools'] = () => head('学校管理','租户', `<button class="btn btn-primary"><i data-lucide="plus" style="width:15px;height:15px"></i> 录入学校</button>`) + `
  <div class="grid grid-4 mb-4">
    ${statCard('building-2','42','入驻学校','#F59E0B')}
    ${statCard('check-circle-2','38','正常运营','#10B981')}
    ${statCard('inbox','3','待审申请','#3B82F6')}
    ${statCard('pause-circle','1','已停用','#64748B')}
  </div>
  <table class="table card" style="overflow:hidden">
    <thead><tr><th>学校</th><th>短码</th><th>类型</th><th>师生数</th><th>到期</th><th>状态</th><th></th></tr></thead>
    <tbody>
    ${[['北京大学','pku','本科','3,404','2026-12-31','正常'],['清华大学','thu','本科','4,120','2026-12-31','正常'],['浙江大学','zju','本科','2,890','2026-08-30','正常'],['某职业学院','xyz','专科','1,200','2025-06-01','已停用']]
      .map(s=>`<tr><td class="fw-600">${s[0]}</td><td class="mono text-xs">${s[1]}</td><td>${s[2]}</td><td>${s[3]}</td><td class="muted">${s[4]}</td>
        <td><span class="badge ${s[5]==='正常'?'badge-green':'badge-gray'}">${s[5]}</span></td>
        <td class="flex gap-2"><button class="btn btn-ghost btn-sm">详情</button><button class="btn btn-ghost btn-sm">${s[5]==='正常'?'停用':'启用'}</button></td></tr>`).join('')}
    </tbody>
  </table>`;

// 入驻审核
PAGES['p-applications'] = () => head('入驻审核','租户') + `
  <div class="tabs"><div class="tab active">待审核 (3)</div><div class="tab">已通过</div><div class="tab">已驳回</div></div>
  <div class="grid grid-2">
    ${[['上海交通大学','本科','王老师','021-xxxx'],['复旦大学','本科','李老师','021-yyyy'],['某高职院校','专科','张老师','010-zzzz']]
      .map(a=>`<div class="card card-pad">
        <div class="flex items-center justify-between mb-3"><div class="fw-700" style="font-size:16px">${a[0]}</div><span class="badge badge-blue">${a[1]}</span></div>
        <div class="text-sm muted" style="line-height:1.9">联系人:${a[2]}<br>联系电话:${a[3]}<br>申请时间:2天前</div>
        <div class="flex gap-2 mt-3" style="padding-top:12px;border-top:1px solid var(--slate-100)">
          <button class="btn btn-primary btn-sm">通过(建租户)</button><button class="btn btn-outline btn-sm">驳回</button></div>
      </div>`).join('')}
  </div>
  <div class="text-sm muted mt-3"><i data-lucide="info" style="width:14px;height:14px;vertical-align:-2px"></i> 审核通过自动创建租户+分配短码+开通学校管理员账号(激活码方式)。私有化部署无此流程。</div>`;

// 运行时管理
PAGES['p-runtimes'] = () => head('运行时管理','引擎', `<button class="btn btn-primary"><i data-lucide="plus" style="width:15px;height:15px"></i> 接入新链</button>`) + `
  <div class="text-sm muted mb-3">区块链运行时数据驱动接入:加镜像 + 适配器清单 → 接入即测通过即可用。新链接入不改平台代码。</div>
  <table class="table card" style="overflow:hidden">
    <thead><tr><th>运行时</th><th>生态</th><th>适配器层级</th><th>镜像</th><th>自检</th><th>状态</th></tr></thead>
    <tbody>
    ${[['EVM · Hardhat','evm','L1声明式','runtime/evm-hardhat:v1.2','通过','可用'],['EVM · Foundry','evm','L1声明式','runtime/evm-foundry:v1.0','通过','可用'],['Hyperledger Fabric','fabric','L2标准接口','runtime/fabric:v2.5','通过','可用'],['FISCO BCOS','fisco','L2标准接口','runtime/fisco:v3.0','通过','可用'],['长安链','chainmaker','L2标准接口','runtime/chainmaker:v2.3','通过','可用'],['Solana','solana','L3深度插件','runtime/solana:v1.0','接入中','接入中']]
      .map(r=>`<tr><td class="fw-600">${r[0]}</td><td><span class="badge badge-gray">${r[1]}</span></td><td>${r[2]}</td><td class="mono text-xs">${r[3]}</td>
        <td><span class="badge ${r[4]==='通过'?'badge-green':'badge-amber'}">${r[4]}</span></td>
        <td><span class="badge ${r[5]==='可用'?'badge-green':'badge-amber'}">${r[5]}</span></td></tr>`).join('')}
    </tbody>
  </table>`;

// 判题器管理
PAGES['p-judgers'] = () => head('判题器管理','引擎', `<button class="btn btn-primary"><i data-lucide="plus" style="width:15px;height:15px"></i> 注册判题器</button>`) + `
  <table class="table card" style="overflow:hidden">
    <thead><tr><th>判题器</th><th>类型</th><th>需起链</th><th>执行器镜像</th><th>状态</th></tr></thead>
    <tbody>
    ${[['测试用例判题','testcase','是','judger/testcase-evm:v1.0','可用'],['链上状态断言','onchain-assert','是','judger/onchain-assert:v1.1','可用'],['Flag判题','flag','部分','judger/flag:v1.0','可用'],['静态安全扫描','static-scan','否','judger/static-scan:v1.0','可用'],['仿真检查点','sim-checkpoint','否','(依赖M4)','可用'],['人工评分','manual','否','—','可用']]
      .map(j=>`<tr><td class="fw-600">${j[0]}</td><td class="mono text-xs">${j[1]}</td><td>${j[2]}</td><td class="mono text-xs">${j[3]}</td>
        <td><span class="badge badge-green">${j[4]}</span></td></tr>`).join('')}
    </tbody>
  </table>`;

// 仿真场景库审核
PAGES['p-sim-lib'] = () => head('仿真场景库','引擎') + `
  <div class="tabs"><div class="tab active">已上架 (43)</div><div class="tab">待审核 (3)</div><div class="tab">已下架</div></div>
  <table class="table card" style="overflow:hidden">
    <thead><tr><th>场景</th><th>分类</th><th>作者</th><th>运行</th><th>版本</th><th>使用</th><th></th></tr></thead>
    <tbody>
    ${[['PoW 挖矿与51%攻击','共识','平台内置','前端','v1.0','156','已上架'],['PBFT 共识过程','共识','平台内置','前端','v1.2','98','已上架'],['自定义:Raft选举可视化','共识','李老师','前端','v1.0','—','待审核'],['默克尔树构建','密码学','平台内置','前端','v2.0','120','已上架']]
      .map(s=>`<tr><td class="fw-600">${s[0]}</td><td><span class="badge badge-gray">${s[1]}</span></td><td class="muted">${s[2]}</td><td>${s[3]}</td><td class="mono text-xs">${s[4]}</td><td>${s[5]}</td>
        <td>${s[6]==='待审核'?'<button class="btn btn-primary btn-sm">审核</button>':'<button class="btn btn-ghost btn-sm">详情</button>'}</td></tr>`).join('')}
    </tbody>
  </table>`;

// 漏洞源
PAGES['p-vuln'] = () => head('真实漏洞源','引擎', `<button class="btn btn-primary"><i data-lucide="plus" style="width:15px;height:15px"></i> 接入漏洞源</button>`) + `
  <div class="text-sm muted mb-3">多源可插拔接入,按可复现性分级。导入即固化为自包含题目存入题库,答题运行时零外部依赖。</div>
  <div class="grid grid-3 mb-4">
    ${[['SWC Registry','标准弱点分类','A级自动转题'],['SlowMist 漏洞情报','真实安全事件','A/B级'],['CVE 链上事件','真实DeFi攻击','forked复现']]
      .map(v=>`<div class="card card-pad">
        <div class="flex items-center gap-2 mb-2"><i data-lucide="shield-alert" style="width:18px;height:18px;color:var(--danger)"></i><span class="fw-600">${v[0]}</span></div>
        <div class="text-sm muted">${v[1]}</div><div class="badge badge-amber mt-2">${v[2]}</div>
        <button class="btn btn-outline btn-sm w-full mt-3" style="justify-content:center"><i data-lucide="refresh-cw" style="width:13px;height:13px"></i> 同步</button>
      </div>`).join('')}
  </div>
  <div class="fw-600 mb-3">待转化漏洞案例</div>
  <table class="table card" style="overflow:hidden">
    <thead><tr><th>漏洞</th><th>来源</th><th>分级</th><th>运行时</th><th>预验证</th><th></th></tr></thead>
    <tbody>
    ${[['重入攻击(The DAO 简化版)','SlowMist','A级','isolated','正向+反向通过'],['整数溢出','SWC-101','A级','isolated','正向+反向通过'],['闪电贷价格操纵','CVE案例','B级','forked','待补全']]
      .map(v=>`<tr><td class="fw-600">${v[0]}</td><td class="muted">${v[1]}</td><td><span class="badge ${v[2]==='A级'?'badge-green':'badge-amber'}">${v[2]}</span></td><td class="mono text-xs">${v[3]}</td>
        <td><span class="badge ${v[4].includes('通过')?'badge-green':'badge-gray'}">${v[4]}</span></td>
        <td><button class="btn btn-primary btn-sm">固化入库</button></td></tr>`).join('')}
    </tbody>
  </table>`;

// 系统配置
PAGES['p-config'] = () => head('系统配置','运维') + `
  <div class="grid grid-2">
    <div class="card card-pad">
      <div class="fw-600 mb-4">全局参数</div>
      <div class="field"><label>沙箱空闲回收(分钟)</label><input class="input" value="30"></div>
      <div class="field"><label>沙箱最长生命周期(分钟)</label><input class="input" value="240"></div>
      <div class="field"><label>判题超时(秒)</label><input class="input" value="120"></div>
      <button class="btn btn-primary">保存(乐观锁)</button>
    </div>
    <div class="card card-pad">
      <div class="fw-600 mb-4">配置变更记录</div>
      <div class="text-sm" style="line-height:2.2">
        <div class="flex justify-between"><span>判题超时 90→120</span><span class="muted text-xs">王管理 · 2天前</span></div>
        <div class="flex justify-between"><span>空闲回收 20→30</span><span class="muted text-xs">王管理 · 5天前</span></div>
      </div>
    </div>
  </div>`;

// 告警
PAGES['p-alerts'] = () => head('告警','运维') + `
  <div class="grid grid-4 mb-4">
    ${statCard('alert-octagon','2','严重','#EF4444')}
    ${statCard('alert-triangle','5','警告','#F59E0B')}
    ${statCard('bell','12','提示','#3B82F6')}
    ${statCard('activity','正常','服务状态','#10B981')}
  </div>
  <table class="table card" style="overflow:hidden">
    <thead><tr><th>级别</th><th>告警</th><th>来源</th><th>时间</th><th>状态</th><th></th></tr></thead>
    <tbody>
    ${[['严重','沙箱集群资源使用率>90%','基础监控','10分钟前','待处理'],['警告','北京大学待审申请积压','业务告警','1小时前','待处理'],['提示','判题队列积压>50','业务告警','2小时前','已处理']]
      .map(a=>`<tr><td><span class="badge ${a[0]==='严重'?'badge-red':a[0]==='警告'?'badge-amber':'badge-blue'}">${a[0]}</span></td><td class="fw-600">${a[1]}</td><td class="muted">${a[2]}</td><td class="muted">${a[3]}</td>
        <td><span class="badge ${a[4]==='待处理'?'badge-amber':'badge-gray'}">${a[4]}</span></td>
        <td><button class="btn btn-ghost btn-sm">处理</button></td></tr>`).join('')}
    </tbody>
  </table>
  <div class="text-sm muted mt-3"><i data-lucide="info" style="width:14px;height:14px;vertical-align:-2px"></i> 基础设施监控外接 Prometheus/Grafana,平台只做业务级告警与嵌入展示。</div>`;

// 审计中心
PAGES['p-audit'] = () => head('审计中心','运维', `<button class="btn btn-outline"><i data-lucide="download" style="width:15px;height:15px"></i> 导出</button>`) + `
  <div class="flex gap-3 mb-3" style="flex-wrap:wrap">
    <select class="select" style="width:140px"><option>全部操作</option><option>account.import</option><option>judge.run</option><option>grade.override</option></select>
    <input class="input" placeholder="操作人" style="width:140px">
    <input class="input" type="text" value="2026-05-01 ~ 05-29" style="width:200px">
    <button class="btn btn-outline">查询</button>
  </div>
  <table class="table card" style="overflow:hidden">
    <thead><tr><th>时间</th><th>操作人</th><th>角色</th><th>动作</th><th>对象</th><th>IP</th></tr></thead>
    <tbody>
    ${[['05-29 14:32','李老师','学校管理员','account.import','导入学生128人','10.0.1.5'],['05-29 14:10','王管理','平台管理员','tenant.approve','审核通过 上海交大','10.0.1.2'],['05-29 13:55','张老师','教师','grade.override','调整成绩 张同学','10.0.2.8'],['05-29 13:40','系统','—','judge.run','判题任务#8801','—']]
      .map(a=>`<tr><td class="muted">${a[0]}</td><td class="fw-600">${a[1]}</td><td><span class="badge badge-gray">${a[2]}</span></td><td class="mono text-xs">${a[3]}</td><td>${a[4]}</td><td class="mono text-xs muted">${a[5]}</td></tr>`).join('')}
    </tbody>
  </table>
  <div class="text-sm muted mt-3"><i data-lucide="info" style="width:14px;height:14px;vertical-align:-2px"></i> 全平台统一审计表(M1),各模块写入,此处多维查询。</div>`;

// 平台数据看板
PAGES['p-dashboard'] = () => head('数据看板','运维') + `
  <div class="grid grid-4 mb-4">
    ${statCard('building-2','42','入驻学校','#F59E0B')}
    ${statCard('users','128K','平台用户','#3B82F6')}
    ${statCard('flask-conical','2,340','今日实验','#8B5CF6')}
    ${statCard('trophy','18','进行竞赛','#10B981')}
  </div>
  <div class="grid" style="grid-template-columns:2fr 1fr">
    <div class="card card-pad">
      <div class="fw-600 mb-3">平台用量趋势</div>
      <div style="height:220px;display:flex;align-items:flex-end;gap:5px">
        ${Array.from({length:30}).map(()=>{const h=40+Math.random()*160; return `<div style="flex:1;background:linear-gradient(var(--amber),var(--amber-100));border-radius:3px 3px 0 0;height:${h}px"></div>`}).join('')}
      </div>
    </div>
    <div class="card card-pad">
      <div class="fw-600 mb-3">服务健康(外接)</div>
      <div class="text-sm" style="line-height:2.4">
        ${[['PostgreSQL','正常'],['Redis','正常'],['Kubernetes','正常'],['MinIO','正常'],['NATS','正常']]
          .map(s=>`<div class="flex justify-between"><span class="muted">${s[0]}</span><span class="badge badge-green"><i data-lucide="check" style="width:12px;height:12px"></i> ${s[1]}</span></div>`).join('')}
      </div>
    </div>
  </div>`;

/* ============================================================
   启动
   ============================================================ */
window.addEventListener('DOMContentLoaded', init);
