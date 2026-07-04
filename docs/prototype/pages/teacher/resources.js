/* ============================================================
   pages/teacher/resources.js — 教师·资源域(题库 / 组卷 / 仿真 / 共享库 / 成绩报送 / 组织 / 消息)
   ------------------------------------------------------------
   覆盖:题库(分类树+多维筛选+卡片)、内容创作编辑器(草稿持久化)、
        组卷(手动/随机)、组卷编辑、仿真场景、共享库(跨校克隆)、
        成绩报送(权重校验+计算+调分+导出+送审)、组织架构(只读)、
        站内信 / 系统公告 / 个人中心。对应 M5 内容、M9 成绩、M10 组织。
   范式:列表 C.head、子页 C.crumb + C.parentRoute;沿用 student 范式。
   ============================================================ */
(function () {
  const C = window.Chaimir;
  const m = C.mock;

  /* 子页 → 高亮的侧栏项 */
  Object.assign(C.parentRoute, {
    'teacher/content-edit': 'teacher/content',
    'teacher/paper-edit': 'teacher/papers',
  });

  /* ============================================================
     1) 题库(分类树 + 多维筛选 + 内容卡片)
     ============================================================ */
  /* 在共享 content 上补充更丰富的题库数据(来源徽章:本校/系统/外部源)*/
  const contentItems = [
    { id: 1, title: '重入漏洞利用与防护', type: '实验模板', cat: '合约安全', difficulty: '进阶', author: '李明远', source: '本校', version: 'v1.3.0', status: '已发布', usage: 12, tags: ['重入', 'CEI'] },
    { id: 2, title: '金库重入渗透 CTF', type: '竞赛题', cat: '合约安全', difficulty: '高级', author: '系统入库', source: '系统', version: 'v1.0.0', status: '已发布', usage: 5, tags: ['CTF', '攻防'] },
    { id: 3, title: 'PBFT 三阶段共识选择题组', type: '理论题', cat: '共识算法', difficulty: '入门', author: '陈雪', source: '本校', version: 'v2.0.0', status: '草稿', usage: 0, tags: ['PBFT', '共识'] },
    { id: 4, title: '闪电贷套利与价格操纵', type: '实验模板', cat: 'DeFi', difficulty: '高级', author: '王思齐', source: '本校', version: 'v1.1.0', status: '已发布', usage: 8, tags: ['闪电贷', '预言机'] },
    { id: 5, title: 'SWC-107 重入(外部源转化)', type: '竞赛题', cat: '合约安全', difficulty: '高级', author: '外部源', source: '外部源', version: 'v1.0.0', status: '已发布', usage: 3, tags: ['SWC', '重入'] },
    { id: 6, title: '默克尔树与轻节点验证', type: '理论题', cat: '密码学', difficulty: '进阶', author: '陈雪', source: '本校', version: 'v1.0.0', status: '已发布', usage: 6, tags: ['默克尔', '验证'] },
  ];
  C.tContent = contentItems;

  /* 分类树(知识点) */
  const cats = [
    { name: '全部', count: contentItems.length },
    { name: '合约安全', count: 3 }, { name: 'DeFi', count: 1 },
    { name: '共识算法', count: 1 }, { name: '密码学', count: 1 },
  ];
  const sourceBadge = (s) => C.badge(s, { '本校': 'gray', '系统': 'blue', '外部源': 'purple' }[s] || 'gray');
  const typeBadge = (t) => C.badge(t, { '实验模板': 'purple', '竞赛题': 'red', '理论题': 'blue' }[t] || 'gray');

  function contentList(ctx) {
    const cat = ctx.query.cat || '全部';
    const rows = contentItems.filter(c => cat === '全部' || c.cat === cat);
    return `${C.head('题库', '资源',
      `<button class="btn btn-outline" onclick="Chaimir.navigate('teacher/vuln-sources')">${C.icon('shield')} 漏洞源</button>
       <button class="btn btn-primary" onclick="Chaimir.navigate('teacher/content-edit')">${C.icon('plus')} 新建内容</button>`)}
      <div class="grid" style="grid-template-columns:220px 1fr;align-items:start">
        <div class="card card-pad">
          <div class="section-title mb-3">分类</div>
          ${cats.map(c => `<a class="side-item ${c.name === cat ? 'active' : ''}" style="border-radius:var(--radius-sm)" onclick="Chaimir.navigate('teacher/content?cat=${encodeURIComponent(c.name)}')">
            ${C.icon('folder')}<span style="flex:1">${c.name}</span><span class="count">${c.count}</span></a>`).join('')}
          <div class="divider"></div>
          <div class="section-title mb-2">知识点标签</div>
          <div class="flex gap-2 wrap">${['重入', 'CEI', '共识', '闪电贷', 'CTF', '默克尔'].map(t => `<span class="badge badge-gray" style="cursor:pointer" onclick="Chaimir.demo('按标签筛选:${t}')">${t}</span>`).join('')}</div>
        </div>
        <div>
          <div class="card card-pad mb-4">
            <div class="grid grid-4" style="gap:10px">
              <div class="input-icon" style="grid-column:span 2">${C.icon('search')}<input class="input" placeholder="搜索标题 / 关键词"></div>
              <select class="select" onchange="Chaimir.demo('按类型筛选')"><option>全部类型</option><option>实验模板</option><option>竞赛题</option><option>理论题</option></select>
              <select class="select" onchange="Chaimir.demo('按难度筛选')"><option>全部难度</option><option>入门</option><option>进阶</option><option>高级</option></select>
            </div>
            <div class="flex gap-2 mt-3 wrap">
              <select class="select" style="width:auto" onchange="Chaimir.demo('按可见性筛选')"><option>全部可见性</option><option>本校</option><option>仅自己</option></select>
              <select class="select" style="width:auto" onchange="Chaimir.demo('按状态筛选')"><option>全部状态</option><option>已发布</option><option>草稿</option></select>
            </div>
          </div>
          <div class="grid grid-3">${rows.map(c => `
            <div class="card card-hover card-pad" onclick="Chaimir.navigate('teacher/content-edit?id=${c.id}')">
              <div class="flex justify-between items-center mb-2">${typeBadge(c.type)}${sourceBadge(c.source)}</div>
              <div class="fw-700" style="font-size:var(--text-md);line-height:1.4">${c.title}</div>
              <div class="muted text-xs mt-2">${c.author} · ${c.cat}</div>
              <div class="flex gap-2 mt-3 wrap">${C.badge(c.difficulty, 'amber')}<span class="badge badge-gray mono">${c.version}</span>${C.badge(c.status, c.status === '已发布' ? 'green' : 'gray')}</div>
              <div class="muted text-xs mt-3 flex items-center gap-2">${C.icon('repeat')} 已复用 ${c.usage} 次</div>
            </div>`).join('')}</div>
          ${C.pagination(1, contentItems.length)}
        </div>
      </div>`;
  }

  /* ---------- 内容创作编辑器(草稿持久化)---------- */
  /* 自动保存指示(复用 student 同款节奏) */
  C.tContentSave = function (manual) {
    const ind = document.getElementById('content-autosave'); if (!ind) return;
    ind.className = 'autosave saving'; ind.innerHTML = C.icon('loader') + ' 正在保存…'; C.refreshIcons();
    setTimeout(() => { ind.className = 'autosave saved'; ind.innerHTML = C.icon('cloud') + ' 草稿已保存到服务端'; C.refreshIcons(); if (manual) C.toast('success', '草稿已保存', '换设备或刷新都不会丢失'); }, 600);
  };
  C.mounts['teacher/content-edit'] = function () {
    C._contentTimer && clearInterval(C._contentTimer);
    C._contentTimer = setInterval(() => { if (location.hash.includes('teacher/content-edit')) C.tContentSave(false); else clearInterval(C._contentTimer); }, 60000);
  };
  function contentEdit(ctx) {
    const c = ctx.query.id ? contentItems.find(x => x.id == ctx.query.id) : null;
    const isNew = !c;
    const type = (c && c.type) || '实验模板';
    /* 按类型渲染不同的内容体表单 */
    let typeBody = '';
    if (type === '实验模板') {
      typeBody = `<div class="field"><label>引用运行时(M2)</label><select class="select"><option>EVM · Foundry</option><option>EVM · Hardhat</option></select></div>
        <div class="field"><label>初始脚手架</label><textarea class="textarea mono" style="min-height:120px" placeholder="// 学生初始代码…">// Vault.sol(待修复)
function withdraw() public { /* ... */ }</textarea></div>
        <div class="field" style="margin-bottom:0"><label class="flex items-center gap-2">判题配置 ${C.badge('答案-学生不可见', 'red')}</label>
          <div class="callout warn">${C.icon('eye-off')}<div>测试用例、链上断言、flag 属于敏感字段,仅用于判题,学生侧不可见。</div></div></div>`;
    } else if (type === '竞赛题') {
      typeBody = `<div class="field"><label>对局/解题模式</label><select class="select"><option>对抗题(攻防)</option><option>对抗题(博弈)</option><option>解题</option></select></div>
        <div class="field"><label>动态分曲线</label><select class="select"><option>随解出人数衰减</option><option>固定分值</option></select></div>
        <div class="field" style="margin-bottom:0"><label class="flex items-center gap-2">Flag / 判题脚本 ${C.badge('答案-学生不可见', 'red')}</label>
          <div class="callout warn">${C.icon('eye-off')}<div>flag 与判题脚本对选手不可见,提交时由系统比对。</div></div></div>`;
    } else {
      typeBody = `<div class="field"><label>题型</label><select class="select"><option>单选</option><option>多选</option><option>判断</option><option>简答</option></select></div>
        <div class="field"><label>选项</label>
          ${['A', 'B', 'C', 'D'].map(o => `<div class="flex items-center gap-2 mb-2"><span class="fw-600">${o}</span><input class="input" placeholder="选项内容"></div>`).join('')}</div>
        <div class="field" style="margin-bottom:0"><label class="flex items-center gap-2">正确答案 ${C.badge('答案-学生不可见', 'red')}</label><input class="input" placeholder="如 A" style="max-width:120px"></div>`;
    }
    return `${C.crumb([{ label: '题库', to: 'teacher/content' }, { label: isNew ? '新建内容' : '编辑内容' }])}
      ${C.head(isNew ? '新建内容' : '编辑内容', c ? c.title : '统一元信息 + 按类型的内容体',
        `<span class="autosave saved" id="content-autosave">${C.icon('cloud')} 草稿已保存到服务端</span>
         <button class="btn btn-outline" onclick="Chaimir.tContentSave(true)">${C.icon('save')} 存草稿</button>
         <button class="btn btn-primary" onclick="Chaimir.toast('success','已发布','内容已生成新版本并可被引用');setTimeout(()=>Chaimir.navigate('teacher/content'),700)">${C.icon('send')} 发布</button>`)}
      <div class="grid" style="grid-template-columns:1fr 300px">
        <div>
          <div class="card card-pad mb-3"><div class="section-title mb-3">统一元信息</div>
            <div class="field"><label>标题<span class="req">*</span></label><input class="input" value="${c ? C.esc(c.title) : ''}" placeholder="如:重入漏洞利用与防护"></div>
            <div class="grid grid-2">
              <div class="field"><label>内容类型<span class="req">*</span></label>
                <select class="select" onchange="Chaimir.demo('切换内容类型将切换下方表单')"><option ${type === '实验模板' ? 'selected' : ''}>实验模板</option><option ${type === '竞赛题' ? 'selected' : ''}>竞赛题</option><option ${type === '理论题' ? 'selected' : ''}>理论题</option></select></div>
              <div class="field"><label>难度</label><select class="select"><option>入门</option><option ${c && c.difficulty === '进阶' ? 'selected' : ''}>进阶</option><option ${c && c.difficulty === '高级' ? 'selected' : ''}>高级</option></select></div>
              <div class="field"><label>分类 / 知识点</label><select class="select"><option>合约安全</option><option>DeFi</option><option>共识算法</option><option>密码学</option></select></div>
              <div class="field"><label>标签(逗号分隔)</label><input class="input" value="${c ? c.tags.join(', ') : ''}" placeholder="重入, CEI"></div>
            </div>
            <div class="field" style="margin-bottom:0"><label>简介</label><textarea class="textarea" placeholder="一句话说明这道题/模板考察什么…"></textarea></div>
          </div>
          <div class="card card-pad"><div class="section-title mb-3">内容体 · ${type}</div>${typeBody}</div>
        </div>
        <div>
          <div class="card card-pad mb-3"><div class="section-title mb-2">版本</div>
            <dl class="dl"><dt>当前版本</dt><dd class="mono">${c ? c.version : 'v1.0.0(草稿)'}</dd><dt>复用次数</dt><dd>${c ? c.usage : 0}</dd></dl>
            <div class="callout info mt-2">${C.icon('info')}<div>发布会生成新版本号;被引用的作业/竞赛锁定其引用时的版本。</div></div>
          </div>
          <div class="card card-pad"><div class="section-title mb-2">可见性</div>
            <label class="radio mb-2" style="display:flex"><input type="radio" name="cvis" checked> 本校可见(可被复用)</label>
            <label class="radio mb-2" style="display:flex"><input type="radio" name="cvis"> 仅自己</label>
            <label class="radio" style="display:flex"><input type="radio" name="cvis"> 申请共享到跨校共享库</label>
          </div>
        </div>
      </div>`;
  }

  /* ============================================================
     2) 组卷(列表 + 编辑)
     ============================================================ */
  const papers = [
    { id: 1, name: '区块链期中卷 A', mode: '手动选题', count: 12, score: 100, status: '已定稿', updated: '2026-05-18' },
    { id: 2, name: '合约安全随机抽题卷', mode: '随机抽题', count: 10, score: 100, status: '草稿', updated: '2026-06-04' },
    { id: 3, name: '共识算法测验卷', mode: '手动选题', count: 8, score: 50, status: '已定稿', updated: '2026-04-30' },
  ];
  C.tPapers = papers;
  function papersList() {
    return `${C.head('组卷', '资源', `<button class="btn btn-primary" onclick="Chaimir.tNewPaper()">${C.icon('plus')} 创建组卷</button>`)}
      <div class="table-wrap"><table class="table">
        <thead><tr><th>试卷名称</th><th>组卷方式</th><th>题量</th><th>总分</th><th>状态</th><th>更新时间</th><th style="text-align:right">操作</th></tr></thead>
        <tbody>${papers.map(p => `<tr>
          <td class="fw-600">${p.name}</td>
          <td>${C.badge(p.mode, p.mode === '随机抽题' ? 'purple' : 'gray')}</td>
          <td class="mono">${p.count}</td><td class="mono">${p.score}</td>
          <td>${C.badge(p.status, p.status === '已定稿' ? 'green' : 'gray')}</td>
          <td class="mono text-xs">${p.updated}</td>
          <td class="row-actions">
            <button class="btn btn-outline btn-sm" onclick="Chaimir.navigate('teacher/paper-edit?id=${p.id}')">${C.icon('pencil')} 编辑</button>
            <button class="btn btn-ghost btn-sm" onclick="Chaimir.demo('预览题面(过滤答案)')">${C.icon('eye')} 预览</button>
          </td></tr>`).join('')}</tbody></table></div>`;
  }
  C.tNewPaper = function () {
    C.modal({
      title: '创建组卷',
      body: `<div class="field"><label>试卷名称<span class="req">*</span></label><input class="input" placeholder="如:区块链期末卷 A"></div>
        <div class="field" style="margin-bottom:0"><label>组卷方式</label>
          <label class="radio mb-2" style="display:flex;padding:11px;border:1px solid var(--color-primary);border-radius:var(--radius-sm);background:var(--color-primary-soft)"><input type="radio" name="pm" checked> 手动选题(逐题挑选、设分值题序)</label>
          <label class="radio" style="display:flex;padding:11px;border:1px solid var(--color-border);border-radius:var(--radius-sm)"><input type="radio" name="pm"> 按条件随机抽题(知识点/难度/数量)</label></div>`,
      foot: `<button class="btn btn-outline" onclick="document.querySelector('.overlay').remove()">取消</button>
             <button class="btn btn-primary" onclick="document.querySelector('.overlay').remove();Chaimir.navigate('teacher/paper-edit')">下一步</button>`,
    });
  };
  function paperEdit(ctx) {
    const p = ctx.query.id ? papers.find(x => x.id == ctx.query.id) : null;
    const isNew = !p;
    const random = p && p.mode === '随机抽题';
    const qs = [
      { no: 1, t: '重入攻击根本防护方式', type: '单选', score: 5 },
      { no: 2, t: 'CEI 模式执行顺序', type: '简答', score: 15 },
      { no: 3, t: '修复合约重入漏洞', type: '编程', score: 30 },
    ];
    return `${C.crumb([{ label: '组卷', to: 'teacher/papers' }, { label: isNew ? '创建组卷' : '组卷编辑' }])}
      ${C.head(isNew ? '创建组卷' : '组卷编辑', p ? p.name : '手动逐题选或按条件随机抽题',
        `<button class="btn btn-outline" onclick="Chaimir.demo('题面视角预览(已过滤答案)')">${C.icon('eye')} 题面预览</button>
         <button class="btn btn-primary" onclick="Chaimir.toast('success','已保存','试卷已保存');setTimeout(()=>Chaimir.navigate('teacher/papers'),700)">${C.icon('save')} 保存定稿</button>`)}
      <div class="grid" style="grid-template-columns:1fr 320px">
        <div class="card mb-3">
          <div class="card-head"><div class="section-title">题目(共 ${qs.length} 题 · 50 分)</div>
            <button class="btn btn-primary btn-sm" onclick="Chaimir.tPickQuestions ? Chaimir.tPickQuestions() : Chaimir.demo('从题库选题')">${C.icon('library')} 选题</button></div>
          <div style="padding:8px">${qs.map(q => `<div class="side-item" style="border-radius:var(--radius-sm)">
            <span style="cursor:grab;color:var(--color-text-faint)">${C.icon('grip-vertical')}</span>
            <span class="muted text-xs mono">${q.no}</span><span style="flex:1">${q.t}</span>
            ${C.badge(q.type, q.type === '编程' ? 'purple' : q.type === '简答' ? 'amber' : 'blue')}
            <input class="input" style="width:60px;text-align:center" type="number" value="${q.score}">
            <button class="btn btn-ghost btn-sm btn-icon" onclick="Chaimir.demo('移除')">${C.icon('x')}</button></div>`).join('')}</div>
        </div>
        <div>
          ${random ? `<div class="card card-pad mb-3"><div class="section-title mb-3">随机抽题条件</div>
            <div class="field"><label>知识点</label><select class="select"><option>合约安全</option><option>共识算法</option></select></div>
            <div class="field"><label>难度</label><select class="select"><option>进阶</option><option>高级</option></select></div>
            <div class="field"><label>抽取数量</label><input class="input" type="number" value="10"></div>
            <button class="btn btn-outline btn-block" onclick="Chaimir.toast('success','已重新抽题','已按条件随机生成 10 题')">${C.icon('shuffle')} 重新抽题</button>
          </div>` : ''}
          <div class="card card-pad"><div class="section-title mb-2">试卷设置</div>
            <div class="field"><label>试卷名称</label><input class="input" value="${p ? C.esc(p.name) : ''}"></div>
            <div class="field" style="margin-bottom:0"><label>说明</label><textarea class="textarea" placeholder="考试须知…"></textarea></div>
          </div>
        </div>
      </div>`;
  }

  /* ============================================================
     3) 仿真场景
     ============================================================ */
  const simPkgs = [
    { id: 1, name: '重入攻击调用栈可视化', scope: '本校', status: '已上架', author: '李明远', usage: 9 },
    { id: 2, name: 'PBFT 三阶段投票矩阵', scope: '平台', status: '已上架', author: '系统', usage: 22 },
    { id: 3, name: '默克尔树构建动画', scope: '本校', status: '审核中', author: '陈雪', usage: 0 },
    { id: 4, name: '跨链桥消息传递仿真', scope: '本校', status: '草稿', author: '王思齐', usage: 0 },
  ];
  function simPackages() {
    return `${C.head('仿真场景', '资源', `<button class="btn btn-primary" onclick="Chaimir.tUploadSim()">${C.icon('upload')} 上传自定义场景</button>`)}
      <div class="callout info mb-4">${C.icon('info')}<div>本校自定义仿真包需先在沙箱预览自测,再提交平台审核;审核通过后方可在实验/课时中引用。</div></div>
      <div class="grid grid-3">${simPkgs.map(s => `
        <div class="card card-pad">
          <div class="flex justify-between items-center mb-2">${C.badge(s.scope, s.scope === '平台' ? 'blue' : 'gray')}${C.badge(s.status, { '已上架': 'green', '审核中': 'amber', '草稿': 'gray' }[s.status])}</div>
          <div class="flex items-center gap-2 mb-2">${C.icon('activity')}<span class="fw-700">${s.name}</span></div>
          <div class="muted text-xs">${s.author} · 已引用 ${s.usage} 次</div>
          <div class="flex gap-2 mt-3">
            <button class="btn btn-outline btn-sm" onclick="Chaimir.navigate('immersive/sim')">${C.icon('play')} 沙箱预览</button>
            ${s.status === '草稿' ? `<button class="btn btn-primary btn-sm" onclick="Chaimir.toast('success','已提交审核','平台管理员将在 1-2 个工作日内审核')">${C.icon('send')} 提交审核</button>` : ''}
          </div>
        </div>`).join('')}</div>`;
  }
  C.tUploadSim = function () {
    C.modal({
      title: '上传自定义仿真场景',
      body: `<div class="field"><label>场景名称<span class="req">*</span></label><input class="input" placeholder="如:跨链桥消息传递仿真"></div>
        <div class="field"><label>场景包(.zip)</label>
          <div style="border:2px dashed var(--color-border-strong);border-radius:var(--radius);padding:24px;text-align:center;color:var(--color-text-faint);cursor:pointer" onclick="Chaimir.demo('选择文件')">
            ${C.icon('upload-cloud')}<div class="text-sm mt-2">点击或拖拽上传仿真包</div><div class="text-xs">含场景定义 / 资源 / 参数 schema</div></div></div>
        <div class="field" style="margin-bottom:0"><label>简介</label><textarea class="textarea" placeholder="说明仿真目标与交互方式…"></textarea></div>`,
      foot: `<button class="btn btn-outline" onclick="document.querySelector('.overlay').remove()">取消</button>
             <button class="btn btn-primary" onclick="document.querySelector('.overlay').remove();Chaimir.toast('success','已上传为草稿','请进入沙箱预览自测后提交审核')">上传</button>`,
    });
  };

  /* ============================================================
     4) 共享库(跨校浏览 / 克隆)
     ============================================================ */
  const sharedItems = [
    { id: 1, title: '以太坊重入攻击全景实验', from: '示例大学', type: '实验模板', difficulty: '进阶', clones: 48 },
    { id: 2, title: 'DeFi 闪电贷攻防 CTF 题集', from: '滨海理工大学', type: '竞赛题', difficulty: '高级', clones: 31 },
    { id: 3, title: 'PBFT / Raft 共识对比题库', from: '北辰大学', type: '理论题', difficulty: '进阶', clones: 27 },
  ];
  function sharedLib() {
    return `${C.head('共享库', '资源', `<div class="input-icon" style="width:260px">${C.icon('search')}<input class="input" placeholder="检索跨校共享内容"></div>`)}
      <div class="callout info mb-4">${C.icon('share-2')}<div>共享库汇集各校公开的优质内容。<b>一键克隆</b>会把内容<b>深拷贝</b>到本校题库,与源完全解耦 —— 源方后续修改不影响你的副本,你的修改也不回写源方。</div></div>
      <div class="grid grid-3">${sharedItems.map(s => `
        <div class="card card-pad">
          <div class="flex justify-between items-center mb-2">${typeBadge(s.type)}${C.badge(s.difficulty, 'amber')}</div>
          <div class="fw-700" style="font-size:var(--text-md);line-height:1.4">${s.title}</div>
          <div class="muted text-xs mt-2">${C.icon('building')} 来自 ${s.from} · 被克隆 ${s.clones} 次</div>
          <div class="flex gap-2 mt-3">
            <button class="btn btn-outline btn-sm" onclick="Chaimir.demo('预览题面(已过滤答案)')">${C.icon('eye')} 预览</button>
            <button class="btn btn-primary btn-sm" onclick="Chaimir.tCloneShared('${C.esc(s.title)}')">${C.icon('copy')} 克隆到本校</button>
          </div>
        </div>`).join('')}</div>`;
  }
  C.tCloneShared = async function (title) {
    if (await C.confirm({ title: '克隆到本校题库', message: '将「' + title + '」深拷贝到本校题库,与源解耦。克隆后可自由编辑且不影响源方。确认克隆?', confirmText: '克隆' })) {
      C.toast('success', '已克隆到本校', '可在题库中找到该副本并编辑');
      setTimeout(() => C.navigate('teacher/content'), 800);
    }
  };

  /* ============================================================
     5) 成绩报送(权重校验 + 计算 + 调分 + 导出 + 送审)
     ============================================================ */
  /* 班级成绩(总评由各项加权;和必须=100%) */
  const gradeRows = [
    { name: '林思远', no: '2023210456', hw: 88, exp: 92, exam: 85, total: 88 },
    { name: '赵雨桐', no: '2023210457', hw: 90, exp: 86, exam: 88, total: 88 },
    { name: '孙浩然', no: '2023210458', hw: 72, exp: 65, exam: 70, total: 69 },
    { name: '周晓彤', no: '2023210459', hw: 95, exp: 90, exam: 92, total: 92 },
    { name: '吴俊杰', no: '2023210460', hw: 80, exp: 88, exam: 76, total: 82 },
  ];
  function gradeSubmit(ctx) {
    return `${C.head('成绩报送', '成绩',
      `<button class="btn btn-outline" onclick="Chaimir.demo('导出成绩 Excel')">${C.icon('download')} 导出 Excel</button>
       <button class="btn btn-primary" onclick="Chaimir.tSubmitGrades()">${C.icon('send')} 提交审核</button>`)}
      <div class="card card-pad mb-4 flex items-center justify-between wrap gap-3">
        <div><div class="muted text-xs">报送课程</div><div class="fw-700" style="font-size:var(--text-lg)">区块链原理与智能合约开发 · 区块链 2301 班</div></div>
        <select class="select" style="width:280px" onchange="Chaimir.demo('切换课程')"><option>区块链原理与智能合约开发</option><option>密码学基础与共识算法</option></select>
      </div>
      <div class="card card-pad mb-4">
        <div class="flex justify-between items-center mb-3"><div class="section-title">成绩权重</div>
          <span class="autosave saved" id="weight-ind">${C.icon('check-circle-2')} 权重合计 100%</span></div>
        <div class="grid grid-3">
          <div class="field" style="margin-bottom:0"><label>作业(%)</label><input class="input" type="number" value="40" oninput="Chaimir.tCheckWeight()" id="w-hw"></div>
          <div class="field" style="margin-bottom:0"><label>实验(%)</label><input class="input" type="number" value="40" oninput="Chaimir.tCheckWeight()" id="w-exp"></div>
          <div class="field" style="margin-bottom:0"><label>考试(%)</label><input class="input" type="number" value="20" oninput="Chaimir.tCheckWeight()" id="w-exam"></div>
        </div>
        <div class="flex justify-end mt-3"><button class="btn btn-outline" onclick="Chaimir.tCalcGrades()">${C.icon('calculator')} 触发全班计算</button></div>
      </div>
      <div class="grid grid-4 mb-4">
        ${C.stat('users', gradeRows.length, '应报送人数', 'blue')}
        ${C.stat('award', '92', '最高分', 'green')}
        ${C.stat('trending-up', '83.8', '平均分', 'amber')}
        ${C.stat('alert-triangle', '1', '不及格', 'red')}
      </div>
      <div class="table-wrap"><table class="table">
        <thead><tr><th>姓名</th><th>学号</th><th>作业(40%)</th><th>实验(40%)</th><th>考试(20%)</th><th>总评</th><th style="text-align:right">操作</th></tr></thead>
        <tbody>${gradeRows.map(r => `<tr>
          <td class="fw-600">${r.name}</td><td class="mono">${r.no}</td>
          <td class="mono">${r.hw}</td><td class="mono">${r.exp}</td><td class="mono">${r.exam}</td>
          <td><span class="fw-700" style="color:var(--${r.total < 60 ? 'red' : r.total >= 85 ? 'green' : 'amber'}-700)">${r.total}</span></td>
          <td class="row-actions"><button class="btn btn-ghost btn-sm" onclick="Chaimir.tAdjustGrade('${C.esc(r.name)}', ${r.total})">${C.icon('pencil')} 调分</button></td>
        </tr>`).join('')}</tbody></table></div>
      <div class="callout warn mt-4">${C.icon('lock')}<div>提交审核后成绩交由<b>学校管理员锁定</b>;锁定后如需修改须走成绩复核流程。手动调分会记入审计日志。</div></div>`;
  }
  /* 权重和=100% 校验 */
  C.tCheckWeight = function () {
    const v = (id) => parseInt((document.getElementById(id) || {}).value || '0', 10) || 0;
    const sum = v('w-hw') + v('w-exp') + v('w-exam');
    const ind = document.getElementById('weight-ind'); if (!ind) return;
    if (sum === 100) { ind.className = 'autosave saved'; ind.innerHTML = C.icon('check-circle-2') + ' 权重合计 100%'; }
    else { ind.className = 'autosave saving'; ind.innerHTML = C.icon('alert-triangle') + ' 权重合计 ' + sum + '%,需等于 100%'; }
    C.refreshIcons();
  };
  C.tCalcGrades = function () {
    const v = (id) => parseInt((document.getElementById(id) || {}).value || '0', 10) || 0;
    const sum = v('w-hw') + v('w-exp') + v('w-exam');
    if (sum !== 100) { C.toast('error', '权重不合法', '作业+实验+考试权重之和必须等于 100%'); return; }
    C.toast('success', '已触发计算', '全班总评已按当前权重重新计算');
  };
  C.tAdjustGrade = function (name, cur) {
    C.modal({
      title: '调分 · ' + name,
      body: `<div class="field"><label>当前总评</label><input class="input" value="${cur}" disabled></div>
        <div class="field"><label>调整后总评<span class="req">*</span></label><input class="input" type="number" value="${cur}"></div>
        <div class="field" style="margin-bottom:0"><label>调分原因(记入审计)<span class="req">*</span></label><textarea class="textarea" placeholder="如:补交实验报告,酌情加分…"></textarea></div>`,
      foot: `<button class="btn btn-outline" onclick="document.querySelector('.overlay').remove()">取消</button>
             <button class="btn btn-primary" onclick="document.querySelector('.overlay').remove();Chaimir.toast('success','已调分','调整已记录,记入审计日志')">保存调分</button>`,
    });
  };
  C.tSubmitGrades = async function () {
    if (await C.confirm({ title: '提交成绩审核', message: '提交后成绩将送学校管理员锁定,期间不可修改。确认本班 ' + gradeRows.length + ' 人成绩无误并提交?', confirmText: '确认提交' }))
      C.toast('success', '已提交审核', '学校管理员审核锁定后,成绩对学生公布');
  };

  /* ============================================================
     6) 组织架构(只读)
     ============================================================ */
  /* 教师只读全校院系/专业/班级树,无增删改 */
  const orgTree = [
    { name: '计算机学院', majors: [
      { name: '区块链工程', classes: ['区块链 2301(46)', '区块链 2302(44)'] },
      { name: '软件工程', classes: ['软工 2301(50)', '软工 2302(48)'] } ] },
    { name: '网络空间安全学院', majors: [
      { name: '信息安全', classes: ['信安 2301(40)', '信安 2302(38)'] } ] },
  ];
  function org() {
    return `${C.head('组织架构', '组织', C.badge('只读', 'gray'))}
      <div class="callout info mb-4">${C.icon('info')}<div>组织架构由学校管理员维护,教师端仅供查看(无增删改)。如需调整请联系学校管理员。</div></div>
      ${orgTree.map(col => `
        <div class="card mb-3"><div class="card-head"><div class="section-title flex items-center gap-2">${C.icon('building')} ${col.name}</div>
          <span class="muted text-xs">${col.majors.length} 个专业</span></div>
          <div class="card-pad">${col.majors.map(maj => `
            <div class="mb-3"><div class="fw-600 mb-2 flex items-center gap-2">${C.icon('graduation-cap')} ${maj.name}</div>
              <div class="flex gap-2 wrap" style="padding-left:24px">${maj.classes.map(cl => `<span class="badge badge-gray">${C.icon('users')} ${cl}</span>`).join('')}</div></div>`).join('')}</div>
        </div>`).join('')}`;
  }

  /* ============================================================
     7) 站内信 / 系统公告 / 个人中心(教师语境)
     ============================================================ */
  const teacherNotis = [
    { type: '批改', title: '《智能合约安全作业》有 64 份提交待批改', read: false, time: '15 分钟前', link: 'teacher/grading' },
    { type: '查重', title: '检测到 1 份高相似度提交,请认定是否抄袭', read: false, time: '1 小时前', link: 'teacher/cheat-review' },
    { type: '成绩', title: '区块链 2301 班成绩审核已被学校管理员退回', read: true, time: '昨天', link: 'teacher/grade-submit' },
    { type: '系统', title: '仿真包「默克尔树构建动画」审核已通过', read: true, time: '2 天前', link: 'teacher/sim-packages' },
  ];
  function notifications() {
    return `${C.head('站内信', '消息', `<button class="btn btn-outline" onclick="Chaimir.toast('success','已全部标记为已读')">${C.icon('check-check')} 全部已读</button>`)}
      <div class="card">${teacherNotis.map((n) => `
        <a class="flex items-start gap-3" style="padding:14px 18px;border-bottom:1px solid var(--color-border);cursor:pointer" onclick="Chaimir.navigate('${n.link}')">
          <div style="width:34px;height:34px;border-radius:50%;display:grid;place-items:center;flex-shrink:0;background:var(--${n.read ? 'slate' : 'amber'}-100);color:var(--${n.read ? 'slate' : 'amber'}-700)">${C.icon({ '批改': 'check-square', '查重': 'copy-check', '成绩': 'bar-chart-3', '系统': 'info' }[n.type] || 'bell')}</div>
          <div style="flex:1;min-width:0"><div class="flex items-center gap-2">${!n.read ? `<span class="dot dot-amber"></span>` : ''}<span class="fw-600 text-sm">${n.title}</span></div>
            <div class="muted text-xs mt-2">${C.badge(n.type, 'gray')} · ${n.time}</div></div>
          ${C.icon('chevron-right')}
        </a>`).join('')}</div>`;
  }
  function announcements() {
    const list = [
      { title: '平台将于本周六 02:00-04:00 进行例行维护', tag: '系统', time: '1 天前', pin: true },
      { title: '新增 Foundry v0.2.0 运行时,支持更快的合约测试', tag: '功能', time: '3 天前', pin: false },
      { title: '关于规范竞赛防作弊处理流程的通知', tag: '通知', time: '1 周前', pin: false },
    ];
    return `${C.head('系统公告', '消息')}
      ${list.map(a => `<div class="card card-pad mb-3">
        <div class="flex justify-between items-center wrap gap-2"><div class="fw-600 flex items-center gap-2">${a.pin ? C.badge('置顶', 'amber') : ''}${C.badge(a.tag, 'blue')} ${a.title}</div>
          <span class="muted text-xs">${a.time}</span></div>
        <div class="muted text-sm mt-2">点击查看公告详情。重要变更会同时通过站内信提醒。</div></div>`).join('')}`;
  }
  function profile() {
    return `${C.head('个人中心', '账户')}
      <div class="grid" style="grid-template-columns:300px 1fr">
        <div class="card card-pad" style="text-align:center">
          <div style="width:72px;height:72px;border-radius:50%;background:linear-gradient(135deg,var(--amber-500),var(--amber-700));display:grid;place-items:center;color:#fff;font-size:var(--text-2xl);font-weight:700;margin:0 auto 12px">李</div>
          <div class="fw-700" style="font-size:var(--text-lg)">李明远</div>
          <div class="muted text-sm">${C.badge('教师', 'amber')} 计算机学院</div>
          <div class="divider"></div>
          <dl class="dl" style="text-align:left"><dt>工号</dt><dd class="mono">T2019033</dd><dt>手机</dt><dd>138****6677</dd><dt>邮箱</dt><dd>li***@univ.edu</dd></dl>
        </div>
        <div>
          <div class="card card-pad mb-3"><div class="section-title mb-3">账号设置</div>
            <div class="field"><label>姓名</label><input class="input" value="李明远"></div>
            <div class="field"><label>联系邮箱</label><input class="input" value="liming@univ.edu"></div>
            <div class="field" style="margin-bottom:0"><label>手机号</label><div class="flex gap-2"><input class="input" value="138****6677" disabled><button class="btn btn-outline" onclick="Chaimir.demo('换绑手机号')">换绑</button></div></div>
          </div>
          <div class="card card-pad mb-3"><div class="section-title mb-3">安全</div>
            <div class="flex justify-between items-center" style="padding:8px 0"><div><div class="fw-600 text-sm">登录密码</div><div class="muted text-xs">建议定期更换</div></div><button class="btn btn-outline btn-sm" onclick="Chaimir.demo('修改密码')">修改</button></div>
            <div class="flex justify-between items-center" style="padding:8px 0;border-top:1px solid var(--color-border)"><div><div class="fw-600 text-sm">两步验证</div><div class="muted text-xs">短信验证码二次确认</div></div><label class="switch"><input type="checkbox" checked><span class="track"></span></label></div>
          </div>
          <div class="card card-pad"><div class="section-title mb-3">通知偏好</div>
            ${[['作业提交提醒', true], ['查重预警提醒', true], ['成绩审核结果', true], ['系统公告', false]].map(([l, on]) => `
              <div class="flex justify-between items-center" style="padding:7px 0"><span class="text-sm">${l}</span><label class="switch"><input type="checkbox" ${on ? 'checked' : ''}><span class="track"></span></label></div>`).join('')}
          </div>
          <div class="flex justify-end mt-3"><button class="btn btn-primary" onclick="Chaimir.toast('success','已保存','个人设置已更新')">${C.icon('save')} 保存设置</button></div>
        </div>
      </div>`;
  }

  C.registerPages({
    'teacher/content': contentList,
    'teacher/content-edit': contentEdit,
    'teacher/papers': papersList,
    'teacher/paper-edit': paperEdit,
    'teacher/sim-packages': simPackages,
    'teacher/shared-lib': sharedLib,
    'teacher/grade-submit': gradeSubmit,
    'teacher/org': org,
    'teacher/notifications': notifications,
    'teacher/announcements': announcements,
    'teacher/profile': profile,
  });
})();
