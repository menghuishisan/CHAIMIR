DROP POLICY IF EXISTS paper_item_tenant_rls ON paper_item;
DROP POLICY IF EXISTS paper_tenant_rls ON paper;
DROP POLICY IF EXISTS content_body_tenant_or_shared_rls ON content_body;
DROP POLICY IF EXISTS content_item_tenant_or_shared_rls ON content_item;
DROP POLICY IF EXISTS content_category_tenant_rls ON content_category;

DROP TABLE IF EXISTS paper_item;
DROP TABLE IF EXISTS paper;
DROP TABLE IF EXISTS content_body;
DROP TABLE IF EXISTS content_item;
DROP TABLE IF EXISTS content_category;
