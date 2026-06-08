/* ============================================================
   pages/auth/auth.js — 登录前页面(全屏,无外壳)
   ------------------------------------------------------------
   覆盖:统一登录(手机号/学号/短信智能识别 + 选学校)、找回密码、
        学校 SSO(CAS/LDAP)、学校入驻申请、激活账号、平台管理员登录、
        强制改密。对应 M1 身份与租户。
   交互:智能识别输入类型、密码/验证码切换、短信倒计时、选校浮层。
   ============================================================ */
(function () {
  const C = window.Chaimir;
  const A = C.authFn = {};

  /* 公共:左侧品牌 Hero + 右侧表单 的外壳 */
  function authShell(formHtml, opts = {}) {
    return `<style>
      .auth-wrap{display:flex;min-height:100vh;background:var(--color-bg)}
      .auth-hero{flex:1.1;background:radial-gradient(120% 120% at 0% 0%, #16223b, var(--slate-900) 60%);position:relative;overflow:hidden;display:flex;flex-direction:column;justify-content:space-between;padding:56px}
      .auth-hero .nodes{position:absolute;inset:0;opacity:.5}
      .auth-hero .nd{position:absolute;width:10px;height:10px;border-radius:50%;background:var(--amber-500);box-shadow:0 0 16px var(--amber-500);animation:authpulse 2.8s infinite}
      @keyframes authpulse{0%,100%{opacity:.4;transform:scale(1)}50%{opacity:1;transform:scale(1.3)}}
      .auth-hero .h-brand{font-size:40px;font-weight:800;color:#fff;letter-spacing:-.02em;position:relative}
      .auth-hero .h-tag{font-size:22px;color:var(--amber-400);font-weight:700;letter-spacing:.1em;margin-top:14px}
      .auth-hero .h-desc{color:var(--color-dark-text-sub);margin-top:16px;font-size:var(--text-base);max-width:380px;line-height:var(--leading-relaxed)}
      .auth-hero .h-foot{position:relative;color:var(--slate-500);font-size:var(--text-xs)}
      .auth-form-side{width:480px;display:flex;align-items:center;justify-content:center;padding:40px;background:var(--color-surface)}
      .auth-box{width:100%;max-width:348px}
      .auth-box h2{font-size:var(--text-3xl);font-weight:700;margin-bottom:6px}
      .auth-tabs{display:flex;gap:4px;background:var(--color-surface-sunken);padding:4px;border-radius:var(--radius-sm);margin-bottom:20px}
      .auth-tab{flex:1;text-align:center;padding:7px;font-size:var(--text-sm);font-weight:600;color:var(--color-text-sub);border-radius:var(--radius-xs);cursor:pointer}
      .auth-tab.active{background:var(--color-surface);color:var(--color-primary-text);box-shadow:var(--shadow-xs)}
      .auth-div{display:flex;align-items:center;gap:12px;color:var(--color-text-faint);font-size:var(--text-xs);margin:18px 0}
      .auth-div::before,.auth-div::after{content:'';flex:1;height:1px;background:var(--color-border)}
      .auth-back{position:relative;display:inline-flex;align-items:center;gap:6px;color:var(--color-dark-text-sub);font-size:var(--text-sm);cursor:pointer}
      .auth-back:hover{color:#fff}
      @media(max-width:860px){.auth-hero{display:none}.auth-form-side{width:100%}}
    </style>
    <div class="auth-wrap">
      <div class="auth-hero">
        <div class="nodes">${[[14,20],[40,14],[66,24],[80,52],[58,70],[28,60],[12,46]].map((p, i) => `<span class="nd" style="left:${p[0]}%;top:${p[1]}%;animation-delay:${i * .35}s"></span>`).join('')}</div>
        <div class="auth-back" onclick="Chaimir.navigate('')">${C.icon('arrow-left')} 返回原型导航</div>
        <div><div class="h-brand">Chaimir</div><div class="h-tag">构建 · 验证 · 对抗</div>
          <p class="h-desc">${opts.heroDesc || '在真实链环境中学习区块链 —— 写下第一行合约,发起一次共识攻击,赢得一场对抗赛。教学、实验、竞赛共享同一套沙箱·评测·题库底座。'}</p></div>
        <div class="h-foot">教学 · 实验 · 竞赛 三位一体 · 多租户 SaaS 与私有化双形态</div>
      </div>
      <div class="auth-form-side"><div class="auth-box">${formHtml}</div></div>
    </div>`;
  }

  /* ---------- 统一登录 ---------- */
  A.switchTab = (mode) => { A._mode = mode; C.rerender(); };
  A.detect = (v) => {
    const tip = document.getElementById('login-tip');
    const school = document.getElementById('school-field');
    v = v.trim();
    if (/^1[3-9]\d{0,9}$/.test(v)) {
      tip.textContent = v.length === 11 ? '已识别为手机号登录' : '继续输入,系统将自动识别手机号或学号';
      tip.style.color = v.length === 11 ? 'var(--color-success)' : 'var(--color-text-sub)';
      school.hidden = true;
    } else if (v.length >= 2 && !/^\d/.test(v)) {
      tip.textContent = '已识别为学号,请选择所属学校核验学籍';
      tip.style.color = 'var(--color-primary-text)';
      school.hidden = false;
    } else { tip.textContent = '输入手机号或学号,系统自动识别登录方式'; tip.style.color = 'var(--color-text-sub)'; school.hidden = true; }
  };
  A.sendSms = (btn) => {
    if (btn.dataset.counting) return;
    let n = 60; btn.dataset.counting = '1'; btn.disabled = true;
    C.toast('success', '验证码已发送', '请查收短信(原型演示验证码:1234)');
    const t = setInterval(() => { btn.textContent = `${n}s 后重发`; if (--n < 0) { clearInterval(t); btn.textContent = '获取验证码'; btn.disabled = false; delete btn.dataset.counting; } }, 1000);
  };
  A.doLogin = () => { C.toast('success', '登录成功', '正在进入学生端…'); setTimeout(() => C.navigate('student/courses'), 700); };

  function loginForm() {
    const mode = A._mode || 'pwd';
    return `
      <h2>欢迎回来</h2>
      <p class="muted mb-4">登录 Chaimir 继续学习</p>
      <div class="auth-tabs">
        <div class="auth-tab ${mode === 'pwd' ? 'active' : ''}" onclick="Chaimir.authFn.switchTab('pwd')">密码登录</div>
        <div class="auth-tab ${mode === 'sms' ? 'active' : ''}" onclick="Chaimir.authFn.switchTab('sms')">短信验证码</div>
      </div>
      <div class="field"><label for="acct">账号凭证</label>
        <div class="input-icon">${C.icon('user')}<input class="input" id="acct" placeholder="手机号 / 学号" oninput="Chaimir.authFn.detect(this.value)"></div>
        <div class="help" id="login-tip">输入手机号或学号,系统自动识别登录方式</div>
      </div>
      <div class="field" id="school-field" hidden><label for="school">所属学校(学籍核验)</label>
        <select class="select" id="school"><option>示例大学</option><option>滨海理工大学</option><option>云岭师范学院</option></select>
      </div>
      ${mode === 'pwd' ? `
        <div class="field"><label for="pwd">密码</label>
          <div class="input-icon">${C.icon('lock')}<input class="input" id="pwd" type="password" placeholder="请输入密码"></div></div>
        <div class="flex items-center justify-between mb-4 text-sm">
          <label class="checkbox"><input type="checkbox"> 记住登录</label>
          <a style="color:var(--color-primary-text);cursor:pointer" onclick="Chaimir.navigate('auth/forgot')">忘记密码</a></div>
      ` : `
        <div class="field"><label for="code">短信验证码</label>
          <div class="flex gap-2"><div class="input-icon" style="flex:1">${C.icon('shield-check')}<input class="input" id="code" placeholder="6 位验证码"></div>
          <button class="btn btn-outline" onclick="Chaimir.authFn.sendSms(this)">获取验证码</button></div></div>
      `}
      <button class="btn btn-primary btn-lg btn-block" onclick="Chaimir.authFn.doLogin()">登 录</button>
      <div class="auth-div">或</div>
      <button class="btn btn-outline btn-block" onclick="Chaimir.navigate('auth/sso')">${C.icon('building-2')} 学校统一认证登录(SSO)</button>
      <div style="text-align:center;margin-top:24px;padding-top:18px;border-top:1px solid var(--color-border)" class="text-sm">
        <span class="muted">还没有学校账号?</span>
        <a style="color:var(--color-primary-text);font-weight:600;cursor:pointer" onclick="Chaimir.navigate('auth/apply')">学校入驻申请 →</a>
      </div>`;
  }

  /* ---------- 找回密码 ---------- */
  function forgotForm() {
    return `
      <div class="auth-back" style="color:var(--color-text-sub);margin-bottom:20px" onclick="Chaimir.navigate('auth/login')">${C.icon('arrow-left')} 返回登录</div>
      <h2>找回密码</h2>
      <p class="muted mb-4">通过手机号验证后重置密码</p>
      <div class="field"><label>手机号</label><div class="input-icon">${C.icon('smartphone')}<input class="input" placeholder="注册手机号"></div></div>
      <div class="field"><label>短信验证码</label><div class="flex gap-2"><div class="input-icon" style="flex:1">${C.icon('shield-check')}<input class="input" placeholder="6 位验证码"></div>
        <button class="btn btn-outline" onclick="Chaimir.authFn.sendSms(this)">获取验证码</button></div></div>
      <div class="field"><label>新密码</label><div class="input-icon">${C.icon('lock')}<input class="input" type="password" placeholder="至少 8 位,含字母与数字"></div>
        <div class="help">密码强度:建议字母 + 数字 + 符号组合</div></div>
      <button class="btn btn-primary btn-lg btn-block" onclick="Chaimir.toast('success','密码已重置','请使用新密码登录');setTimeout(()=>Chaimir.navigate('auth/login'),800)">重置密码</button>`;
  }

  /* ---------- SSO ---------- */
  A.ssoMode = (m) => { A._sso = m; C.rerender(); };
  function ssoForm() {
    const m = A._sso || 'cas';
    return `
      <div class="auth-back" style="color:var(--color-text-sub);margin-bottom:20px" onclick="Chaimir.navigate('auth/login')">${C.icon('arrow-left')} 返回登录</div>
      <h2>学校统一认证</h2>
      <p class="muted mb-4">使用学校账号(CAS / LDAP)登录</p>
      <div class="auth-tabs"><div class="auth-tab ${m === 'cas' ? 'active' : ''}" onclick="Chaimir.authFn.ssoMode('cas')">CAS 跳转</div>
        <div class="auth-tab ${m === 'ldap' ? 'active' : ''}" onclick="Chaimir.authFn.ssoMode('ldap')">LDAP 直连</div></div>
      <div class="field"><label>学校</label><select class="select"><option>示例大学(demo-univ)</option><option>滨海理工大学(bhit)</option></select></div>
      ${m === 'cas' ? `
        <div class="callout info mb-4">${C.icon('info')}<div>将跳转到学校统一身份认证页完成登录;系统仅核验名单内账号,未导入名单的账号无法放行。</div></div>
        <button class="btn btn-primary btn-lg btn-block" onclick="Chaimir.toast('info','正在跳转','原型演示:将跳转学校 CAS 认证页')">${C.icon('external-link')} 前往学校认证</button>
      ` : `
        <div class="field"><label>用户名</label><div class="input-icon">${C.icon('user')}<input class="input" placeholder="学校账号"></div></div>
        <div class="field"><label>密码</label><div class="input-icon">${C.icon('lock')}<input class="input" type="password" placeholder="学校密码"></div></div>
        <button class="btn btn-primary btn-lg btn-block" onclick="Chaimir.authFn.doLogin()">登 录</button>
      `}`;
  }

  /* ---------- 学校入驻申请 ---------- */
  function applyForm() {
    return `
      <div class="auth-back" style="color:var(--color-text-sub);margin-bottom:20px" onclick="Chaimir.navigate('auth/login')">${C.icon('arrow-left')} 返回登录</div>
      <h2>学校入驻申请</h2>
      <p class="muted mb-4">提交后由平台审核,通过将向联系人发送开通激活码</p>
      <div class="field"><label>学校名称<span class="req">*</span></label><input class="input" placeholder="如:江南科技大学"></div>
      <div class="field"><label>学校类型<span class="req">*</span></label><select class="select"><option>本科院校</option><option>高职高专</option><option>科研机构</option></select></div>
      <div class="field"><label>联系人<span class="req">*</span></label><input class="input" placeholder="姓名 / 部门"></div>
      <div class="field"><label>联系电话<span class="req">*</span></label><input class="input" placeholder="手机号"></div>
      <div class="field"><label>联系邮箱<span class="req">*</span></label><input class="input" placeholder="用于接收审核结果"></div>
      <button class="btn btn-primary btn-lg btn-block" onclick="Chaimir.toast('success','申请已提交','我们已受理,审核通过后将联系您');setTimeout(()=>Chaimir.navigate('auth/login'),1000)">提交申请</button>
      <p class="muted text-xs mt-3" style="text-align:center">本平台禁止自助注册,师生账号由学校管理员统一导入</p>`;
  }

  /* ---------- 激活账号 ---------- */
  function activateForm() {
    return `
      <div class="auth-back" style="color:var(--color-text-sub);margin-bottom:20px" onclick="Chaimir.navigate('auth/login')">${C.icon('arrow-left')} 返回登录</div>
      <h2>激活账号</h2>
      <p class="muted mb-4">输入管理员发放的激活码并设置密码</p>
      <div class="field"><label>激活码<span class="req">*</span></label><div class="input-icon">${C.icon('ticket')}<input class="input" placeholder="一次性激活码"></div></div>
      <div class="field"><label>设置密码<span class="req">*</span></label><div class="input-icon">${C.icon('lock')}<input class="input" type="password" placeholder="至少 8 位,含字母与数字"></div></div>
      <div class="field"><label>确认密码<span class="req">*</span></label><div class="input-icon">${C.icon('lock')}<input class="input" type="password" placeholder="再次输入"></div></div>
      <button class="btn btn-primary btn-lg btn-block" onclick="Chaimir.toast('success','账号已激活','请使用新密码登录');setTimeout(()=>Chaimir.navigate('auth/login'),800)">激活并登录</button>`;
  }

  /* ---------- 平台管理员登录 ---------- */
  function platformLoginForm() {
    return `
      <div class="auth-back" style="color:var(--color-text-sub);margin-bottom:20px" onclick="Chaimir.navigate('auth/login')">${C.icon('arrow-left')} 返回</div>
      <div style="width:44px;height:44px;border-radius:11px;background:var(--purple-100);color:var(--purple-700);display:grid;place-items:center;margin-bottom:16px">${C.icon('shield')}</div>
      <h2>平台管理员</h2>
      <p class="muted mb-4">独立入口 · 私有化部署下关闭</p>
      <div class="field"><label>用户名</label><div class="input-icon">${C.icon('user')}<input class="input" placeholder="平台管理员账号"></div></div>
      <div class="field"><label>密码</label><div class="input-icon">${C.icon('lock')}<input class="input" type="password" placeholder="密码"></div></div>
      <button class="btn btn-primary btn-lg btn-block" onclick="Chaimir.toast('success','登录成功','正在进入平台管理端…');setTimeout(()=>Chaimir.navigate('platform-admin/tenants'),700)">登 录</button>
      <div class="callout warn mt-4">${C.icon('alert-triangle')}<div>平台管理员仅管理租户与平台级资源,不进入任何学校的业务数据。</div></div>`;
  }

  /* ---------- 强制改密(登录后中间态)---------- */
  function changePwdForm() {
    return `
      <div style="width:44px;height:44px;border-radius:11px;background:var(--amber-100);color:var(--amber-800);display:grid;place-items:center;margin-bottom:16px">${C.icon('key-round')}</div>
      <h2>请修改初始密码</h2>
      <p class="muted mb-4">首次登录需设置新密码后才能进入系统</p>
      <div class="field"><label>初始密码</label><div class="input-icon">${C.icon('lock')}<input class="input" type="password" placeholder="管理员发放的初始密码"></div></div>
      <div class="field"><label>新密码</label><div class="input-icon">${C.icon('lock')}<input class="input" type="password" placeholder="至少 8 位,含字母与数字"></div></div>
      <div class="field"><label>确认新密码</label><div class="input-icon">${C.icon('lock')}<input class="input" type="password" placeholder="再次输入"></div></div>
      <button class="btn btn-primary btn-lg btn-block" onclick="Chaimir.toast('success','密码已更新','正在进入系统…');setTimeout(()=>Chaimir.navigate('student/courses'),700)">提交并进入</button>`;
  }

  C.registerPages({
    'auth/login': () => authShell(loginForm()),
    'auth/forgot': () => authShell(forgotForm()),
    'auth/sso': () => authShell(ssoForm()),
    'auth/apply': () => authShell(applyForm(), { heroDesc: '欢迎贵校加入 Chaimir。入驻后即可为师生开通教学、实验与竞赛全流程能力,支持 SaaS 托管或私有化部署。' }),
    'auth/activate': () => authShell(activateForm()),
    'auth/platform-login': () => authShell(platformLoginForm(), { heroDesc: '平台运营控制台:审核学校入驻、管理租户与运行时/判题器/仿真包等平台级资源。' }),
    'auth/change-pwd': () => authShell(changePwdForm()),
  });
})();
