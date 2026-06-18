/* ============================================================
   pages/platform-admin/engine.js — 平台管理员·引擎与运维域
   ------------------------------------------------------------
   覆盖:运行时(链)管理 + 运行时详情(三层适配器 / 自检可视化 / 镜像 / 预拉取)、
        工具管理、判题器管理(J1~J6)、仿真包审核(静态扫描+确定性双绿才可上架)、
        配额管理、系统配置(乐观锁版本冲突 + 变更历史/回退)、告警(规则/事件)、
        审计中心、基础监控(外接 Grafana iframe 占位)、备份记录、个人中心。
   说明:遵循 courses.js 范式;子页登记 C.parentRoute;接入/自检/预拉取均以
        进度可视化呈现;危险/不可逆操作经 C.confirm + C.toast;无外部库,无裸 hex。
   ============================================================ */
(function () {
  const C = window.Chaimir;
  const m = C.mock;

  /* 子页 → 高亮侧栏 */
  Object.assign(C.parentRoute, {
    'platform-admin/runtime-detail': 'platform-admin/runtimes',
  });

  /* 通用:自检状态 → 徽章 */
  const selftestBadge = (s) => ({ '通过': 'green', '预拉取中': 'amber', '失败': 'red', '未自检': 'gray' }[s] || 'gray');

  /* 进度条 + 数值(逐节点/逐步骤复用) */
  function progressLine(label, cur, total, color) {
    const pct = total ? (cur / total * 100) : 0;
    return `<div class="mb-3"><div class="flex justify-between text-sm mb-2"><span>${C.esc(label)}</span><span class="mono muted">${cur}/${total}</span></div>
      <div class="progress ${color === 'green' ? 'green' : ''}" style="height:9px"><span style="width:${pct.toFixed(0)}%"></span></div></div>`;
  }

  /* ============================================================
     1) 运行时(链)管理
     ============================================================ */
  function runtimes() {
    const rows = m.runtimes.map(r => {
      const [cur, tot] = r.nodes.split('/').map(Number);
      return `<tr>
        <td><div class="mono fw-600">${C.esc(r.code)}</div></td>
        <td><a style="cursor:pointer;color:var(--color-primary-text);font-weight:600" onclick="Chaimir.navigate('platform-admin/runtime-detail?id=${r.id}')">${C.esc(r.name)}</a></td>
        <td><span class="badge badge-${selftestBadge(r.selftest)}">${r.selftest}</span></td>
        <td class="mono text-sm">${C.esc(r.img)}</td>
        <td style="min-width:140px"><div class="flex items-center gap-2"><div class="progress ${cur === tot ? 'green' : ''}" style="flex:1;height:8px"><span style="width:${(cur / tot * 100).toFixed(0)}%"></span></div><span class="mono text-xs muted">${r.nodes}</span></div></td>
        <td>${r.def ? C.badge('默认', 'amber', 'star') : `<button class="btn btn-ghost btn-sm" onclick="Chaimir.toast('success','已设为默认','新建实验将默认使用该运行时')">设为默认</button>`}</td>
        <td class="row-actions">
          <button class="btn btn-outline btn-sm" onclick="Chaimir.paSelftest('${C.esc(r.name)}')">${C.icon('stethoscope')} 自检</button>
          <button class="btn btn-outline btn-sm" onclick="Chaimir.navigate('platform-admin/runtime-detail?id=${r.id}')">详情</button>
        </td>
      </tr>`;
    }).join('');
    return `${C.head('运行时管理', '引擎', `<button class="btn btn-outline" onclick="Chaimir.paRegisterImage()">${C.icon('package')} 镜像登记</button>
        <button class="btn btn-primary" onclick="Chaimir.paAddChain()">${C.icon('plus')} 接入新链</button>`)}
      <div class="callout info mb-4">${C.icon('info')}<div>运行时通过<b>三层适配器</b>(链生命周期 / 合约部署 / 状态查询)接入;接入后须自检四步全通过,并完成各节点镜像预拉取方可投入教学。</div></div>
      <div class="grid grid-3 mb-4">
        ${C.stat('boxes', m.runtimes.length, '已接入运行时', 'amber')}
        ${C.stat('check-circle-2', m.runtimes.filter(r => r.selftest === '通过').length, '自检通过', 'green')}
        ${C.stat('download-cloud', '1', '预拉取进行中', 'blue')}
      </div>
      <div class="table-wrap"><table class="table"><thead><tr>
        <th>Code</th><th>名称</th><th>自检状态</th><th>镜像版本</th><th>预拉取节点</th><th>默认</th><th></th>
      </tr></thead><tbody>${rows}</tbody></table></div>`;
  }

  /* 接入新链(向导式弹窗;三层适配器登记) */
  C.paAddChain = function () {
    C.modal({
      title: '接入新链', size: 'lg',
      body: `<div class="callout info mb-4">${C.icon('plug')}<div>填写三层适配器后,系统将拉起一次性沙箱运行自检(起链 → 部署 → 查询 → 重置),全部通过才登记成功。</div></div>
        <div class="grid grid-2">
          <div class="field"><label>运行时 Code <span class="req">*</span></label><div class="input-icon">${C.icon('hash')}<input class="input mono" placeholder="如 solana-anchor"></div></div>
          <div class="field"><label>显示名称 <span class="req">*</span></label><input class="input" placeholder="如 Solana · Anchor"></div>
        </div>
        <div class="field"><label>① 链生命周期适配器(镜像)<span class="req">*</span></label><input class="input mono" placeholder="registry/solana-node:v1.18"></div>
        <div class="field"><label>② 合约部署适配器(镜像)<span class="req">*</span></label><input class="input mono" placeholder="registry/anchor-deploy:v0.30"></div>
        <div class="field"><label>③ 状态查询适配器(镜像)<span class="req">*</span></label><input class="input mono" placeholder="registry/solana-query:v1.18"></div>
        <div class="field" style="margin-bottom:0"><label>预拉取节点</label><select class="select"><option>全部计算节点(6)</option><option>仅 GPU 节点(2)</option></select></div>`,
      foot: `<button class="btn btn-outline" onclick="document.querySelector('.overlay').remove()">取消</button>
             <button class="btn btn-primary" onclick="document.querySelector('.overlay').remove();Chaimir.toast('success','已提交接入','正在拉起沙箱自检与镜像预拉取,可在详情页查看进度')">提交并自检</button>`,
    });
  };

  /* 镜像登记 / 预拉取 */
  C.paRegisterImage = function () {
    C.modal({
      title: '镜像登记与预拉取', size: '',
      body: `<div class="field"><label>所属运行时</label><select class="select">${m.runtimes.map(r => `<option>${C.esc(r.name)}</option>`).join('')}</select></div>
        <div class="field"><label>镜像版本标签 <span class="req">*</span></label><input class="input mono" placeholder="如 v2.23.0"></div>
        <div class="field" style="margin-bottom:0"><label class="checkbox"><input type="checkbox" checked> 登记后立即向各节点预拉取</label></div>`,
      foot: `<button class="btn btn-outline" onclick="document.querySelector('.overlay').remove()">取消</button>
             <button class="btn btn-primary" onclick="document.querySelector('.overlay').remove();Chaimir.toast('success','镜像已登记','预拉取任务已下发到 6 个节点')">登记</button>`,
    });
  };

  /* 自检(在 toast 中演示;详情页有完整可视化) */
  C.paSelftest = function (name) {
    C.toast('info', `正在自检「${name}」`, '起链 → 部署 → 查询 → 重置,进入详情查看实时进度');
    setTimeout(() => C.toast('success', '自检通过', `「${name}」四步自检全部通过`), 1400);
  };

  /* ============================================================
     2) 运行时详情(子页)
     ============================================================ */
  function runtimeDetail(ctx) {
    const r = m.runtimes.find(x => x.id == ctx.query.id) || m.runtimes[0];
    const [cur, tot] = r.nodes.split('/').map(Number);
    /* 自检四步:依据自检状态推断进度(原型) */
    const steps = [['起链', 'play'], ['部署合约', 'upload-cloud'], ['状态查询', 'search'], ['环境重置', 'rotate-ccw']];
    const passedSteps = r.selftest === '通过' ? 4 : r.selftest === '预拉取中' ? 4 : r.selftest === '失败' ? 2 : 0;
    /* 逐节点预拉取进度(原型构造 N 节点) */
    const nodes = Array.from({ length: tot }, (_, i) => ({ name: `node-${i + 1}`, done: i < cur }));

    return `${C.crumb([{ label: '运行时管理', to: 'platform-admin/runtimes' }, { label: r.name }])}
      <div class="content-head">
        <div><div class="page-sub mono">${C.esc(r.code)} · 镜像 ${C.esc(r.img)}</div><h1 class="page-title">${C.esc(r.name)}</h1></div>
        <div class="content-actions">
          ${r.def ? C.badge('默认运行时', 'amber', 'star') : ''}
          <button class="btn btn-primary" onclick="Chaimir.paSelftest('${C.esc(r.name)}')">${C.icon('stethoscope')} 重新自检</button>
        </div>
      </div>
      <div class="grid grid-2 mb-4">
        <div class="card"><div class="card-head"><div class="section-title">三层适配器</div><span class="badge badge-${selftestBadge(r.selftest)}">${r.selftest}</span></div><div class="card-pad">
          ${[['链生命周期', 'box', r.code + '-node', '起停/重置链实例'], ['合约部署', 'upload-cloud', r.code + '-deploy', '编译与部署合约'], ['状态查询', 'search', r.code + '-query', '读取链上状态/事件']].map(([t, ic, img, desc], i) => `
            <div class="flex items-center gap-3" style="padding:11px 0;${i < 2 ? 'border-bottom:1px solid var(--color-border)' : ''}">
              <div class="stat-icon" style="width:36px;height:36px;background:var(--amber-100);color:var(--amber-700)">${C.icon(ic)}</div>
              <div style="flex:1"><div class="fw-600 text-sm">第 ${i + 1} 层 · ${t}</div><div class="muted text-xs mono">${img}:${C.esc(r.img)}</div></div>
              <span class="muted text-xs">${desc}</span>
            </div>`).join('')}
        </div></div>
        <div class="card"><div class="card-head"><div class="section-title">自检可视化</div><span class="muted text-xs">${passedSteps}/4 步</span></div><div class="card-pad">
          <div class="steps" style="margin-bottom:18px">${steps.map(([l], i) => `
            <div class="step ${i < passedSteps ? 'done' : (i === passedSteps && r.selftest !== '通过' ? 'active' : '')}">
              <div class="dot-n">${i < passedSteps ? C.icon('check') : (i + 1)}</div>
              <div class="step-label">${l}</div>${i < steps.length - 1 ? '<div class="line"></div>' : ''}
            </div>`).join('')}</div>
          ${r.selftest === '失败'
            ? `<div class="callout danger">${C.icon('x-circle')}<div>第 3 步「状态查询」失败:查询适配器返回超时。请检查节点镜像与网络策略后重试。<span class="muted">报障编号 TRC-9F2A11</span></div></div>`
            : r.selftest === '通过'
              ? `<div class="callout success">${C.icon('check-circle-2')}<div>四步自检全部通过,运行时可投入教学使用。</div></div>`
              : `<div class="callout warn">${C.icon('loader')}<div>自检已通过,正在等待各节点镜像预拉取完成。</div></div>`}
        </div></div>
      </div>
      <div class="grid grid-2">
        <div class="card"><div class="card-head"><div class="section-title">镜像版本</div><button class="btn btn-ghost btn-sm" onclick="Chaimir.paRegisterImage()">${C.icon('plus')} 登记新版本</button></div><div class="card-pad">
          <div class="table-wrap" style="border:none"><table class="table"><thead><tr><th>版本</th><th>登记时间</th><th>状态</th></tr></thead><tbody>
            <tr><td class="mono fw-600">${C.esc(r.img)} ${C.badge('当前', 'green')}</td><td class="mono text-sm">2026-05-28</td><td>${C.statusDot('green', '已就绪')}</td></tr>
            <tr><td class="mono">v2.21.0</td><td class="mono text-sm">2026-04-10</td><td>${C.statusDot('gray', '历史版本')}</td></tr>
          </tbody></table></div>
        </div></div>
        <div class="card"><div class="card-head"><div class="section-title">逐节点预拉取进度</div><span class="badge badge-${cur === tot ? 'green' : 'amber'}">${r.nodes}</span></div><div class="card-pad">
          ${progressLine('整体进度', cur, tot, cur === tot ? 'green' : 'amber')}
          <div class="grid grid-2" style="gap:8px">${nodes.map(n => `
            <div class="flex items-center justify-between" style="padding:8px 10px;border:1px solid var(--color-border);border-radius:var(--radius-sm)">
              <span class="flex items-center gap-2 text-sm mono">${C.icon('server')} ${n.name}</span>
              ${n.done ? C.badge('已拉取', 'green', 'check') : `<span class="badge badge-amber">${C.icon('loader')} 拉取中</span>`}
            </div>`).join('')}</div>
        </div></div>
      </div>`;
  }

  /* ============================================================
     3) 工具管理
     ============================================================ */
  function tools() {
    const data = [
      { id: 1, name: 'Remix 在线 IDE', type: 'iframe', port: 8080, res: '0.5C / 512Mi', chains: 'EVM 系', status: '正常' },
      { id: 2, name: 'Web 终端(ttyd)', type: 'ws终端', port: 7681, res: '0.25C / 256Mi', chains: '全部', status: '正常' },
      { id: 3, name: 'JSON-RPC 代理', type: 'http代理', port: 8545, res: '0.25C / 128Mi', chains: 'EVM 系', status: '正常' },
      { id: 4, name: 'Fabric Explorer', type: 'iframe', port: 8090, res: '0.5C / 512Mi', chains: 'Fabric', status: '停用' },
    ];
    const typeBadge = (t) => ({ 'iframe': 'blue', 'ws终端': 'purple', 'http代理': 'teal' }[t] || 'gray');
    const rows = data.map(d => `<tr>
      <td class="fw-600">${C.esc(d.name)}</td>
      <td>${C.badge(d.type, typeBadge(d.type))}</td>
      <td class="mono text-sm">:${d.port}</td>
      <td class="mono text-sm">${C.esc(d.res)}</td>
      <td>${C.esc(d.chains)}</td>
      <td>${C.statusDot(d.status === '正常' ? 'green' : 'gray', d.status)}</td>
      <td class="row-actions"><button class="btn btn-outline btn-sm" onclick="Chaimir.demo()">配置</button></td>
    </tr>`).join('');
    return `${C.head('工具管理', '引擎', `<button class="btn btn-primary" onclick="Chaimir.paRegisterTool()">${C.icon('plus')} 注册工具</button>`)}
      <div class="callout info mb-4">${C.icon('wrench')}<div>工具以受控代理形态接入实验环境(iframe 嵌入 / ws 终端 / http 代理),受沙箱网络策略约束,仅暴露白名单端口。</div></div>
      <div class="table-wrap"><table class="table"><thead><tr>
        <th>工具名称</th><th>接入类型</th><th>端口</th><th>资源</th><th>链兼容</th><th>状态</th><th></th>
      </tr></thead><tbody>${rows}</tbody></table></div>`;
  }
  C.paRegisterTool = function () {
    C.modal({
      title: '注册工具', size: 'lg',
      body: `<div class="grid grid-2">
          <div class="field"><label>工具名称 <span class="req">*</span></label><input class="input" placeholder="如 Remix 在线 IDE"></div>
          <div class="field"><label>接入类型 <span class="req">*</span></label><select class="select"><option>iframe 嵌入</option><option>ws 终端</option><option>http 代理</option></select></div>
          <div class="field"><label>容器镜像 <span class="req">*</span></label><input class="input mono" placeholder="registry/remix:v0.40"></div>
          <div class="field"><label>暴露端口 <span class="req">*</span></label><input class="input mono" placeholder="8080"></div>
          <div class="field"><label>CPU 限额</label><input class="input mono" placeholder="0.5"></div>
          <div class="field"><label>内存限额</label><input class="input mono" placeholder="512Mi"></div>
        </div>
        <div class="field" style="margin-bottom:0"><label>链兼容</label><input class="input" placeholder="EVM 系 / Fabric / 全部"></div>`,
      foot: `<button class="btn btn-outline" onclick="document.querySelector('.overlay').remove()">取消</button>
             <button class="btn btn-primary" onclick="document.querySelector('.overlay').remove();Chaimir.toast('success','工具已注册','可在实验编排中选用')">注册</button>`,
    });
  };

  /* ============================================================
     4) 判题器管理(J1~J6)
     ============================================================ */
  function judgers() {
    const data = [
      { code: 'J1', name: '单元测试判题', desc: '运行测试用例统计通过率', img: 'judge-unit:v1.4', timeout: 60, retry: 1, status: '正常' },
      { code: 'J2', name: '检查点判题', desc: '按链上状态断言逐点核对', img: 'judge-checkpoint:v1.2', timeout: 90, retry: 2, status: '正常' },
      { code: 'J3', name: 'Gas 优化判题', desc: '对比 gas 消耗与基线阈值', img: 'judge-gas:v1.1', timeout: 60, retry: 1, status: '正常' },
      { code: 'J4', name: 'CTF flag 判题', desc: '校验提交 flag 是否命中', img: 'judge-ctf:v1.0', timeout: 30, retry: 0, status: '正常' },
      { code: 'J5', name: '对抗赛结算判题', desc: '攻防对局自动裁决与计分', img: 'judge-battle:v1.3', timeout: 120, retry: 2, status: '正常' },
      { code: 'J6', name: '人工/混合判题', desc: '机判初筛 + 教师复核', img: 'judge-hybrid:v1.0', timeout: 60, retry: 1, status: '维护中' },
    ];
    const cards = data.map(j => `
      <div class="card"><div class="card-pad">
        <div class="flex items-center justify-between mb-2">
          <div class="flex items-center gap-2"><span class="badge badge-amber mono">${j.code}</span><span class="fw-700">${C.esc(j.name)}</span></div>
          ${C.statusDot(j.status === '正常' ? 'green' : 'amber', j.status)}
        </div>
        <p class="muted text-sm mb-3">${C.esc(j.desc)}</p>
        <dl class="dl" style="grid-template-columns:80px 1fr;gap:6px 12px">
          <dt>镜像</dt><dd class="mono text-xs">${C.esc(j.img)}</dd>
          <dt>超时</dt><dd class="mono">${j.timeout}s</dd>
          <dt>重试</dt><dd class="mono">${j.retry} 次</dd>
        </dl>
        <div class="flex gap-2 mt-3">
          <button class="btn btn-outline btn-sm" onclick="Chaimir.paConfigJudger('${j.code}','${C.esc(j.name)}',${j.timeout},${j.retry})">${C.icon('settings-2')} 配置</button>
          <button class="btn btn-outline btn-sm" onclick="Chaimir.paJudgerSelftest('${C.esc(j.name)}')">${C.icon('flask-conical')} 样例自检</button>
        </div>
      </div></div>`).join('');
    return `${C.head('判题器管理', '引擎', `<button class="btn btn-primary" onclick="Chaimir.paConfigJudger('','新判题器',60,1)">${C.icon('plus')} 注册判题器</button>`)}
      <div class="callout info mb-4">${C.icon('scale')}<div>判题器为可插拔引擎(J1~J6);每类绑定运行时镜像版本、超时与重试策略,上线前须以官方样例通过自检。</div></div>
      <div class="grid grid-3">${cards}</div>`;
  }
  C.paConfigJudger = function (code, name, timeout, retry) {
    C.modal({
      title: (code ? code + ' · ' : '') + name + ' 配置', size: '',
      body: `<div class="field"><label>绑定运行时镜像版本 <span class="req">*</span></label><select class="select">${m.runtimes.map(r => `<option>${C.esc(r.name)} · ${C.esc(r.img)}</option>`).join('')}</select></div>
        <div class="grid grid-2">
          <div class="field"><label>超时(秒)</label><input class="input mono" value="${timeout}"></div>
          <div class="field"><label>重试次数</label><input class="input mono" value="${retry}"></div>
        </div>
        <div class="field" style="margin-bottom:0"><label class="checkbox"><input type="checkbox" checked> 判题在独立沙箱执行,用后即毁</label></div>`,
      foot: `<button class="btn btn-outline" onclick="document.querySelector('.overlay').remove()">取消</button>
             <button class="btn btn-primary" onclick="document.querySelector('.overlay').remove();Chaimir.toast('success','配置已保存','下次判题生效')">保存</button>`,
    });
  };
  C.paJudgerSelftest = function (name) {
    C.toast('info', `正在以官方样例自检「${name}」`, '运行样例输入并比对预期判定…');
    setTimeout(() => C.toast('success', '样例自检通过', `「${name}」判定结果与预期一致`), 1300);
  };

  /* ============================================================
     5) 仿真包审核(approve 前须双绿)
     ============================================================ */
  function simReview() {
    const data = [
      { id: 1, name: 'PBFT 拜占庭容错可视化', author: '王思齐', ver: 'v1.2.0', scan: 'passed', det: 'passed', time: '2026-06-05' },
      { id: 2, name: 'PoW 难度调整动态仿真', author: '李明远', ver: 'v1.0.0', scan: 'passed', det: 'running', time: '2026-06-04' },
      { id: 3, name: '跨链桥消息中继沙盘', author: '陈雪', ver: 'v0.9.1', scan: 'failed', det: 'pending', time: '2026-06-02' },
    ];
    const scanText = { passed: ['通过', 'green'], running: ['检查中', 'amber'], failed: ['未通过', 'red'], pending: ['待执行', 'gray'] };
    const rows = data.map(d => {
      const [st, sc] = scanText[d.scan]; const [dt, dc] = scanText[d.det];
      const ready = d.scan === 'passed' && d.det === 'passed';
      return `<tr>
        <td><div class="fw-600">${C.esc(d.name)}</div><div class="muted text-xs">${C.icon('user')} ${d.author} · ${d.ver}</div></td>
        <td><span class="flex items-center gap-2"><span class="dot dot-${sc}"></span>静态扫描:${st}</span></td>
        <td><span class="flex items-center gap-2"><span class="dot dot-${dc}"></span>确定性:${dt}</span></td>
        <td class="mono text-sm">${C.esc(d.time)}</td>
        <td class="row-actions">
          <button class="btn btn-outline btn-sm" onclick="Chaimir.paSimPreview('${C.esc(d.name)}')">${C.icon('eye')} 预览</button>
          <button class="btn btn-outline btn-sm" onclick="Chaimir.paSimReport('${C.esc(d.name)}', '${d.scan}', '${d.det}')">${C.icon('file-search')} 扫描报告</button>
          <button class="btn btn-primary btn-sm" ${ready ? '' : 'aria-disabled="true" title="须静态扫描与确定性检查均通过"'} onclick="${ready ? `Chaimir.paSimApprove('${C.esc(d.name)}')` : `Chaimir.toast('error','尚不可上架','须静态扫描与确定性检查均为通过')`}">上架</button>
          <button class="btn btn-danger btn-sm" onclick="Chaimir.paSimReturn('${C.esc(d.name)}')">退回</button>
        </td>
      </tr>`;
    }).join('');
    return `${C.head('仿真包审核', '引擎', `<span class="badge badge-amber" style="align-self:center">${data.filter(d => !(d.scan === 'passed' && d.det === 'passed')).length} 项待处理</span>`)}
      <div class="callout warn mb-4">${C.icon('shield-alert')}<div>仿真包来自教师上传,属不可信内容。<b>上架前必须 static_scan=passed 且 determinism_check=passed</b>;任一未过则上架按钮锁定。预览在隔离沙箱中进行。</div></div>
      <div class="table-wrap"><table class="table"><thead><tr>
        <th>仿真包</th><th>静态扫描</th><th>确定性检查</th><th>提交时间</th><th></th>
      </tr></thead><tbody>${rows}</tbody></table></div>`;
  }
  C.paSimPreview = function (name) {
    C.drawer({
      title: '沙箱预览 · ' + name,
      body: `<div class="callout info mb-3">${C.icon('shield')}<div>预览运行于隔离沙箱(deny-all 网络),仅用于审核,不接触生产数据。</div></div>
        <div style="aspect-ratio:4/3;background:var(--color-dark-bg);border-radius:var(--radius);display:grid;place-items:center;color:var(--color-dark-text-sub);margin-bottom:14px">
          <div style="text-align:center">${C.icon('activity')}<div class="mt-2 text-sm">仿真渲染预览(原型占位)</div></div></div>
        <dl class="dl"><dt>渲染类型</dt><dd>图网络 + 时序泳道</dd><dt>初始节点</dt><dd>7</dd><dt>可注入故障</dt><dd>拜占庭节点 / 网络分区</dd><dt>确定性种子</dt><dd class="mono">0x7f3a…</dd></dl>`,
      foot: `<button class="btn btn-outline" onclick="document.querySelector('.overlay').remove()">关闭</button>`,
    });
  };
  C.paSimReport = function (name, scan, det) {
    const line = (label, st) => {
      const map = { passed: ['通过', 'green', 'check-circle-2'], running: ['检查中', 'amber', 'loader'], failed: ['未通过', 'red', 'x-circle'], pending: ['待执行', 'gray', 'circle'] };
      const [t, c, ic] = map[st];
      return `<div class="flex items-center justify-between" style="padding:10px 0;border-bottom:1px solid var(--color-border)"><span class="flex items-center gap-2">${C.icon(ic)} ${label}</span><span class="badge badge-${c}">${t}</span></div>`;
    };
    C.modal({
      title: '扫描报告 · ' + name, size: 'lg',
      body: `<div class="section-title mb-2">静态扫描</div>
        ${line('依赖与镜像漏洞扫描', scan)}
        ${line('危险系统调用 / 越权访问检测', scan)}
        ${line('资源声明合法性', scan)}
        <div class="section-title mb-2 mt-4">确定性检查</div>
        ${line('同种子多次运行结果一致', det)}
        ${line('无外部网络依赖', det)}
        ${scan === 'failed' ? `<div class="callout danger mt-4">${C.icon('x-circle')}<div>检出 1 个高危项:镜像基础层存在已知 CVE。请教师修复后重新提交。</div></div>` : ''}`,
      foot: `<button class="btn btn-outline" onclick="document.querySelector('.overlay').remove()">关闭</button>`,
    });
  };
  C.paSimApprove = async function (name) {
    if (await C.confirm({ title: '上架仿真包', confirmText: '确认上架', message: `「${name}」静态扫描与确定性检查均已通过,确认上架到平台仿真库?` }))
      C.toast('success', '已上架', `「${name}」已对全平台教师可见`);
  };
  C.paSimReturn = function (name) {
    C.modal({
      title: '退回仿真包', size: '',
      body: `<p class="text-sm mb-3">退回「${C.esc(name)}」,意见将通知提交教师。</p>
        <div class="field" style="margin-bottom:0"><label>退回意见 <span class="req">*</span></label><textarea class="textarea" id="pa-sim-reason" placeholder="如:确定性检查未通过,存在依赖系统时间的随机行为,请改用固定种子"></textarea></div>`,
      foot: `<button class="btn btn-outline" onclick="document.querySelector('.overlay').remove()">取消</button>
             <button class="btn btn-danger" onclick="(function(){var v=(document.getElementById('pa-sim-reason')||{}).value||'';if(!v.trim()){Chaimir.toast('error','请填写退回意见','意见将通知提交教师,不能为空');return;}document.querySelector('.overlay').remove();Chaimir.toast('success','已退回','退回意见已通知提交教师');})()">确认退回</button>`,
    });
  };

  /* ============================================================
     6) 配额管理
     ============================================================ */
  function quota() {
    const data = m.tenants.map((t, i) => ({
      name: t.name, code: t.code,
      maxBox: [40, 24, 12][i] || 16, useBox: [12, 6, 0][i] || 0,
      cpu: ['20 核', '12 核', '8 核'][i] || '8 核', mem: ['40 Gi', '24 Gi', '16 Gi'][i] || '16 Gi',
      idle: ['15 分', '10 分', '10 分'][i] || '10 分', live: ['2 时', '90 分', '60 分'][i] || '60 分', snap: ['7 天', '7 天', '3 天'][i] || '3 天',
    }));
    const rows = data.map(d => `<tr>
      <td><div class="fw-600">${C.esc(d.name)}</div><div class="muted text-xs mono">${C.esc(d.code)}</div></td>
      <td style="min-width:150px"><div class="flex items-center gap-2"><div class="progress ${d.useBox / d.maxBox > .8 ? '' : 'green'}" style="flex:1;height:8px"><span style="width:${(d.useBox / d.maxBox * 100).toFixed(0)}%"></span></div><span class="mono text-xs">${d.useBox}/${d.maxBox}</span></div></td>
      <td class="mono text-sm">${C.esc(d.cpu)}</td>
      <td class="mono text-sm">${C.esc(d.mem)}</td>
      <td class="mono text-sm">${C.esc(d.idle)}</td>
      <td class="mono text-sm">${C.esc(d.live)}</td>
      <td class="mono text-sm">${C.esc(d.snap)}</td>
      <td class="row-actions"><button class="btn btn-outline btn-sm" onclick="Chaimir.paEditQuota('${C.esc(d.name)}',${d.maxBox})">${C.icon('sliders-horizontal')} 调整</button></td>
    </tr>`).join('');
    return `${C.head('配额管理', '引擎', '')}
      <div class="callout info mb-4">${C.icon('gauge')}<div>配额按租户隔离硬限:超出最大并发沙箱将排队;空闲/最大存活到期自动回收;快照按保留期清理。调整即时生效。</div></div>
      <div class="grid grid-3 mb-4">
        ${C.stat('boxes', '18 / 76', '全平台并发沙箱', 'amber')}
        ${C.stat('cpu', '40 / 40 核', 'CPU 已分配', 'purple')}
        ${C.stat('memory-stick', '80 Gi', '内存已分配', 'blue')}
      </div>
      <div class="table-wrap"><table class="table"><thead><tr>
        <th>租户</th><th>并发沙箱(用量/上限)</th><th>CPU</th><th>内存</th><th>空闲超时</th><th>最大存活</th><th>快照保留</th><th></th>
      </tr></thead><tbody>${rows}</tbody></table></div>`;
  }
  C.paEditQuota = function (name, maxBox) {
    C.modal({
      title: '调整配额 · ' + name, size: '',
      body: `<div class="grid grid-2">
          <div class="field"><label>最大并发沙箱</label><input class="input mono" value="${maxBox}"></div>
          <div class="field"><label>CPU 上限(核)</label><input class="input mono" value="20"></div>
          <div class="field"><label>内存上限(Gi)</label><input class="input mono" value="40"></div>
          <div class="field"><label>空闲超时(分)</label><input class="input mono" value="15"></div>
          <div class="field"><label>最大存活(分)</label><input class="input mono" value="120"></div>
          <div class="field"><label>快照保留(天)</label><input class="input mono" value="7"></div>
        </div>
        <div class="callout warn" style="margin-top:4px">${C.icon('alert-triangle')}<div>下调上限可能导致该校进行中的实验被排队或回收,建议在低峰期调整。</div></div>`,
      foot: `<button class="btn btn-outline" onclick="document.querySelector('.overlay').remove()">取消</button>
             <button class="btn btn-primary" onclick="document.querySelector('.overlay').remove();Chaimir.toast('success','配额已更新','新配额即时生效')">保存</button>`,
    });
  };

  /* ============================================================
     7) 系统配置(乐观锁 + 变更历史/回退)
     ============================================================ */
  function config() {
    const data = [
      { id: 1, key: 'sandbox.default_runtime', val: '"evm-hardhat"', scope: '全局', ver: 7, by: '平台管理员', time: '2026-06-05' },
      { id: 2, key: 'judge.max_concurrency', val: '32', scope: '全局', ver: 3, by: '平台管理员', time: '2026-05-28' },
      { id: 3, key: 'sim.preview_ttl_sec', val: '600', scope: '全局', ver: 2, by: '运维', time: '2026-05-20' },
      { id: 4, key: 'tenant.signup_review', val: 'true', scope: '全局', ver: 5, by: '平台管理员', time: '2026-05-12' },
    ];
    const rows = data.map(d => `<tr>
      <td class="mono fw-600">${C.esc(d.key)}</td>
      <td><code style="background:var(--color-surface-sunken);padding:2px 6px;border-radius:var(--radius-xs);font-family:var(--font-mono);font-size:var(--text-xs)">${C.esc(d.val)}</code></td>
      <td>${C.badge(d.scope, 'gray')}</td>
      <td class="mono text-sm">v${d.ver}</td>
      <td class="muted text-sm">${C.esc(d.by)} · ${C.esc(d.time)}</td>
      <td class="row-actions">
        <button class="btn btn-outline btn-sm" onclick="Chaimir.paEditConfig('${C.esc(d.key)}','${C.esc(d.val).replace(/'/g, "\\'")}',${d.ver})">${C.icon('pencil')} 编辑</button>
        <button class="btn btn-ghost btn-sm" onclick="Chaimir.paConfigHistory('${C.esc(d.key)}')">${C.icon('history')} 历史</button>
      </td>
    </tr>`).join('');
    return `${C.head('系统配置', '运维', `<button class="btn btn-outline" onclick="Chaimir.demo()">${C.icon('plus')} 新增配置项</button>`)}
      <div class="callout info mb-4">${C.icon('settings')}<div>配置以 JSONB 存储并带版本号;保存采用<b>乐观锁</b>,若版本已被他人更新将提示冲突并需重试。每次变更入历史,可一键回退。</div></div>
      <div class="table-wrap"><table class="table"><thead><tr>
        <th>配置键</th><th>值(JSONB)</th><th>作用范围</th><th>版本</th><th>更新人</th><th></th>
      </tr></thead><tbody>${rows}</tbody></table></div>`;
  }
  C.paEditConfig = function (key, val, ver) {
    C.modal({
      title: '编辑配置 · ' + key, size: '',
      body: `<div class="dl mb-3"><dt>当前版本</dt><dd class="mono">v${ver}</dd></div>
        <div class="field"><label>值(JSONB)<span class="req">*</span></label><textarea class="textarea mono" id="pa-cfg-val" style="min-height:70px">${C.esc(val)}</textarea><div class="help">须为合法 JSON;保存时校验取值范围</div></div>
        <input type="hidden" id="pa-cfg-ver" value="${ver}">
        <div class="callout warn" style="margin-bottom:0">${C.icon('lock')}<div>保存基于版本 v${ver} 的乐观锁;若期间被他人修改,将提示版本冲突。</div></div>`,
      foot: `<button class="btn btn-outline" onclick="document.querySelector('.overlay').remove()">取消</button>
             <button class="btn btn-primary" onclick="Chaimir.paSaveConfig('${C.esc(key)}')">保存</button>`,
    });
  };
  /* 模拟乐观锁冲突:演示一次冲突提示 + 重试路径 */
  C.paSaveConfig = function (key) {
    if (!C._cfgConflictShown) {
      C._cfgConflictShown = true;
      C.toast('error', '版本冲突,保存失败', `「${key}」已被他人更新到新版本,请关闭后重新打开以基于最新版本编辑`, 'TRC-CFG-409');
      return;
    }
    document.querySelector('.overlay') && document.querySelector('.overlay').remove();
    C.toast('success', '配置已保存', `「${key}」已更新,版本号 +1,变更已入历史`);
  };
  C.paConfigHistory = function (key) {
    const hist = [
      { ver: 7, val: '"evm-hardhat"', by: '平台管理员', time: '2026-06-05 14:20' },
      { ver: 6, val: '"evm-foundry"', by: '运维', time: '2026-05-10 09:30' },
      { ver: 5, val: '"evm-hardhat"', by: '平台管理员', time: '2026-04-02 11:05' },
    ];
    C.modal({
      title: '变更历史 · ' + key, size: 'lg',
      body: `<div class="table-wrap" style="border:none"><table class="table"><thead><tr><th>版本</th><th>值</th><th>更新人</th><th>时间</th><th></th></tr></thead><tbody>
        ${hist.map((h, i) => `<tr>
          <td class="mono fw-600">v${h.ver} ${i === 0 ? C.badge('当前', 'green') : ''}</td>
          <td><code style="background:var(--color-surface-sunken);padding:2px 6px;border-radius:var(--radius-xs);font-family:var(--font-mono);font-size:var(--text-xs)">${C.esc(h.val)}</code></td>
          <td>${C.esc(h.by)}</td><td class="mono text-sm">${C.esc(h.time)}</td>
          <td class="row-actions">${i === 0 ? '<span class="muted text-xs">—</span>' : `<button class="btn btn-outline btn-sm" onclick="Chaimir.paRollbackConfig('${C.esc(key)}',${h.ver})">回退到此版</button>`}</td>
        </tr>`).join('')}
      </tbody></table></div>`,
      foot: `<button class="btn btn-outline" onclick="document.querySelector('.overlay').remove()">关闭</button>`,
    });
  };
  C.paRollbackConfig = async function (key, ver) {
    document.querySelector('.overlay') && document.querySelector('.overlay').remove();
    if (await C.confirm({ title: '回退配置', danger: true, confirmText: '确认回退', message: `将把「${key}」回退到 v${ver} 的取值,并生成一条新版本记录。确认回退?` }))
      C.toast('success', '已回退', `「${key}」已回退到 v${ver} 的取值`);
  };

  /* ============================================================
     8) 告警(规则 + 事件 Tab)
     ============================================================ */
  function alerts(ctx) {
    const tab = ctx.query.tab || 'events';
    const events = [
      { id: 1, level: '严重', title: '沙箱节点 node-4 不可调度', src: '集群', time: '08 分钟前', status: '未处理' },
      { id: 2, level: '警告', title: 'Fabric 运行时镜像预拉取超时重试', src: '运行时', time: '32 分钟前', status: '未处理' },
      { id: 3, level: '提示', title: '示例大学并发沙箱达上限 80%', src: '配额', time: '1 小时前', status: '已忽略' },
      { id: 4, level: '警告', title: '判题队列积压超过阈值(>120)', src: '判题', time: '2 小时前', status: '已处理' },
    ];
    const rules = [
      { id: 1, name: '沙箱节点不可用', cond: 'node Ready=false 持续 > 1 分钟', level: '严重', on: true },
      { id: 2, name: '判题队列积压', cond: 'pending_jobs > 100 持续 > 5 分钟', level: '警告', on: true },
      { id: 3, name: '租户配额逼近', cond: 'sandbox_usage / quota > 0.8', level: '提示', on: true },
      { id: 4, name: '备份失败', cond: 'backup.status = failed', level: '严重', on: false },
    ];
    const levelBadge = (l) => ({ '严重': 'red', '警告': 'amber', '提示': 'blue' }[l] || 'gray');
    const stBadge = (s) => ({ '未处理': 'red', '已处理': 'green', '已忽略': 'gray' }[s] || 'gray');

    const body = tab === 'events'
      ? `<div class="table-wrap"><table class="table"><thead><tr><th>级别</th><th>告警内容</th><th>来源</th><th>时间</th><th>状态</th><th></th></tr></thead><tbody>
          ${events.map(e => `<tr>
            <td><span class="badge badge-${levelBadge(e.level)}">${e.level}</span></td>
            <td class="fw-600">${C.esc(e.title)}</td>
            <td>${C.badge(e.src, 'gray')}</td>
            <td class="muted text-sm">${C.esc(e.time)}</td>
            <td><span class="badge badge-${stBadge(e.status)}">${e.status}</span></td>
            <td class="row-actions">${e.status === '未处理'
              ? `<button class="btn btn-primary btn-sm" onclick="Chaimir.toast('success','已标记处理','告警已置为已处理')">处理</button><button class="btn btn-outline btn-sm" onclick="Chaimir.toast('success','已忽略','此告警将不再提醒')">忽略</button>`
              : '<span class="muted text-xs">—</span>'}</td>
          </tr>`).join('')}
        </tbody></table></div>`
      : `<div class="table-wrap"><table class="table"><thead><tr><th>规则名称</th><th>触发条件</th><th>级别</th><th>启用</th><th></th></tr></thead><tbody>
          ${rules.map(r => `<tr>
            <td class="fw-600">${C.esc(r.name)}</td>
            <td class="mono text-sm muted">${C.esc(r.cond)}</td>
            <td><span class="badge badge-${levelBadge(r.level)}">${r.level}</span></td>
            <td><label class="switch"><input type="checkbox" ${r.on ? 'checked' : ''} onchange="Chaimir.toast('success','已更新','告警规则状态已保存')"><span class="track"></span></label></td>
            <td class="row-actions"><button class="btn btn-outline btn-sm" onclick="Chaimir.demo()">编辑</button></td>
          </tr>`).join('')}
        </tbody></table></div>`;

    return `${C.head('告警', '运维', tab === 'rules' ? `<button class="btn btn-primary" onclick="Chaimir.demo()">${C.icon('plus')} 新建规则</button>` : `<span class="badge badge-red" style="align-self:center">${events.filter(e => e.status === '未处理').length} 条未处理</span>`)}
      <div class="tabs">
        <a class="tab ${tab === 'events' ? 'active' : ''}" onclick="Chaimir.navigate('platform-admin/alerts?tab=events')">告警事件</a>
        <a class="tab ${tab === 'rules' ? 'active' : ''}" onclick="Chaimir.navigate('platform-admin/alerts?tab=rules')">告警规则</a>
      </div>
      ${body}`;
  }

  /* ============================================================
     9) 审计中心
     ============================================================ */
  function audit() {
    const data = [
      { time: '2026-06-07 15:42', actor: '平台管理员', action: '停用租户', obj: '租户 · 云岭师范学院', ip: '10.0.2.31', result: '成功' },
      { time: '2026-06-07 14:20', actor: '平台管理员', action: '修改系统配置', obj: '配置 · sandbox.default_runtime', ip: '10.0.2.31', result: '成功' },
      { time: '2026-06-07 11:08', actor: '系统任务', action: '完成自动备份', obj: '备份 · 全量', ip: '10.0.2.45', result: '成功' },
      { time: '2026-06-06 16:40', actor: '平台管理员', action: '审核通过入驻', obj: '租户 · 示例大学', ip: '10.0.2.31', result: '成功' },
      { time: '2026-06-06 09:55', actor: '运维', action: '接入运行时', obj: '运行时 · Hyperledger Fabric', ip: '10.0.2.45', result: '失败' },
    ];
    const actionBadge = (a) => a.includes('停用') || a.includes('删除') ? 'red' : a.includes('配置') ? 'amber' : 'blue';
    const rows = data.map(d => `<tr>
      <td class="mono text-sm">${C.esc(d.time)}</td>
      <td>${C.esc(d.actor)}</td>
      <td>${C.badge(d.action, actionBadge(d.action))}</td>
      <td>${C.esc(d.obj)}</td>
      <td class="mono text-sm muted">${C.esc(d.ip)}</td>
      <td>${C.statusDot(d.result === '成功' ? 'green' : 'red', d.result)}</td>
    </tr>`).join('');
    return `${C.head('审计中心', '运维', `<button class="btn btn-outline" onclick="Chaimir.toast('success','已导出','审计日志(CSV)已生成,原型演示')">${C.icon('download')} 导出</button>`)}
      <div class="callout info mb-4">${C.icon('scroll-text')}<div>平台级操作统一写入唯一审计表,不可篡改;敏感字段已脱敏。可按操作人/动作/对象类型/时间组合检索。</div></div>
      <div class="card mb-3"><div class="card-pad flex items-center gap-3 wrap" style="padding:14px 18px">
        ${C.icon('filter')}
        <select class="select" style="max-width:150px"><option>全部操作人</option><option>平台管理员</option><option>运维</option></select>
        <select class="select" style="max-width:150px"><option>全部动作</option><option>停用租户</option><option>修改系统配置</option><option>审核入驻</option></select>
        <select class="select" style="max-width:150px"><option>全部对象类型</option><option>租户</option><option>配置</option><option>运行时</option><option>备份</option></select>
        <input class="input" type="date" value="2026-06-01" style="max-width:150px"><span class="muted">至</span><input class="input" type="date" value="2026-06-07" style="max-width:150px">
        <button class="btn btn-primary btn-sm">${C.icon('search')} 查询</button>
      </div></div>
      <div class="table-wrap"><table class="table"><thead><tr>
        <th>时间</th><th>操作人</th><th>动作</th><th>对象</th><th>来源 IP</th><th>结果</th>
      </tr></thead><tbody>${rows}</tbody></table></div>
      ${C.pagination(1, 248)}`;
  }

  /* ============================================================
     10) 基础监控(外接 Grafana iframe 占位)
     ============================================================ */
  function monitoring(ctx) {
    const panel = ctx.query.p || 'health';
    const panels = [['health', '服务健康'], ['resource', '资源时序'], ['sandbox', '沙箱与判题']];
    return `${C.head('基础监控', '运维', `<button class="btn btn-outline" onclick="Chaimir.toast('info','在 Grafana 打开','原型演示:跳转外接 Grafana 控制台')">${C.icon('external-link')} 在 Grafana 打开</button>`)}
      <div class="callout info mb-4">${C.icon('radar')}<div>监控数据来自外接 <b>Prometheus / Grafana</b>,本页以 iframe 嵌入只读面板;告警阈值在「告警」中配置。</div></div>
      <div class="flex gap-1 mb-4">${panels.map(([k, l]) => `<button class="btn ${k === panel ? 'btn-primary' : 'btn-outline'} btn-sm" onclick="Chaimir.navigate('platform-admin/monitoring?p=${k}')">${l}</button>`).join('')}</div>
      <div class="card" style="overflow:hidden">
        <div class="card-head"><div class="section-title">${panels.find(x => x[0] === panel)[1]} · Grafana 面板</div><span class="badge badge-green">${C.icon('dot')} 数据源在线</span></div>
        <div style="height:420px;background:var(--color-dark-bg);display:grid;place-items:center;color:var(--color-dark-text-sub);position:relative">
          <div style="text-align:center">${C.icon('line-chart')}<div class="mt-2 text-sm">外接 Grafana 面板(iframe 占位)</div>
            <div class="muted text-xs mt-2" style="color:var(--color-dark-text-sub)">/d/${panel}/${panel}-overview · refresh 30s</div></div>
          <span class="badge badge-gray" style="position:absolute;top:12px;left:12px">iframe</span>
        </div>
      </div>`;
  }

  /* ============================================================
     11) 备份记录
     ============================================================ */
  function backups() {
    const data = [
      { id: 1, type: '全量', size: '12.4 GB', status: '成功', start: '2026-06-07 02:00', end: '2026-06-07 02:38' },
      { id: 2, type: '全量', size: '12.1 GB', status: '成功', start: '2026-06-06 02:00', end: '2026-06-06 02:36' },
      { id: 3, type: '全量', size: '12.0 GB', status: '成功', start: '2026-06-05 02:00', end: '2026-06-05 02:34' },
      { id: 4, type: '全量', size: '11.9 GB', status: '失败', start: '2026-05-31 02:00', end: '2026-05-31 02:12' },
    ];
    const rows = data.map(d => `<tr>
      <td>${C.badge(d.type, 'purple')}</td>
      <td class="mono">${C.esc(d.size)}</td>
      <td>${C.statusDot(d.status === '成功' ? 'green' : 'red', d.status)}</td>
      <td class="mono text-sm">${C.esc(d.start)}</td>
      <td class="mono text-sm">${C.esc(d.end)}</td>
    </tr>`).join('');
    return `${C.head('备份记录', '运维')}
      <div class="grid grid-3 mb-4">
        ${C.stat('check-circle-2', '最近成功', '2026-06-07 02:38', 'green')}
        ${C.stat('hard-drive', '12.4 GB', '最近全量大小', 'purple')}
        ${C.stat('calendar-check', '每日 02:23', '自动备份计划', 'blue')}
      </div>
      <div class="table-wrap"><table class="table"><thead><tr>
        <th>类型</th><th>大小</th><th>状态</th><th>开始时间</th><th>结束时间</th>
      </tr></thead><tbody>${rows}</tbody></table></div>`;
  }

  /* ============================================================
     12) 个人中心
     ============================================================ */
  function profile() {
    return `${C.head('个人中心', '账户')}
      <div class="grid grid-2">
        <div class="card"><div class="card-head"><div class="section-title">基本信息</div></div><div class="card-pad">
          <div class="flex items-center gap-3 mb-4">
            <span class="ava" style="width:56px;height:56px;border-radius:50%;background:linear-gradient(135deg,var(--amber-500),var(--amber-700));display:grid;place-items:center;color:#fff;font-weight:700;font-size:var(--text-xl)">平</span>
            <div><div class="fw-700" style="font-size:var(--text-lg)">平台管理员</div><div class="muted text-sm">${C.icon('shield')} 超级管理员 · 全平台</div></div>
          </div>
          <dl class="dl">
            <dt>登录账号</dt><dd class="mono">admin@chaimir.platform</dd>
            <dt>角色</dt><dd>${C.badge('平台管理员', 'amber', 'shield')}</dd>
            <dt>手机号</dt><dd class="mono">186****0000</dd>
            <dt>上次登录</dt><dd class="mono">2026-06-07 09:12 · 10.0.2.31</dd>
            <dt>双因子</dt><dd>${C.statusDot('green', '已开启(TOTP)')}</dd>
          </dl>
          <button class="btn btn-outline mt-4" onclick="Chaimir.demo()">${C.icon('pencil')} 编辑资料</button>
        </div></div>
        <div class="card"><div class="card-head"><div class="section-title">修改密码</div></div><div class="card-pad">
          <div class="field"><label>当前密码 <span class="req">*</span></label><input class="input" type="password" placeholder="输入当前密码"></div>
          <div class="field"><label>新密码 <span class="req">*</span></label><input class="input" type="password" placeholder="至少 12 位,含大小写/数字/符号"></div>
          <div class="field"><label>确认新密码 <span class="req">*</span></label><input class="input" type="password" placeholder="再次输入新密码"></div>
          <div class="callout warn mb-3">${C.icon('shield-alert')}<div>平台管理员账号权限极高,修改密码后所有已登录会话将被强制下线。</div></div>
          <button class="btn btn-primary" onclick="Chaimir.paChangePwd()">保存新密码</button>
        </div></div>
      </div>`;
  }
  C.paChangePwd = async function () {
    if (await C.confirm({ title: '修改密码', danger: true, confirmText: '确认修改', message: '保存后当前账号的所有登录会话将被强制下线,需重新登录。确认修改密码?' }))
      C.toast('success', '密码已更新', '请使用新密码重新登录');
  };

  /* ---------- 注册 ---------- */
  C.registerPages({
    'platform-admin/runtimes': runtimes,
    'platform-admin/runtime-detail': runtimeDetail,
    'platform-admin/tools': tools,
    'platform-admin/judgers': judgers,
    'platform-admin/sim-review': simReview,
    'platform-admin/quota': quota,
    'platform-admin/config': config,
    'platform-admin/alerts': alerts,
    'platform-admin/audit': audit,
    'platform-admin/monitoring': monitoring,
    'platform-admin/backups': backups,
    'platform-admin/profile': profile,
  });
})();
