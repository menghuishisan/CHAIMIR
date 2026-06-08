-- 建库初始化:应用运行角色 chaimir_app(安全基石)。
-- 依据 docs/01-身份与租户/02-数据模型.md §8 + CLAUDE.md §7。
--
-- 为什么放 scripts/ 而非 migration:
--   migration 只管表结构 + RLS 策略(纯 schema);数据库【角色与授权】属于建库初始化,
--   是部署期一次性动作,不随表结构演进,故归 scripts/(docs/总-工程目录设计 §7)。
--
-- 关键安全前提:PostgreSQL 表属主/超级用户【绕过】RLS。故:
--   · 应用常规请求必须以非属主、无 BYPASSRLS 的 chaimir_app 连接 → RLS 才生效;
--   · 迁移/种子、以及登录前受控跨租户查询(一号多校/Refresh 定位)走属主特权连接。
--
-- 用法(部署期,以数据库属主/超级用户执行一次):
--   CHAIMIR_APP_PASSWORD="<从 Secret 注入的应用角色口令>" psql -f scripts/db/00_role.sql
-- 口令经环境变量注入,不硬编码进脚本,也不进入 psql 进程参数(CLAUDE.md §6)。

-- 创建/更新应用角色(可登录),口令由部署期环境变量提供。
-- 第一步:从 psql 环境读取口令到本进程变量,避免命令行参数泄漏。
\getenv app_password CHAIMIR_APP_PASSWORD
\if :{?app_password}
\else
  \echo '缺少 CHAIMIR_APP_PASSWORD,无法设置 chaimir_app 角色口令。'
  \quit 1
\endif

-- 第二步:判断角色是否存在,用 psql SQL 字面量引用语法安全传入口令。
SELECT NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'chaimir_app') AS need_create \gset

\if :need_create
  CREATE ROLE chaimir_app LOGIN PASSWORD :'app_password';
\else
  ALTER ROLE chaimir_app LOGIN PASSWORD :'app_password';
\endif

-- 库连接权。
GRANT CONNECT ON DATABASE :"db_name" TO chaimir_app;
GRANT USAGE ON SCHEMA public TO chaimir_app;

-- 对现有表的 DML(受 RLS 约束;migration 已建的表在此授权)。
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO chaimir_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO chaimir_app;

-- 未来由属主新建的表/序列自动授予(各模块迁移建表后无需重复 GRANT)。
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO chaimir_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT USAGE, SELECT ON SEQUENCES TO chaimir_app;
