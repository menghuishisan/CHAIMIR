/* ============================================================
   pages/immersive/battle-replay.js — 对抗赛对局回放(全屏沉浸)
   ------------------------------------------------------------
   对应 M8 竞赛对抗赛回放。布局:顶(对局信息)+ 中(攻防拓扑 +
   交易/事件日志流)+ 底(时间轴回溯条:逐笔交易刻度/播放/变速)。
   创意复活:区块时空回溯器(拖时间轴倒带 VM 执行树)。
   ============================================================ */
(function () {
  const C = window.Chaimir;
  const B = C.brFn = {};
  const LOG = C.mock.battleLog;
  const kindColor = { deploy: 'var(--cyan-300)', attack: '#fca5a5', defend: '#6ee7b7', settle: 'var(--amber-300)' };
  const kindText = { deploy: '部署', attack: '攻击', defend: '防御', settle: '结算' };
  B._i = 2;

  B.goto = function (i) {
    B._i = Math.max(0, Math.min(LOG.length - 1, i));
    const bar = document.getElementById('br-bar'); if (bar) bar.style.width = (B._i / (LOG.length - 1) * 100) + '%';
    document.querySelectorAll('.br-dot').forEach((d, idx) => d.classList.toggle('on', idx <= B._i));
    const tick = document.getElementById('br-tick'); if (tick) tick.textContent = `区块高度 #${LOG[B._i].h} · 第 ${B._i + 1}/${LOG.length} 步`;
    const stream = document.getElementById('br-stream');
    if (stream) stream.innerHTML = LOG.slice(0, B._i + 1).map((l, idx) => `
      <div class="br-line ${idx === B._i ? 'cur' : ''}" style="border-left-color:${kindColor[l.kind]}">
        <div class="flex justify-between"><span class="mono text-xs" style="color:${kindColor[l.kind]}">[#${l.h}] ${kindText[l.kind]}</span>${idx === B._i ? '<span class="badge" style="background:rgba(34,211,238,.15);color:var(--cyan-300)">当前</span>' : ''}</div>
        <div class="text-xs" style="color:var(--color-dark-text-sub);margin-top:3px">${C.esc(l.text)}</div></div>`).join('');
    // 拓扑高亮:攻击步亮红线,防御/结算步亮守方
    const atk = document.getElementById('br-atk'), def = document.getElementById('br-def'), wire = document.getElementById('br-wire');
    const k = LOG[B._i].kind;
    if (atk) atk.classList.toggle('hot', k === 'attack');
    if (def) def.classList.toggle('hot', k === 'defend' || k === 'settle');
    if (wire) wire.setAttribute('stroke', k === 'attack' ? '#ef4444' : (k === 'defend' || k === 'settle') ? '#10b981' : '#475569');
  };
  B.play = function () {
    if (B._timer) { clearInterval(B._timer); B._timer = null; const pb = document.getElementById('br-play'); if (pb) pb.innerHTML = C.icon('play'); C.refreshIcons(); return; }
    const pb = document.getElementById('br-play'); if (pb) pb.innerHTML = C.icon('pause'); C.refreshIcons();
    B._timer = setInterval(() => {
      if (!document.getElementById('br-bar')) return clearInterval(B._timer);
      if (B._i >= LOG.length - 1) { clearInterval(B._timer); B._timer = null; const p = document.getElementById('br-play'); if (p) p.innerHTML = C.icon('play'); C.refreshIcons(); return; }
      B.goto(B._i + 1);
    }, 1100);
  };
  C.mounts['immersive/battle-replay'] = function () { B._i = 2; B.goto(2); };

  C.registerPages({
    'immersive/battle-replay': () => `<style>
      .br{position:fixed;inset:0;display:flex;flex-direction:column;background:var(--color-dark-bg);z-index:var(--z-immersive);color:var(--color-dark-text)}
      .br-bar-top{height:48px;background:var(--color-terminal-bg);border-bottom:1px solid var(--color-dark-border);display:flex;align-items:center;gap:14px;padding:0 14px;flex-shrink:0}
      .br-back{display:flex;align-items:center;gap:6px;color:var(--color-dark-text-sub);font-size:var(--text-sm);cursor:pointer}.br-back:hover{color:#fff}
      .br-mid{flex:1;display:flex;overflow:hidden}
      .br-stage{flex:1;position:relative;background:radial-gradient(circle at 50% 40%,#16223b,#0b1120);overflow:hidden}
      .br-stagelabel{position:absolute;top:14px;left:16px;color:var(--color-dark-text-sub);font-size:var(--text-xs);letter-spacing:.05em}
      .bnode{position:absolute;width:120px;padding:12px;border-radius:var(--radius);transform:translate(-50%,-50%);background:var(--color-dark-surface);border:2px solid var(--color-dark-elevated);text-align:center;transition:all .3s var(--ease)}
      .bnode .bt{font-weight:700;font-size:var(--text-sm)}.bnode .bs{font-size:11px;color:var(--color-dark-text-sub);margin-top:3px}
      .bnode.def{border-color:var(--green-600)}.bnode.def.hot{box-shadow:0 0 22px rgba(16,185,129,.5)}
      .bnode.atk{border-color:var(--red-600)}.bnode.atk.hot{box-shadow:0 0 22px rgba(239,68,68,.5)}
      .br-right{width:340px;background:var(--color-dark-surface);border-left:1px solid var(--color-dark-border);flex-shrink:0;overflow:auto;padding:14px}
      .br-line{padding:9px 11px;border-left:3px solid;background:rgba(255,255,255,.02);border-radius:0 var(--radius-sm) var(--radius-sm) 0;margin-bottom:8px}
      .br-line.cur{background:rgba(34,211,238,.08)}
      .br-ctrl{background:var(--color-terminal-bg);border-top:1px solid var(--color-dark-border);padding:12px 20px;flex-shrink:0}
      .br-track{position:relative;height:6px;background:var(--color-dark-elevated);border-radius:var(--radius-full);margin:14px 0 10px}
      .br-fill{position:absolute;left:0;top:0;height:100%;background:var(--cyan-500);border-radius:var(--radius-full);transition:width .4s var(--ease)}
      .br-dots{position:absolute;inset:-5px 0 0;display:flex;justify-content:space-between}
      .br-dot{width:16px;height:16px;border-radius:50%;background:var(--color-dark-elevated);border:3px solid var(--color-terminal-bg);cursor:pointer;transition:all .2s}
      .br-dot.on{background:var(--cyan-400)}
      .cbtn{width:36px;height:36px;border-radius:var(--radius-sm);background:var(--color-dark-elevated);color:#fff;display:grid;place-items:center}.cbtn:hover{background:var(--cyan-600)}.cbtn.play{background:var(--cyan-600)}
      .spd button{padding:4px 10px;font-size:var(--text-xs);color:var(--color-dark-text-sub);border-radius:var(--radius-xs)}.spd button.on{background:var(--cyan-600);color:#fff}
    </style>
    <div class="br">
      <div class="br-bar-top">
        <span class="br-back" onclick="Chaimir.navigate('student/contests')">${C.icon('arrow-left')} 返回竞赛</span>
        <span class="fw-600 text-sm">对局回放 #2847 · 金库重入渗透赛</span>
        <div style="flex:1"></div>
        <span class="badge" style="background:rgba(16,185,129,.15);color:#6ee7b7">守方胜 · 资产零损耗</span>
        <span class="text-xs mono" style="color:var(--amber-300)">ELO +18</span>
      </div>
      <div class="br-mid">
        <div class="br-stage">
          <div class="br-stagelabel">攻防拓扑 · 交易重放</div>
          <svg style="position:absolute;inset:0;width:100%;height:100%" preserveAspectRatio="none"><line id="br-wire" x1="30%" y1="50%" x2="70%" y2="50%" stroke="#475569" stroke-width="2" stroke-dasharray="6 6"/></svg>
          <div class="bnode def hot" id="br-def" style="left:30%;top:50%"><div class="bt" style="color:#6ee7b7">守方金库合约</div><div class="bs">nonReentrant 状态锁</div></div>
          <div class="bnode atk" id="br-atk" style="left:70%;top:50%"><div class="bt" style="color:#fca5a5">攻方渗透脚本</div><div class="bs">receive() 递归提取</div></div>
        </div>
        <div class="br-right">
          <div class="fw-600 text-sm mb-3" style="color:#fff">${C.icon('scroll-text')} 链上交易 / 事件日志</div>
          <div id="br-stream"></div>
        </div>
      </div>
      <div class="br-ctrl">
        <div class="fw-600 text-xs" style="color:var(--color-dark-text-sub)">区块高度状态调节轮 · 拖拽或点击刻度可倒带重组 VM 执行树</div>
        <div class="br-track"><div class="br-fill" id="br-bar"></div>
          <div class="br-dots">${LOG.map((l, i) => `<span class="br-dot" onclick="Chaimir.brFn.goto(${i})" title="#${l.h}"></span>`).join('')}</div></div>
        <div class="flex items-center gap-2">
          <button class="cbtn" onclick="Chaimir.brFn.goto(Chaimir.brFn._i-1)" title="上一步">${C.icon('skip-back')}</button>
          <button class="cbtn play" id="br-play" onclick="Chaimir.brFn.play()" title="播放/暂停">${C.icon('play')}</button>
          <button class="cbtn" onclick="Chaimir.brFn.goto(Chaimir.brFn._i+1)" title="下一步">${C.icon('skip-forward')}</button>
          <span class="text-xs mono" id="br-tick" style="color:var(--color-dark-text-sub);margin-left:6px">区块高度 #1026 · 第 3/4 步</span>
          <div style="flex:1"></div>
          <span class="text-xs muted">速度</span>
          <div class="spd"><button>0.5×</button><button class="on">1×</button><button>2×</button></div>
        </div>
      </div>
    </div>`,
  });
})();
