/* ============================================================
   pages/immersive/exp-ide.js — 代码实验工作台(全屏沉浸)
   ------------------------------------------------------------
   对应 M7 实验 + M2 沙箱 + M3 评测。布局:顶栏(含"区块孵化器"
   VM 活体状态)+ 左(文件树/检查点)+ 中(Monaco 式编辑器 + 终端)
   + 右(检查点判分 + 链上操作 + 说明)+ 底部 WS 启动进度。
   创意复活:区块孵化器(VM 状态机活体指示)。
   ============================================================ */
(function () {
  const C = window.Chaimir;
  const S = C.expFn = {};

  S.run = function () {
    const dot = document.getElementById('inc-dot'), txt = document.getElementById('inc-txt'), term = document.getElementById('exp-term');
    if (!dot) return;
    dot.className = 'inc-dot mining'; txt.textContent = '编译中 · 正在挖矿封块…';
    term.insertAdjacentHTML('beforeend', `<div style="color:var(--cyan-400)">$ npx hardhat run scripts/mine.js</div><div class="muted">[VM] 正在发起自动化评测断言校验,寻找满足难度的哈希…</div>`);
    term.scrollTop = term.scrollHeight;
    setTimeout(() => {
      dot.className = 'inc-dot ok'; txt.textContent = '已封块 #8921 · 就绪';
      term.insertAdjacentHTML('beforeend', `<div style="color:var(--green-400,#34d399)">[VM] 成功打包区块 #8921,合约检查点断言判定通过。</div><div>$ _</div>`);
      term.scrollTop = term.scrollHeight;
      const cp = document.querySelector('#exp-cp-1 .cp-flag'); if (cp) { cp.className = 'cp-flag pass'; cp.textContent = '通过'; }
      C.toast('success', '运行完成', '区块已封装,检查点 1 判定通过');
    }, 1500);
  };
  S.judge = function (n) {
    const flag = document.querySelector(`#exp-cp-${n} .cp-flag`); if (!flag) return;
    flag.className = 'cp-flag judging'; flag.textContent = '判题中';
    C.toast('info', '已提交判题', '正在沙箱中运行测试用例…');
    setTimeout(() => { flag.className = 'cp-flag pass'; flag.textContent = '通过'; C.toast('success', `检查点 ${n} 通过`, '断言全部满足'); }, 1400);
  };
  S.chain = (op) => C.toast('info', '链上操作', { deploy: '正在部署合约到沙箱链…', tx: '已发送交易,等待打包…', query: '查询链上状态成功', reset: '链状态已重置到初始区块' }[op]);

  C.mounts['immersive/exp-ide'] = function () {
    let i = 0; const bar = document.getElementById('exp-ws'); if (!bar) return;
    const steps = bar.querySelectorAll('.ws-phase');
    const t = setInterval(() => { if (!document.getElementById('exp-ws')) return clearInterval(t); if (i < steps.length) { steps[i].classList.add('done'); i++; } else clearInterval(t); }, 700);
  };

  const FILES = [['contracts', 'folder', 1], ['Miner.sol', 'file-code', 2, true], ['Vault.sol', 'file-code', 2], ['test', 'folder', 1], ['mine.test.js', 'file-code', 2], ['hardhat.config.js', 'file-cog', 1]];
  const CODE = `<span class="ln">1</span><span class="cm">// SPDX-License-Identifier: MIT</span>
<span class="ln">2</span><span class="kw">pragma</span> solidity ^0.8.20;
<span class="ln">3</span>
<span class="ln">4</span><span class="kw">contract</span> <span class="ty">Miner</span> {
<span class="ln">5</span>    <span class="kw">uint256</span> <span class="kw">public</span> difficulty = <span class="nm">4</span>;
<span class="ln">6</span>    <span class="kw">bytes32</span> <span class="kw">public</span> lastHash;
<span class="ln">7</span>
<span class="ln">8</span>    <span class="kw">function</span> <span class="fn">mine</span>(<span class="kw">uint256</span> nonce) <span class="kw">external</span> {
<span class="ln">9</span>        <span class="kw">bytes32</span> h = <span class="fn">keccak256</span>(<span class="fn">abi.encodePacked</span>(block.number, nonce));
<span class="ln">10</span>        <span class="fn">require</span>(<span class="kw">uint256</span>(h) < (<span class="nm">2</span>**<span class="nm">256</span> - <span class="nm">1</span>) >> difficulty, <span class="st">"not solved"</span>);
<span class="ln">11</span>        lastHash = h;
<span class="ln">12</span>        <span class="kw">emit</span> <span class="fn">BlockMined</span>(msg.sender, nonce, h);
<span class="ln">13</span>    }
<span class="ln">14</span>}`;

  C.registerPages({
    'immersive/exp-ide': () => `<style>
      .imm{position:fixed;inset:0;display:flex;flex-direction:column;background:var(--color-dark-bg);z-index:var(--z-immersive)}
      .imm-bar{height:48px;background:var(--color-terminal-bg);border-bottom:1px solid var(--color-dark-border);display:flex;align-items:center;gap:14px;padding:0 14px;color:var(--color-dark-text);flex-shrink:0}
      .imm-back{display:flex;align-items:center;gap:6px;color:var(--color-dark-text-sub);font-size:var(--text-sm);cursor:pointer}.imm-back:hover{color:#fff}
      .inc{display:flex;align-items:center;gap:8px;background:rgba(255,255,255,.04);border:1px solid var(--color-dark-border);padding:5px 12px;border-radius:var(--radius-full);font-size:var(--text-xs);font-family:var(--font-mono);color:var(--color-dark-text-sub)}
      .inc-dot{width:8px;height:8px;border-radius:50%;background:var(--cyan-400)}
      .inc-dot.mining{background:var(--amber-400);animation:incp .6s infinite alternate}
      .inc-dot.ok{background:var(--green-400,#34d399)}
      @keyframes incp{from{opacity:.35}to{opacity:1}}
      .imm-work{flex:1;display:flex;overflow:hidden}
      .imm-side{width:248px;background:var(--color-dark-surface);border-right:1px solid var(--color-dark-border);display:flex;flex-direction:column;flex-shrink:0}
      .imm-side-h{padding:10px 14px;font-size:11px;letter-spacing:.06em;color:var(--color-dark-text-sub);text-transform:uppercase}
      .ftree{padding:0 8px;font-size:var(--text-sm);color:var(--color-dark-text-sub)}
      .ftree .fi{display:flex;align-items:center;gap:8px;padding:6px 8px;border-radius:var(--radius-xs);cursor:pointer}
      .ftree .fi:hover{background:rgba(255,255,255,.05);color:#fff}
      .ftree .fi.on{background:rgba(34,211,238,.12);color:var(--cyan-300)}
      .ftree .fi .lucide{width:15px;height:15px}
      .imm-main{flex:1;display:flex;flex-direction:column;min-width:0;background:var(--color-editor-bg)}
      .etabs{height:38px;background:var(--color-dark-surface);display:flex;align-items:center;gap:2px;padding:0 8px;border-bottom:1px solid var(--color-dark-border)}
      .etab{padding:6px 14px;font-size:var(--text-xs);color:var(--color-dark-text-sub);border-radius:var(--radius-xs) var(--radius-xs) 0 0;cursor:pointer}
      .etab.on{background:var(--color-editor-bg);color:#fff}
      .editor{flex:1;overflow:auto;padding:14px 16px;font-family:var(--font-mono);font-size:13px;line-height:1.75;white-space:pre;color:#c8d3e6;tab-size:4}
      .editor .ln{display:inline-block;width:30px;color:#3f5374;user-select:none;text-align:right;margin-right:16px}
      .editor .kw{color:#5ad1 e6} .editor .kw{color:var(--cyan-300)} .editor .ty{color:#7dd3fc} .editor .fn{color:var(--amber-300)} .editor .st{color:#86efac} .editor .nm{color:#fca5a5} .editor .cm{color:#5b6b86;font-style:italic}
      .term{height:184px;background:var(--color-terminal-bg);border-top:1px solid var(--color-dark-border);padding:12px 14px;font-family:var(--font-mono);font-size:12.5px;line-height:1.7;color:#cbd5e1;overflow:auto;flex-shrink:0}
      .imm-right{width:330px;background:var(--color-dark-surface);border-left:1px solid var(--color-dark-border);flex-shrink:0;overflow:auto;padding:16px;color:var(--color-dark-text)}
      .imm-card{background:rgba(255,255,255,.03);border:1px solid var(--color-dark-border);border-radius:var(--radius-sm);padding:12px;margin-bottom:12px}
      .imm-card h4{font-size:var(--text-sm);margin-bottom:8px;display:flex;align-items:center;gap:6px}
      .cp{display:flex;align-items:flex-start;gap:10px;padding:10px;border:1px solid var(--color-dark-border);border-radius:var(--radius-sm);margin-bottom:8px;font-size:var(--text-xs)}
      .cp .cp-n{width:18px;height:18px;border-radius:50%;background:var(--color-dark-elevated);color:var(--color-dark-text-sub);display:grid;place-items:center;font-size:10px;font-weight:700;flex-shrink:0}
      .cp-flag{margin-left:auto;font-size:11px;font-weight:600;padding:2px 8px;border-radius:var(--radius-full);background:var(--color-dark-elevated);color:var(--color-dark-text-sub)}
      .cp-flag.pass{background:rgba(16,185,129,.2);color:#6ee7b7}.cp-flag.judging{background:rgba(245,158,11,.2);color:var(--amber-300)}
      .imm-ws{height:36px;background:var(--color-terminal-bg);border-top:1px solid var(--color-dark-border);display:flex;align-items:center;gap:18px;padding:0 16px;flex-shrink:0;font-size:var(--text-xs);color:var(--color-dark-text-sub)}
      .ws-phase{display:flex;align-items:center;gap:6px}.ws-phase .d{width:7px;height:7px;border-radius:50%;background:var(--color-dark-elevated)}
      .ws-phase.done{color:var(--cyan-300)}.ws-phase.done .d{background:var(--cyan-400)}
      .chain-grid{display:grid;grid-template-columns:1fr 1fr;gap:8px}
      .chain-grid .btn{justify-content:center}
    </style>
    <div class="imm">
      <div class="imm-bar">
        <span class="imm-back" onclick="Chaimir.navigate('student/experiments')">${C.icon('arrow-left')} 返回实验</span>
        <span class="fw-600 text-sm">代码实验工作台 · PoW 挖矿与 51% 算力攻击</span>
        <div class="inc"><span class="inc-dot ok" id="inc-dot"></span><span id="inc-txt">VM 就绪 · EVM/Hardhat</span></div>
        <div style="flex:1"></div>
        <button class="btn btn-primary btn-sm" onclick="Chaimir.expFn.run()">${C.icon('play')} 运行并构建</button>
      </div>
      <div class="imm-work">
        <div class="imm-side">
          <div class="imm-side-h">资源管理器</div>
          <div class="ftree">${FILES.map(([n, ic, d, on]) => `<div class="fi ${on ? 'on' : ''}" style="padding-left:${d * 10}px">${C.icon(ic)} ${n}</div>`).join('')}</div>
        </div>
        <div class="imm-main">
          <div class="etabs"><span class="etab on">Miner.sol</span><span class="etab">mine.test.js</span></div>
          <div class="editor">${CODE}</div>
          <div class="term" id="exp-term"><div style="color:var(--cyan-400)">$ npx hardhat test</div><div class="muted">自动化虚拟机沙箱环境连通测试成功。点击右上「运行并构建」开始。</div><div>$ _</div></div>
        </div>
        <div class="imm-right">
          <div class="imm-card"><h4>${C.icon('list-checks')} 自动化检查点</h4>
            ${[['沙箱运行时连通性探测', 'pass', '通过'], ['挖矿核心逻辑与 Nonce 判定', '', '待运行'], ['算力分叉与断言审计', '', '待运行']].map((c, i) => `
              <div class="cp" id="exp-cp-${i + 1}"><span class="cp-n">${i + 1}</span>
                <div style="flex:1"><div class="fw-600">检查点 ${i + 1}</div><div class="muted" style="margin-top:2px">${c[0]}</div></div>
                <span class="cp-flag ${c[1]}">${c[2]}</span>
                <button class="btn btn-ghost btn-sm" style="color:var(--cyan-300);padding:2px 6px" onclick="Chaimir.expFn.judge(${i + 1})">判题</button></div>`).join('')}
            <div class="flex justify-between text-xs muted" style="margin-top:4px"><span>已得分</span><span class="fw-700" style="color:var(--cyan-300)">30 / 100</span></div>
          </div>
          <div class="imm-card"><h4>${C.icon('link')} 链上操作</h4>
            <div class="chain-grid">
              <button class="btn btn-on-dark btn-sm" onclick="Chaimir.expFn.chain('deploy')">${C.icon('upload-cloud')} 部署</button>
              <button class="btn btn-on-dark btn-sm" onclick="Chaimir.expFn.chain('tx')">${C.icon('send')} 交易</button>
              <button class="btn btn-on-dark btn-sm" onclick="Chaimir.expFn.chain('query')">${C.icon('search')} 查询</button>
              <button class="btn btn-on-dark btn-sm" onclick="Chaimir.expFn.chain('reset')">${C.icon('rotate-ccw')} 重置</button>
            </div></div>
          <div class="imm-card"><h4>${C.icon('book-open')} 实验说明</h4>
            <p class="text-xs" style="color:var(--color-dark-text-sub);line-height:1.7">实现 Miner 合约的工作量证明:寻找使区块哈希低于难度目标的 nonce。完成后运行检查点验证挖矿逻辑,并尝试模拟 51% 算力分叉观察最长链规则。</p></div>
        </div>
      </div>
      <div class="imm-ws" id="exp-ws">
        ${['分配环境', '环境就绪', '个性化初始化', '完全就绪'].map(p => `<span class="ws-phase"><span class="d"></span>${p}</span>`).join('')}
        <div style="flex:1"></div><span>${C.icon('wifi')} 实时通道已连接</span><span class="autosave saved" style="color:var(--cyan-300)">${C.icon('check')} 代码已自动持久化</span>
      </div>
    </div>`,
  });
})();
