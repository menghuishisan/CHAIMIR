/* ============================================================
   PBFT 仿真 — 可视化模式引擎演示
   演示思路:仿真只声明 状态 + 模式映射 + 交互,
   下面的"模式引擎"(图网络/时序泳道/投票矩阵)负责渲染与动画。
   ============================================================ */

// ---------- 仿真状态(由"仿真包"定义) ----------
let S = {
  n: 4,                 // 节点数
  f: 1,                 // 容错数
  byzantine: new Set(), // 拜占庭节点索引
  phase: -1,            // -1就绪 0 PrePrepare 1 Prepare 2 Commit 3完成
  votes: [],            // 各节点投票:null/true/false
  committed: false,
  tick: 0,
};
const PHASES = ['Pre-Prepare', 'Prepare', 'Commit', '已达成共识'];
const PHASE_DESC = {
  '-1': '点击「主节点发起提案」开始。主节点(N1,琥珀色)将向所有副本广播提案。',
  '0': 'Pre-Prepare:主节点 N1 向所有副本节点广播提案消息(琥珀飞线)。',
  '1': 'Prepare:各副本验证提案后,互相广播 Prepare 投票(蓝色飞线)。需收集到 2f+1 个一致投票。',
  '2': 'Commit:节点确认 Prepare 达到 2f+1 后,广播 Commit 消息(绿色飞线),准备提交。',
  '3': null, // 动态生成
};

let playing = false, speed = 1, timer = null;
const STAGE = () => document.getElementById('stage');

// ---------- 模式引擎 1:图网络(环形布局 + 节点渲染) ----------
function layoutNodes() {
  STAGE().querySelectorAll('.node').forEach(n => n.remove());
  const rect = STAGE().getBoundingClientRect();
  const cx = rect.width / 2, cy = rect.height / 2;
  const R = Math.min(rect.width, rect.height) * 0.30;
  S._pos = [];
  for (let i = 0; i < S.n; i++) {
    const ang = -Math.PI / 2 + i * (2 * Math.PI / S.n);
    const x = cx + R * Math.cos(ang), y = cy + R * Math.sin(ang);
    S._pos.push([x, y]);
    const isPrimary = i === 0;
    const isByz = S.byzantine.has(i);
    const node = document.createElement('div');
    node.className = 'node' + (isPrimary ? ' primary' : '') + (isByz ? ' byzantine' : '');
    node.id = 'node' + i;
    node.style.left = x + 'px'; node.style.top = y + 'px';
    node.innerHTML = `
      <div class="node-core">
        <div class="nlabel">N${i + 1}</div>
        <div class="nrole">${isByz ? '拜占庭' : isPrimary ? '主节点' : '副本'}</div>
      </div>
      <div class="vote-dot"><i data-lucide="check" style="width:11px;height:11px"></i></div>`;
    node.onclick = () => toggleByzantine(i);
    STAGE().appendChild(node);
  }
  lucide.createIcons();
}

// 飞线动画:从 src 节点飞到 dst 节点
function flyMsg(src, dst, type, delay) {
  return new Promise(resolve => {
    setTimeout(() => {
      const [x1, y1] = S._pos[src], [x2, y2] = S._pos[dst];
      const m = document.createElement('div');
      m.className = 'msg ' + type;
      m.style.left = x1 + 'px'; m.style.top = y1 + 'px';
      STAGE().appendChild(m);
      const dur = 700 / speed;
      m.animate([{ left: x1 + 'px', top: y1 + 'px' }, { left: x2 + 'px', top: y2 + 'px' }],
        { duration: dur, easing: 'ease-in-out' });
      setTimeout(() => { m.remove(); resolve(); }, dur);
    }, delay / speed);
  });
}

// ---------- 模式引擎 2:时序泳道(三阶段进度) ----------
function renderPhaseTrack() {
  const phases = [['Pre-Prepare', 'megaphone'], ['Prepare', 'check-check'], ['Commit', 'lock']];
  document.getElementById('phaseTrack').innerHTML = phases.map((p, i) => {
    const st = S.phase > i || S.phase === 3 ? 'done' : S.phase === i ? 'active' : '';
    const pct = S.phase > i || S.phase === 3 ? 100 : S.phase === i ? 60 : 0;
    return `<div class="phase-row ${st}">
      <div class="phase-ico"><i data-lucide="${p[1]}" style="width:15px;height:15px"></i></div>
      <div style="flex:1">
        <div class="text-xs" style="color:#cbd5e1">${p[0]}</div>
        <div class="phase-bar"><span style="width:${pct}%"></span></div>
      </div>
    </div>`;
  }).join('');
  lucide.createIcons();
}

// ---------- 模式引擎 3:投票矩阵 ----------
function renderVoteMatrix() {
  document.getElementById('vmatrix').innerHTML = Array.from({ length: S.n }).map((_, i) => {
    const v = S.votes[i];
    const cls = v === true ? 'ok' : v === false ? 'bad' : '';
    const ic = v === true ? '✓' : v === false ? '✗' : ('N' + (i + 1));
    return `<div class="vcell ${cls}">${ic}</div>`;
  }).join('');
}

// ---------- 交互:节点数 ----------
function setNodes(v) {
  S.n = +v; S.f = Math.floor((S.n - 1) / 3);
  document.getElementById('nVal').textContent = S.n;
  document.getElementById('fVal').textContent = S.f;
  // 拜占庭集合清掉超出范围的
  S.byzantine = new Set([...S.byzantine].filter(i => i < S.n));
  resetSim();
}

// ---------- 交互:拜占庭选择 ----------
function renderByzPick() {
  document.getElementById('byzPick').innerHTML = Array.from({ length: S.n }).map((_, i) => {
    if (i === 0) return ''; // 主节点演示中不设为拜占庭(简化)
    const on = S.byzantine.has(i);
    return `<span class="badge ${on ? 'badge-red' : 'badge-gray'}" style="cursor:pointer;padding:6px 11px" onclick="toggleByzantine(${i})">
      <i data-lucide="${on ? 'bug' : 'plus'}" style="width:12px;height:12px"></i> N${i + 1}</span>`;
  }).join('');
  lucide.createIcons();
}
function toggleByzantine(i) {
  if (i === 0) return;
  if (S.byzantine.has(i)) S.byzantine.delete(i); else S.byzantine.add(i);
  resetSim();
}

// ---------- 共识流程(状态机驱动 + 模式引擎渲染) ----------
async function startConsensus() {
  resetSim(true);
  S.phase = 0; updateAll();
  banner('Pre-Prepare', true);
  // Pre-Prepare:主节点 → 所有副本
  await Promise.all(Array.from({ length: S.n }).map((_, i) => i === 0 ? null : flyMsg(0, i, 'pre', i * 120)).filter(Boolean));
  await sleep(300);

  // Prepare:各节点互相广播投票
  S.phase = 1; updateAll(); banner('Prepare');
  for (let i = 0; i < S.n; i++) {
    // 拜占庭节点投反对/矛盾票
    S.votes[i] = S.byzantine.has(i) ? false : true;
    const node = document.getElementById('node' + i);
    if (node) {
      node.classList.add(S.votes[i] ? 'voted' : 'rejected');
      if (!S.votes[i]) node.querySelector('.vote-dot').innerHTML = '<i data-lucide="x" style="width:11px;height:11px"></i>';
    }
  }
  lucide.createIcons();
  // 飞线:每个诚实节点向其他广播 prepare
  const flights = [];
  for (let i = 0; i < S.n; i++) for (let j = 0; j < S.n; j++)
    if (i !== j) flights.push(flyMsg(i, j, S.byzantine.has(i) ? 'bad' : 'prepare', (i * S.n + j) * 30));
  renderVoteMatrix();
  await Promise.all(flights);
  await sleep(300);

  // Commit:统计 2f+1
  S.phase = 2; updateAll(); banner('Commit');
  const yes = S.votes.filter(v => v === true).length;
  const need = 2 * S.f + 1;
  await Promise.all(Array.from({ length: S.n }).map((_, i) =>
    S.votes[i] ? flyMsg(i, (i + 1) % S.n, 'commit', i * 100) : null).filter(Boolean));
  await sleep(400);

  // 结果
  S.committed = yes >= need;
  S.phase = 3; updateAll(); banner(S.committed ? '✓ 共识达成' : '✗ 共识失败');
  if (S.committed) {
    for (let i = 0; i < S.n; i++) if (S.votes[i]) document.getElementById('node' + i)?.classList.add('committed');
  }
  showResult(yes, need);
}

function showResult(yes, need) {
  const el = document.getElementById('result');
  if (S.committed) {
    el.innerHTML = `<div class="badge badge-green" style="margin-bottom:6px"><i data-lucide="check" style="width:12px;height:12px"></i> 共识达成</div>
      <div class="text-xs" style="color:var(--slate-400);line-height:1.7">收集到 ${yes} 个一致投票 ≥ 2f+1=${need},提案提交成功。${S.byzantine.size}个拜占庭节点未能阻止共识。</div>`;
  } else {
    el.innerHTML = `<div class="badge badge-red" style="margin-bottom:6px"><i data-lucide="x" style="width:12px;height:12px"></i> 共识失败</div>
      <div class="text-xs" style="color:var(--slate-400);line-height:1.7">仅 ${yes} 个一致投票 < 2f+1=${need}。拜占庭节点过多(${S.byzantine.size} > f=${S.f}),系统无法达成共识。</div>`;
  }
  lucide.createIcons();
}

// ---------- 辅助 ----------
function sleep(ms) { return new Promise(r => setTimeout(r, ms / speed)); }
function banner(text, show) {
  const b = document.getElementById('phaseBanner');
  b.style.display = 'block'; b.textContent = text;
}
function updateNarrative() {
  let txt;
  if (S.phase === 3) {
    txt = S.committed
      ? `共识达成!即使存在 ${S.byzantine.size} 个拜占庭节点,只要不超过 f=${S.f},诚实节点收集到 2f+1=${2*S.f+1} 个一致投票即可安全提交。这正是 PBFT 的容错保证。`
      : `共识失败!拜占庭节点数 ${S.byzantine.size} 超过了容错上限 f=${S.f}。一致投票不足 2f+1,系统无法安全提交 —— 这说明 PBFT 的安全边界被突破了。`;
  } else {
    txt = PHASE_DESC[S.phase];
  }
  document.getElementById('narrText').textContent = txt;
}
function updateAll() {
  renderPhaseTrack(); renderVoteMatrix(); updateNarrative();
  S.tick++;
  document.getElementById('tickInfo').textContent = `Tick ${S.tick} · ${S.phase < 0 ? '就绪' : PHASES[S.phase]}`;
}

// ---------- 重置 ----------
function resetSim(keepRunning) {
  S.phase = -1; S.committed = false; S.votes = new Array(S.n).fill(null); S.tick = 0;
  document.getElementById('phaseBanner').style.display = 'none';
  document.getElementById('result').innerHTML = '等待发起...';
  document.getElementById('tickInfo').textContent = 'Tick 0 · 就绪';
  layoutNodes(); renderPhaseTrack(); renderVoteMatrix(); renderByzPick();
  if (!keepRunning) updateNarrative();
}

// ---------- 时间控制(演示) ----------
function togglePlay() {
  if (S.phase >= 3 || S.phase === -1) startConsensus();
}
function stepFwd() { if (S.phase < 0) startConsensus(); }
function stepBack() { resetSim(); }
function setSpd(el, v) { speed = v; document.querySelectorAll('.spd button').forEach(b => b.classList.remove('active')); el.classList.add('active'); }

// ---------- 左侧 Tab ----------
function ltab(el, i) {
  document.querySelectorAll('.slt').forEach(t => t.classList.remove('active'));
  el.classList.add('active');
  document.getElementById('lb0').style.display = i === 0 ? 'block' : 'none';
  document.getElementById('lb1').style.display = i === 1 ? 'block' : 'none';
}

// ---------- 启动 ----------
window.addEventListener('DOMContentLoaded', () => {
  S.f = Math.floor((S.n - 1) / 3);
  resetSim();
  lucide.createIcons();
});
window.addEventListener('resize', () => { layoutNodes(); });
