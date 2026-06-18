DROP POLICY IF EXISTS paper_item_tenant_rls ON paper_item;
DROP POLICY IF EXISTS paper_tenant_rls ON paper;
DROP POLICY IF EXISTS content_usage_ref_tenant_rls ON content_usage_ref;
DROP POLICY IF EXISTS content_body_delete_tenant_rls ON content_body;
DROP POLICY IF EXISTS content_body_update_tenant_rls ON content_body;
DROP POLICY IF EXISTS content_body_insert_tenant_rls ON content_body;
DROP POLICY IF EXISTS content_body_select_tenant_or_shared_rls ON content_body;
DROP POLICY IF EXISTS content_item_delete_tenant_rls ON content_item;
DROP POLICY IF EXISTS content_item_update_tenant_rls ON content_item;
DROP POLICY IF EXISTS content_item_insert_tenant_rls ON content_item;
DROP POLICY IF EXISTS content_item_select_tenant_or_shared_rls ON content_item;
DROP POLICY IF EXISTS content_category_tenant_rls ON content_category;

DROP TABLE IF EXISTS paper_item;
DROP TABLE IF EXISTS paper;
DROP TABLE IF EXISTS content_usage_ref;
DROP TABLE IF EXISTS content_body;
DROP TABLE IF EXISTS content_item;
DROP TABLE IF EXISTS content_category;
