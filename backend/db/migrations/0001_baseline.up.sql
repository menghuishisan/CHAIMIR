-- 迁移 0001:平台基线(纯 schema:通用辅助 + RLS 约定文档)。
-- 依据 docs/总-数据库表总览.md §1/§4:雪花 ID(应用生成)、TIMESTAMPTZ、RLS 多租户。
-- 注:数据库角色/授权(chaimir_app)不在 migration 内 —— 那是建库初始化,放 scripts/。
--   migration 只管表结构与 RLS 策略(纯 schema)。

-- 触发器函数:自动维护 updated_at(所有带 updated_at 的表统一复用)。
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- RLS 约定(SSOT,各租户表统一套用,杜绝写法不一):
--   1) 表必含 tenant_id BIGINT NOT NULL,并建索引。
--   2) ALTER TABLE <t> ENABLE ROW LEVEL SECURITY;(不 FORCE:
--      应用以非属主 chaimir_app 连接受约束,属主用于迁移/特权跨租户查询)。
--   3) CREATE POLICY tenant_isolation ON <t>
--        USING (tenant_id = current_setting('app.tenant_id')::BIGINT)
--        WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
--   4) 应用每事务 SET LOCAL app.tenant_id(platform/db.WithTenantTx),LOCAL 防连接池串号。
--   5) 私有化单租户:tenant_id 恒定,策略恒真,逻辑零差异。
-- 平台级表(无 tenant_id)不启用 RLS,由应用层控制访问。
