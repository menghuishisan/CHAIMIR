/* ============================================================
   pages/immersive/sim.js — 仿真可视化工作台(全屏沉浸)
   ------------------------------------------------------------
   对应 M4 仿真可视化引擎(教学引擎,非渲染引擎)。
   设计依据 M4 文档:可视化模式(graph 图网络 / lane 时序泳道 /
   matrix 投票矩阵)+ 教学叙事(分步引导 D1 / 关键释义"为什么" D2 /
   设问检查点 D3)+ 单步推进回退 + 通用交互(注入拜占庭 / 调节点数)。
   教学重点:让学生看清 PBFT 每个阶段做了什么、产生什么效果、
   为什么重要,以及法定人数 2f+1 如何决定共识成败。
   ============================================================ */
(function () {
  const C = window.Chaimir;
  const S = C.simFn = {};
  const SIM = { phase: 0, N: 4, byz: {}, reveal: false };

  /* PBFT 五个阶段:就绪 → 预准备 → 准备 → 提交 → 已提交。每阶段含"效果"与"为什么" */
  const PHASES = [
    { name: '就绪', tag: 'IDLE', effect: '客户端向主节点 N1 提交请求,等待主节点发起提案。', why: '主节点负责为请求定序;一旦作恶,后续阶段由副本多数票纠正。' },
    { name: 'Pre-Prepare 预准备', tag: 'PRE-PREPARE', effect: '主节点 N1 为请求分配序号,并把提案广播给所有副本。', why: '确立全局唯一的请求顺序。若主节点给不同副本发矛盾提案,会在 Prepare 阶段被多数识破。' },
    { name: 'Prepare 准备', tag: 'PREPARE', effect: '每个副本校验签名与视图后,向所有节点广播 Prepare。收到 2f+1 个一致 Prepare 即进入 prepared。', why: '保证同一视图内不会对同一序号确认两个不同请求,为提交建立法定人数(防冲突)。' },
    { name: 'Commit 提交', tag: 'COMMIT', effect: 'prepared 的节点广播 Commit;收到 2f+1 个 Commit 后进入 committed-local。', why: '跨视图持久化:即使之后更换主节点,已提交的请求仍然有效——这是容错的关键。' },
    { name: 'Reply 已提交', tag: 'REPLY', effect: '各节点执行请求并回复客户端;客户端收到 f+1 个一致回复即确认结果。', why: 'f+1 个一致回复中至少一个来自诚实节点,保证最终结果可信。' },
  ];

  const f = () => Math.floor((SIM.N - 1) / 3);
  const quorum = () => 2 * f() + 1;
  const honestCount = () => { let n = 0; for (let i = 1; i <= SIM.N; i++) if (!SIM.byz[i]) n++; return n; };
  const pos = (i) => { const a = (-90 + (i - 1) * 360 / SIM.N) * Math.PI / 180; return { x: 50 + Math.cos(a) * 34, y: 52 + Math.sin(a) * 36 }; };

  /* 节点层 */
  function nodesHTML() {
    let h = '';
    for (let i = 1; i <= SIM.N; i++) {
      const p = pos(i); const byz = !!SIM.byz[i]; const pri = i === 1;
      const committed = SIM.phase >= 4 && !byz;
      const cls = ['snode', pri ? 'pri' : '', byz ? 'byz' : '', committed ? 'committed' : ''].join(' ');
      const role = byz ? '拜占庭' : (pri ? '主节点' : '副本');
      h += `<div class="${cls}" style="left:${p.x}%;top:${p.y}%" onclick="${pri ? '' : `Chaimir.simFn.toggleByz(${i})`}" title="${pri ? '主节点不可设为作恶' : '点击切换作恶'}">
        <div class="nm">N${i}</div><div class="rl">${role}</div>
        ${committed ? `<span class="ntick">${C.icon('check')}</span>` : ''}</div>`;
    }
    return h;
  }

  /* 消息飞线层(按当前阶段不同) */
  function msgsHTML() {
    const ph = SIM.phase; if (ph === 0 || ph === 4) return '';
    const lines = [];
    const addLine = (a, b, cls) => { const pa = pos(a), pb = pos(b); lines.push(`<line x1="${pa.x}%" y1="${pa.y}%" x2="${pb.x}%" y2="${pb.y}%" class="msgline ${cls}"/>`); };
    if (ph === 1) { // 预准备:主节点广播给副本
      for (let j = 2; j <= SIM.N; j++) addLine(1, j, SIM.byz[1] ? 'bad' : 'pre');
    } else { // 准备/提交:节点间广播(诚实=阶段色,作恶=红)
      const cls = ph === 2 ? 'prepare' : 'commit';
      for (let a = 1; a <= SIM.N; a++) for (let b = a + 1; b <= SIM.N; b++) {
        const bad = SIM.byz[a] || SIM.byz[b];
        addLine(a, b, bad ? 'bad' : cls);
      }
    }
    return lines.join('');
  }

  /* 投票矩阵:每个节点在当前阶段的参与状态 */
  function matrixHTML() {
    const ph = SIM.phase;
    let cells = '';
    for (let i = 1; i <= SIM.N; i++) {
      let st = 'idle', mark = '·';
      if (ph >= 1) {
        if (SIM.byz[i]) { st = 'bad'; mark = '✕'; }
        else if (ph === 1) { st = i === 1 ? 'ok' : 'wait'; mark = i === 1 ? '✓' : '…'; }
        else { st = 'ok'; mark = '✓'; }
      }
      cells += `<div class="vcell ${st}" title="N${i}"><span>N${i}</span><b>${mark}</b></div>`;
    }
    return cells;
  }

  /* 重绘:从 SIM 状态刷新画布与所有面板(单一真相) */
  function repaint() {
    const ph = SIM.phase, P = PHASES[ph];
    const setHTML = (id, html) => { const e = document.getElementById(id); if (e) e.innerHTML = html; };
    const setText = (id, t) => { const e = document.getElementById(id); if (e) e.textContent = t; };
    setHTML('sim-nodes', nodesHTML());
    setHTML('sim-msgs', msgsHTML());
    // 阶段横幅
    const banner = document.getElementById('sim-phase'); if (banner) { banner.textContent = P.tag; banner.style.display = ph === 0 ? 'none' : 'block'; }
    // 时序泳道
    document.querySelectorAll('.ph').forEach((row, idx) => { row.classList.toggle('active', idx + 1 === ph); row.classList.toggle('done', idx + 1 < ph); });
    // 矩阵
    setHTML('sim-matrix', matrixHTML());
    // 叙事:当前阶段效果 + 为什么
    setText('sim-pname', P.name);
    setText('sim-effect', P.effect);
    setText('sim-why', P.why);
    // 法定人数
    const need = quorum(), got = honestCount();
    const pass = got >= need;
    const qbox = document.getElementById('sim-quorum');
    if (qbox) {
      if (ph <= 1) qbox.innerHTML = `<span class="muted">N=${SIM.N} · 可容忍 f=${f()} 个作恶 · 法定人数 2f+1=${need}</span>`;
      else qbox.innerHTML = `诚实节点 <b>${got}</b> / 需 <b>${need}</b> · ${pass ? `<span style="color:#6ee7b7">${C.icon('check-circle-2')} 达到法定人数</span>` : `<span style="color:#fca5a5">${C.icon('x-circle')} 不足,无法推进</span>`}`;
      C.refreshIcons();
    }
    // 结果
    const res = document.getElementById('sim-result');
    if (res) {
      if (ph < 4) res.innerHTML = ph === 0 ? '<span class="muted">点击「下一阶段」或「自动播放」开始</span>' : '<span class="muted">共识进行中…</span>';
      else res.innerHTML = pass
        ? `<span style="color:#6ee7b7">${C.icon('check-circle-2')} 已达成共识 · ${got}/${SIM.N} 节点提交(≥ 2f+1=${need})</span>`
        : `<span style="color:#fca5a5">${C.icon('x-circle')} 共识失败 · 作恶节点超过容错上限 f=${f()}</span>`;
    }
    // 代码追溯高亮
    const hasByz = honestCount() < SIM.N;
    const line = document.getElementById('trace-3'); if (line) line.classList.toggle('hot', hasByz);
    setText('trace-note', hasByz ? '检测到作恶节点 → Prepare 阶段触发 require(verify(m.sig)) 签名校验断言' : '点击副本节点注入拜占庭,观察底层断言被触发');
    // 步进按钮态
    setText('sim-step', `阶段 ${ph}/4 · ${P.name}`);
    C.refreshIcons();
  }

  S.step = () => { if (SIM.phase < 4) { SIM.phase++; repaint(); } };
  S.back = () => { if (SIM.phase > 0) { SIM.phase--; repaint(); } };
  S.play = function () {
    if (S._t) { clearInterval(S._t); S._t = null; const b = document.getElementById('sim-play'); if (b) { b.innerHTML = C.icon('play'); C.refreshIcons(); } return; }
    const b = document.getElementById('sim-play'); if (b) { b.innerHTML = C.icon('pause'); C.refreshIcons(); }
    S._t = setInterval(() => {
      if (!document.getElementById('sim-nodes')) return clearInterval(S._t);
      if (SIM.phase >= 4) { clearInterval(S._t); S._t = null; const p = document.getElementById('sim-play'); if (p) { p.innerHTML = C.icon('play'); C.refreshIcons(); } return; }
      SIM.phase++; repaint();
    }, 1400);
  };
  S.toggleByz = (i) => { SIM.byz[i] = !SIM.byz[i]; repaint(); };
  S.setN = (v) => { SIM.N = +v; SIM.byz = {}; SIM.phase = 0; const lbl = document.getElementById('sim-nval'); if (lbl) lbl.textContent = v; const fl = document.getElementById('sim-fval'); if (fl) fl.textContent = f(); repaint(); };
  S.reset = () => { SIM.phase = 0; SIM.byz = {}; repaint(); };
  S.revealCp = function () {
    SIM.reveal = true;
    const box = document.getElementById('sim-cp-ans');
    if (box) box.innerHTML = `<div class="callout success" style="background:rgba(16,185,129,.1);border-color:#10b981;color:#6ee7b7">${C.icon('check-circle-2')}<div class="text-xs">正确答案:<b>能</b>。N=4 时 f=1,容忍 1 个作恶;剩余 3 个诚实节点恰好达到 2f+1=3 的法定人数,仍能达成共识。若再多 1 个作恶则失败。</div></div>`;
    C.refreshIcons();
  };

  /* 进入页面:重置到就绪态并首次绘制 */
  C.mounts['immersive/sim'] = function () { SIM.phase = 0; SIM.byz = {}; SIM.reveal = false; repaint(); };

  C.registerPages({
    'immersive/sim': () => `<style>
      .sm{position:fixed;inset:0;display:flex;flex-direction:column;background:var(--color-dark-bg);z-index:var(--z-immersive)}
      .sm-bar{height:48px;background:var(--color-terminal-bg);border-bottom:1px solid var(--color-dark-border);display:flex;align-items:center;gap:14px;padding:0 14px;color:var(--color-dark-text);flex-shrink:0}
      .sm-back{display:flex;align-items:center;gap:6px;color:var(--color-dark-text-sub);font-size:var(--text-sm);cursor:pointer}.sm-back:hover{color:#fff}
      .sm-mid{flex:1;display:flex;overflow:hidden}
      .sm-left{width:330px;background:var(--color-dark-surface);border-right:1px solid var(--color-dark-border);overflow:auto;padding:16px;color:var(--color-dark-text);flex-shrink:0}
      .sm-card{background:rgba(255,255,255,.03);border:1px solid var(--color-dark-border);border-radius:var(--radius-sm);padding:12px;margin-bottom:12px}
      .sm-card h4{font-size:var(--text-sm);margin-bottom:8px;display:flex;align-items:center;gap:6px}
      .eff{background:rgba(34,211,238,.08);border-left:3px solid var(--cyan-400);padding:11px 12px;border-radius:0 var(--radius-sm) var(--radius-sm) 0;margin-bottom:10px}
      .eff .ph-name{font-weight:700;color:var(--cyan-300);font-size:var(--text-sm);margin-bottom:4px}
      .eff .lbl{font-size:10px;letter-spacing:.06em;color:var(--color-dark-text-sub);text-transform:uppercase;margin-top:8px}
      .eff p{font-size:var(--text-xs);line-height:1.7;color:#cbd5e1;margin-top:2px}
      .byzbtn{padding:7px 10px;border:1px solid var(--color-dark-border);border-radius:var(--radius-sm);font-size:var(--text-xs);color:var(--color-dark-text-sub);background:rgba(255,255,255,.02)}
      .byzbtn.on{border-color:var(--red-600);color:#fca5a5;background:rgba(239,68,68,.12)}
      .slider{width:100%;accent-color:var(--cyan-500)}
      .trace{background:var(--color-terminal-bg);border:1px solid var(--color-dark-border);border-radius:var(--radius-sm);padding:10px;font-family:var(--font-mono);font-size:11px;line-height:1.7;color:#94a3b8;white-space:pre;overflow:auto}
      .trace .tl.hot{background:rgba(245,158,11,.18);color:var(--amber-300);font-weight:600;border-radius:3px}
      .sm-stage{flex:1;position:relative;background:radial-gradient(circle at 50% 44%,#16223b,#0b1120);overflow:hidden}
      .sm-stagelabel{position:absolute;top:14px;left:16px;color:var(--color-dark-text-sub);font-size:var(--text-xs);letter-spacing:.05em}
      .sm-phasebanner{position:absolute;top:14px;left:50%;transform:translateX(-50%);background:rgba(34,211,238,.14);border:1px solid var(--cyan-500);color:var(--cyan-300);padding:5px 18px;border-radius:var(--radius-full);font-size:var(--text-xs);font-weight:700;letter-spacing:.08em;display:none}
      .sm-svg{position:absolute;inset:0;width:100%;height:100%;pointer-events:none}
      .msgline{stroke-width:2;fill:none;stroke-dasharray:5 7;animation:flow .9s linear infinite;opacity:.9}
      .msgline.pre{stroke:var(--cyan-400)}.msgline.prepare{stroke:#60a5fa}.msgline.commit{stroke:#34d399}.msgline.bad{stroke:#ef4444;stroke-dasharray:3 5;opacity:.85}
      @keyframes flow{to{stroke-dashoffset:-24}}
      .snode{position:absolute;width:74px;height:74px;border-radius:50%;transform:translate(-50%,-50%);display:flex;flex-direction:column;align-items:center;justify-content:center;background:var(--color-dark-surface);border:2px solid var(--color-dark-elevated);color:#cbd5e1;cursor:pointer;transition:all .3s var(--ease);z-index:2}
      .snode.pri{border-color:var(--cyan-400);background:#0c2630;color:var(--cyan-300);box-shadow:0 0 22px rgba(34,211,238,.4)}
      .snode.byz{border-color:#ef4444;background:#2a1616;color:#fca5a5;box-shadow:0 0 22px rgba(239,68,68,.45)}
      .snode.committed{border-color:#10b981;background:#0f2a1e;color:#6ee7b7}
      .snode .nm{font-weight:700;font-size:var(--text-sm)}.snode .rl{font-size:10px;opacity:.85}
      .snode .ntick{position:absolute;top:-4px;right:-4px;width:20px;height:20px;border-radius:50%;background:#10b981;color:#fff;display:grid;place-items:center}
      .snode .ntick .lucide{width:12px;height:12px}
      .sm-right{width:300px;background:var(--color-dark-surface);border-left:1px solid var(--color-dark-border);flex-shrink:0;overflow:auto;padding:16px;color:var(--color-dark-text)}
      .ph{display:flex;align-items:center;gap:10px;padding:8px 0}
      .ph .pic{width:28px;height:28px;border-radius:var(--radius-sm);background:var(--color-dark-elevated);display:grid;place-items:center;flex-shrink:0;color:var(--color-dark-text-sub)}
      .ph.active .pic{background:var(--cyan-500);color:#04252b;box-shadow:0 0 0 4px rgba(34,211,238,.18)}
      .ph.done .pic{background:var(--green-600);color:#fff}
      .ph .pl{font-size:var(--text-sm)}.ph.active .pl{color:#fff;font-weight:600}
      .vmatrix{display:grid;grid-template-columns:repeat(4,1fr);gap:6px;margin-top:8px}
      .vcell{aspect-ratio:1;border-radius:var(--radius-sm);background:var(--color-dark-elevated);display:flex;flex-direction:column;align-items:center;justify-content:center;font-size:10px;color:var(--color-dark-text-sub);transition:all .3s var(--ease)}
      .vcell b{font-size:var(--text-md)}
      .vcell.ok{background:rgba(16,185,129,.22);color:#6ee7b7}.vcell.bad{background:rgba(239,68,68,.22);color:#fca5a5}.vcell.wait{background:rgba(245,158,11,.16);color:var(--amber-300)}
      .sm-ctrl{height:56px;background:var(--color-terminal-bg);border-top:1px solid var(--color-dark-border);display:flex;align-items:center;gap:12px;padding:0 18px;flex-shrink:0}
      .cbtn{width:38px;height:38px;border-radius:var(--radius-sm);background:var(--color-dark-elevated);color:#fff;display:grid;place-items:center}.cbtn:hover{background:var(--cyan-600)}.cbtn.play{background:var(--cyan-600)}
      .spd button{padding:4px 10px;font-size:var(--text-xs);color:var(--color-dark-text-sub);border-radius:var(--radius-xs)}.spd button.on{background:var(--cyan-600);color:#fff}
    </style>
    <div class="sm">
      <div class="sm-bar">
        <span class="sm-back" onclick="Chaimir.navigate('student/experiments')">${C.icon('arrow-left')} 返回实验</span>
        <span class="fw-600 text-sm">算法仿真 · PBFT 拜占庭容错共识</span>
        <span class="badge" style="background:rgba(34,211,238,.15);color:var(--cyan-300)">图网络 · 时序泳道 · 投票矩阵</span>
        <div style="flex:1"></div>
        <span class="text-xs muted mono">seed 0x7af3 · 确定性可复现</span>
      </div>
      <div class="sm-mid">
        <div class="sm-left">
          <div class="sm-card"><h4>${C.icon('graduation-cap')} 当前阶段效果</h4>
            <div class="eff"><div class="ph-name" id="sim-pname">就绪</div>
              <div class="lbl">这一步做了什么</div><p id="sim-effect"></p>
              <div class="lbl">为什么重要</div><p id="sim-why"></p></div>
            <div class="text-xs" id="sim-quorum" style="padding:8px 10px;background:rgba(255,255,255,.03);border-radius:var(--radius-sm)"></div>
          </div>
          <div class="sm-card"><h4>${C.icon('sliders-horizontal')} 节点总数 N</h4>
            <input type="range" class="slider" min="4" max="7" value="4" oninput="Chaimir.simFn.setN(this.value)">
            <div class="text-xs muted mt-2">N=<b id="sim-nval">4</b> · 可容忍 f=<b id="sim-fval">1</b> 个拜占庭(N=3f+1)</div></div>
          <div class="sm-card"><h4>${C.icon('bug')} 注入拜占庭 <span class="badge badge-red" style="margin-left:auto">攻击</span></h4>
            <div class="muted text-xs mb-2">点击画布副本节点,或下方按钮,使其发送矛盾/错误消息</div>
            <div class="flex gap-2 wrap">${[2, 3, 4].map(n => `<button class="byzbtn" onclick="Chaimir.simFn.toggleByz(${n})">N${n} 作恶</button>`).join('')}</div></div>
          <div class="sm-card"><h4>${C.icon('help-circle')} 设问检查点</h4>
            <p class="text-xs" style="color:#cbd5e1;line-height:1.7">预测:N=4 且 N4 作恶时,还能达成共识吗?</p>
            <div class="flex gap-2 mt-2"><button class="btn btn-on-dark btn-sm" onclick="Chaimir.simFn.revealCp()">能</button><button class="btn btn-on-dark btn-sm" onclick="Chaimir.simFn.revealCp()">不能</button></div>
            <div id="sim-cp-ans" class="mt-2"></div></div>
          <div class="sm-card"><h4>${C.icon('code')} VM 底层断言追溯</h4>
            <div class="muted text-xs mb-2" id="trace-note">点击副本节点注入拜占庭,观察底层断言被触发</div>
            <div class="trace"><span class="tl">1: contract PBFTValidator {</span>
<span class="tl">2:   function checkPrepare(Msg m) {</span>
<span class="tl" id="trace-3">3:     require(verify(m.sig), "BadSig");</span>
<span class="tl">4:     require(m.view == currentView);</span>
<span class="tl">5:     votes[m.sender] = m.vote;</span>
<span class="tl">6:   } }</span></div></div>
        </div>
        <div class="sm-stage">
          <div class="sm-stagelabel">图网络模式 · 环形布局 · 消息飞线随阶段变化</div>
          <div class="sm-phasebanner" id="sim-phase">PRE-PREPARE</div>
          <svg class="sm-svg" id="sim-msgs" preserveAspectRatio="none"></svg>
          <div id="sim-nodes"></div>
        </div>
        <div class="sm-right">
          <div class="fw-600 text-sm mb-2" style="color:#fff">共识阶段 · 时序泳道</div>
          ${[['Pre-Prepare', 'megaphone'], ['Prepare', 'check-check'], ['Commit', 'lock'], ['Reply', 'corner-down-left']].map(([l, ic]) => `
            <div class="ph"><span class="pic">${C.icon(ic)}</span><span class="pl">${l}</span></div>`).join('')}
          <div class="fw-600 text-sm mb-2 mt-4" style="color:#fff">投票矩阵</div>
          <div class="vmatrix" id="sim-matrix"></div>
          <div class="fw-600 text-sm mb-2 mt-4" style="color:#fff">共识结果</div>
          <div id="sim-result" class="text-sm muted">点击「下一阶段」或「自动播放」开始</div>
        </div>
      </div>
      <div class="sm-ctrl">
        <button class="cbtn" onclick="Chaimir.simFn.back()" title="上一阶段">${C.icon('skip-back')}</button>
        <button class="cbtn play" id="sim-play" onclick="Chaimir.simFn.play()" title="自动播放">${C.icon('play')}</button>
        <button class="cbtn" onclick="Chaimir.simFn.step()" title="下一阶段">${C.icon('skip-forward')}</button>
        <button class="cbtn" onclick="Chaimir.simFn.reset()" title="重置">${C.icon('rotate-ccw')}</button>
        <span class="text-xs mono" id="sim-step" style="color:var(--color-dark-text-sub);margin-left:6px">阶段 0/4 · 就绪</span>
        <div style="flex:1"></div>
        <span class="text-xs muted">速度</span>
        <div class="spd"><button>0.5×</button><button class="on">1×</button><button>2×</button></div>
      </div>
    </div>`,
  });
})();
