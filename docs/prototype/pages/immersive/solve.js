/* ============================================================
   pages/immersive/solve.js — 解题赛答题页(全屏沉浸)
   ------------------------------------------------------------
   对应 M8 竞赛解题赛。布局:顶(赛事/计时/提交)+ 左(题目列表)
   + 中(题面,取自 M5 已过滤答案)+ 右(作答区:实操起环境 / 理论作答
   + 提交判定)。实操题可跳代码工作台。
   ============================================================ */
(function () {
  const C = window.Chaimir;
  const V = C.solveFn = {};
  const PROBLEMS = [
    { id: 'A', title: '金库重入渗透', type: '实操', score: 300, dyn: 264, solved: false, desc: '目标合约存在重入漏洞。编写攻击合约掏空金库余额,取得 flag 资产证明。允许部署到隔离沙箱链调试。' },
    { id: 'B', title: '整数溢出铸币', type: '实操', score: 250, dyn: 230, solved: true, desc: '利用 ERC20 合约的未检查算术,铸造超额代币并提交链上状态证明。' },
    { id: 'C', title: '共识安全单选', type: '理论', score: 100, dyn: 100, solved: false, desc: 'PBFT 协议在 N=3f+1 下最多可容忍多少个拜占庭节点?' },
  ];
  V._cur = 'A';
  V.pick = (id) => { V._cur = id; C.rerender(); };
  V.submit = function () {
    const p = PROBLEMS.find(x => x.id === V._cur);
    C.toast('info', '已提交', '正在判题机评测…');
    setTimeout(() => {
      const box = document.getElementById('sv-verdict');
      if (box) box.innerHTML = `<div class="callout success">${C.icon('check-circle-2')}<div><b>判定通过 · 得 ${p.dyn} 分</b><div class="muted text-xs" style="margin-top:2px">flag 校验成功,排行榜已更新。动态分随解出人数衰减。</div></div></div>`;
      C.toast('success', '判定通过', `获得 ${p.dyn} 分,天梯排名上升`);
    }, 1200);
  };

  C.registerPages({
    'immersive/solve': () => {
      const p = PROBLEMS.find(x => x.id === V._cur) || PROBLEMS[0];
      return `<style>
        .sv{position:fixed;inset:0;display:flex;flex-direction:column;background:var(--color-bg);z-index:var(--z-immersive)}
        .sv-top{height:52px;background:var(--color-dark-bg);color:var(--color-dark-text);display:flex;align-items:center;gap:14px;padding:0 16px;flex-shrink:0}
        .sv-back{display:flex;align-items:center;gap:6px;color:var(--color-dark-text-sub);font-size:var(--text-sm);cursor:pointer}.sv-back:hover{color:#fff}
        .sv-timer{font-family:var(--font-mono);background:rgba(255,255,255,.06);padding:5px 12px;border-radius:var(--radius-full);font-size:var(--text-sm);color:var(--amber-300)}
        .sv-mid{flex:1;display:flex;overflow:hidden}
        .sv-list{width:220px;background:var(--color-surface);border-right:1px solid var(--color-border);overflow:auto;padding:12px;flex-shrink:0}
        .sv-pi{display:flex;align-items:center;gap:10px;padding:11px;border:1px solid var(--color-border);border-radius:var(--radius-sm);margin-bottom:8px;cursor:pointer;transition:all .15s var(--ease)}
        .sv-pi:hover{border-color:var(--color-primary)}
        .sv-pi.on{border-color:var(--color-primary);background:var(--color-primary-soft)}
        .sv-pi .pid{width:26px;height:26px;border-radius:var(--radius-sm);background:var(--color-surface-sunken);display:grid;place-items:center;font-weight:700;font-size:var(--text-sm);flex-shrink:0}
        .sv-pi.solved .pid{background:var(--green-100);color:var(--green-700)}
        .sv-main{flex:1;overflow:auto;padding:28px 32px;min-width:0}
        .sv-right{width:380px;background:var(--color-surface);border-left:1px solid var(--color-border);overflow:auto;padding:24px;flex-shrink:0}
      </style>
      <div class="sv">
        <div class="sv-top">
          <span class="sv-back" onclick="Chaimir.navigate('student/contests')">${C.icon('arrow-left')} 退出赛场</span>
          <span class="fw-600 text-sm">链上夺旗 · 金库重入渗透赛</span>
          <div style="flex:1"></div>
          <span class="sv-timer">${C.icon('timer')} 剩余 01:24:36</span>
          <span class="role-pill">${C.icon('trophy')} 当前第 7 名</span>
        </div>
        <div class="sv-mid">
          <div class="sv-list">
            <div class="side-group-title">赛题 (${PROBLEMS.filter(x => x.solved).length}/${PROBLEMS.length})</div>
            ${PROBLEMS.map(x => `<div class="sv-pi ${x.id === p.id ? 'on' : ''} ${x.solved ? 'solved' : ''}" onclick="Chaimir.solveFn.pick('${x.id}')">
              <span class="pid">${x.solved ? C.icon('check') : x.id}</span>
              <div style="flex:1;min-width:0"><div class="fw-600 text-sm ellipsis">${x.title}</div>
                <div class="muted text-xs">${x.type} · ${x.dyn} 分</div></div></div>`).join('')}
          </div>
          <div class="sv-main">
            <div class="flex items-center gap-2 mb-3">${C.badge('题 ' + p.id, 'gray')}${C.badge(p.type, p.type === '实操' ? 'purple' : 'blue')}${C.badge('当前 ' + p.dyn + ' 分(动态)', 'amber')}</div>
            <h1 class="page-title mb-3">${p.title}</h1>
            <div class="card card-pad mb-4"><p style="line-height:var(--leading-relaxed)">${p.desc}</p></div>
            ${p.type === '实操' ? `
              <div class="callout info mb-4">${C.icon('info')}<div>本题为实操题:可在隔离沙箱链中部署与调试攻击合约,提交链上状态证明由判题机自动校验。</div></div>
              <div style="background:var(--color-editor-bg);border-radius:var(--radius-sm);padding:16px;font-family:var(--font-mono);font-size:12.5px;color:#cbd5e1;white-space:pre;overflow:auto;line-height:1.7">contract Vault {
    mapping(address => uint) public balances;
    function withdraw() public {
        uint amt = balances[msg.sender];
        (bool ok,) = msg.sender.call{value: amt}("");  <span style="color:#fca5a5">// 重入点</span>
        balances[msg.sender] = 0;
    }
}</div>
              <button class="btn btn-primary mt-3" onclick="Chaimir.navigate('immersive/exp-ide')">${C.icon('code-2')} 进入沙箱编写攻击合约</button>
            ` : `
              <div class="card card-pad">
                <div class="fw-600 mb-3">请选择正确答案</div>
                ${['f 个', '2f 个', '3f 个', 'N/2 个'].map((o, i) => `<label class="radio" style="display:flex;padding:11px;border:1px solid var(--color-border);border-radius:var(--radius-sm);margin-bottom:8px"><input type="radio" name="sv-q"> ${o}</label>`).join('')}
              </div>`}
          </div>
          <div class="sv-right">
            <div class="section-title mb-3">${C.icon('flag')} 提交答案</div>
            ${p.type === '实操'
        ? `<div class="field"><label>flag / 链上状态证明</label><div class="input-icon">${C.icon('key')}<input class="input" placeholder="flag{...} 或交易哈希"></div></div>`
        : `<div class="muted text-sm mb-3">选择上方选项后提交。</div>`}
            <button class="btn btn-primary btn-block" onclick="Chaimir.solveFn.submit()">${C.icon('send')} 提交判定</button>
            <div id="sv-verdict" class="mt-3"></div>
            <div class="divider"></div>
            <div class="section-title mb-2">${C.icon('history')} 提交记录</div>
            <div class="muted text-xs">暂无提交。每次提交将进入判题队列,结果实时回传。</div>
            <div class="divider"></div>
            <div class="callout warn">${C.icon('shield')}<div class="text-xs">赛场启用防作弊:代码查重 + 行为分析。异常提交将人工复核。</div></div>
          </div>
        </div>
      </div>`;
    },
  });
})();
