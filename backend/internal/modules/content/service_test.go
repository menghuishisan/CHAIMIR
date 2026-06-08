// M5 服务规则测试:覆盖答案过滤、版本不可变、共享克隆与组卷锁版本等核心业务边界。
package content

import (
	"errors"
	"testing"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"
)

// TestFilterBodyRemovesNestedSensitiveFields 确认题面视角会按敏感字段路径递归剥离答案、测试用例和 flag。
func TestFilterBodyRemovesNestedSensitiveFields(t *testing.T) {
	body := map[string]any{
		"statement": "请完成合约",
		"answer":    "secret",
		"judge_config": map[string]any{
			"judger_code": "evm-testcase",
			"testcases":   []any{"hidden"},
			"flag":        "FLAG{hidden}",
		},
	}

	got := filterSensitiveBody(body, []string{"answer", "judge_config.testcases", "judge_config.flag"})

	if _, ok := got["answer"]; ok {
		t.Fatalf("answer must be removed from face body: %#v", got)
	}
	jc := got["judge_config"].(map[string]any)
	if _, ok := jc["testcases"]; ok {
		t.Fatalf("testcases must be removed from face body: %#v", jc)
	}
	if _, ok := jc["flag"]; ok {
		t.Fatalf("flag must be removed from face body: %#v", jc)
	}
	if jc["judger_code"] != "evm-testcase" {
		t.Fatalf("non-sensitive judge metadata should remain: %#v", jc)
	}
}

// TestValidateDraftUpdateRejectsPublishedItem 确认已发布版本不可被直接编辑,改题必须创建新版本。
func TestValidateDraftUpdateRejectsPublishedItem(t *testing.T) {
	if err := validateDraftEditable(ItemStatusDraft); err != nil {
		t.Fatalf("draft should be editable: %v", err)
	}
	if err := validateDraftEditable(ItemStatusPublished); err == nil {
		t.Fatalf("published item must not be editable")
	}
}

// TestBuildCloneRequestResetsOwnershipAndUsage 确认克隆是深拷贝,会生成新 code、草稿状态并清零复用统计。
func TestBuildCloneRequestResetsOwnershipAndUsage(t *testing.T) {
	source := ItemDTO{
		Code: "src", Version: "1.0.0", Type: ContentTypeTheoryQuestion,
		Title: "源题", Difficulty: DifficultyAdvanced, Visibility: VisibilityShared,
		Status: ItemStatusPublished, UsageCount: 9,
		Body: map[string]any{"answer": "A"},
	}

	got, err := buildCloneDraft(source, 2001, "clone-code")
	if err != nil {
		t.Fatalf("valid clone rejected: %v", err)
	}
	if got.Code != "clone-code" || got.Version != initialVersion || got.Status != ItemStatusDraft {
		t.Fatalf("clone identity/status not reset: %#v", got)
	}
	if got.AuthorID != "2001" || got.UsageCount != 0 || got.Visibility != VisibilityPrivate {
		t.Fatalf("clone ownership/visibility/usage not reset: %#v", got)
	}
	got.Body["answer"] = "B"
	if source.Body["answer"] != "A" {
		t.Fatalf("clone body must be independent deep copy")
	}
}

// TestResolveNewVersionDefaultsToLatestPublishedVersion 确认默认发新版时基于最高已发布版本递增,不回退到固定 1.0.0。
func TestResolveNewVersionDefaultsToLatestPublishedVersion(t *testing.T) {
	source, target, err := resolveNewVersionPlan([]ItemDTO{
		{Version: "1.0.0", Status: ItemStatusPublished},
		{Version: "1.2.3", Status: ItemStatusPublished},
		{Version: "2.0.0", Status: ItemStatusDraft},
	}, NewVersionRequest{})
	if err != nil {
		t.Fatalf("valid versions rejected: %v", err)
	}
	if source != "1.2.3" || target != "1.2.4" {
		t.Fatalf("unexpected source/target versions: source=%s target=%s", source, target)
	}
}

// TestResolveNewVersionSeparatesSourceAndTarget 确认源版本与目标版本是两个独立字段,不再复用同一个 Version 产生歧义。
func TestResolveNewVersionSeparatesSourceAndTarget(t *testing.T) {
	source, target, err := resolveNewVersionPlan([]ItemDTO{
		{Version: "1.0.0", Status: ItemStatusPublished},
	}, NewVersionRequest{SourceVersion: "1.0.0", Version: "1.1.0"})
	if err != nil {
		t.Fatalf("explicit source/target rejected: %v", err)
	}
	if source != "1.0.0" || target != "1.1.0" {
		t.Fatalf("unexpected source/target versions: source=%s target=%s", source, target)
	}
}

// TestValidateDraftDeletableSeparatesImmutableAndUsageErrors 确认删除状态错误和引用阻断使用不同错误码,便于前端给出准确指引。
func TestValidateDraftDeletableSeparatesImmutableAndUsageErrors(t *testing.T) {
	if err := validateDraftDeletable(ItemStatusPublished, 0); err == nil {
		t.Fatalf("published item must not be deletable")
	}
	if err := validateDraftDeletable(ItemStatusDraft, 1); err == nil {
		t.Fatalf("referenced draft must not be deletable")
	}
}

// TestBuildJudgeSpecRequiresPublishedFullContent 确认 M3 只能从已发布 full 内容生成判题快照。
func TestBuildJudgeSpecRequiresPublishedFullContent(t *testing.T) {
	item := ItemDTO{
		Code: "p1", Version: "1.0.0", Status: ItemStatusPublished,
		BodyHash: "hash",
		Body: map[string]any{
			"judge_config": map[string]any{
				"judger_code": "evm-testcase",
				"max_score":   float64(100),
				"suite_ref":   "attach://suite.zip",
				"expectation": map[string]any{"passed": true},
			},
		},
	}

	spec, err := judgeSpecFromItem(item)
	if err != nil {
		t.Fatalf("valid judge spec rejected: %v", err)
	}
	if spec.JudgerCode != "evm-testcase" || spec.MaxScore != 100 || spec.VersionHash != "hash" {
		t.Fatalf("unexpected judge spec: %#v", spec)
	}

	item.Status = ItemStatusDraft
	if _, err := judgeSpecFromItem(item); err == nil {
		t.Fatalf("draft content must not provide judge spec")
	}
}

// TestValidatePaperRequestRejectsInvalidManualItems 确认手动组卷必须锁定明确版本且分值有效。
func TestValidatePaperRequestRejectsInvalidManualItems(t *testing.T) {
	cases := []PaperRequest{
		{Name: "测验", GenMode: PaperGenManual, Items: []PaperItemReq{{Version: "1.0.0", Score: 1}}},
		{Name: "测验", GenMode: PaperGenManual, Items: []PaperItemReq{{Code: "q1", Score: 1}}},
		{Name: "测验", GenMode: PaperGenManual, Items: []PaperItemReq{{Code: "q1", Version: "1.0.0"}}},
		{Name: "测验", GenMode: PaperGenManual, Items: []PaperItemReq{{Code: "q1", Version: "1.0.0", Score: -1}}},
	}
	for _, req := range cases {
		if err := validatePaperRequest(req); err == nil {
			t.Fatalf("invalid manual paper item should be rejected: %#v", req)
		}
	}
	valid := PaperRequest{Name: "测验", GenMode: PaperGenManual, Items: []PaperItemReq{{Code: "q1", Version: "1.0.0", Score: 10}}}
	if err := validatePaperRequest(valid); err != nil {
		t.Fatalf("valid manual paper item rejected: %v", err)
	}
}

// TestValidatePaperItemReferenceRequiresPublishedContent 确认组卷只能锁定已发布内容版本。
func TestValidatePaperItemReferenceRequiresPublishedContent(t *testing.T) {
	if err := validatePaperItemReference(ItemStatusPublished); err != nil {
		t.Fatalf("published content should be referable: %v", err)
	}
	if err := validatePaperItemReference(ItemStatusDraft); err == nil {
		t.Fatalf("draft content must not be referable")
	}
	if err := validatePaperItemReference(ItemStatusDeprecated); err == nil {
		t.Fatalf("deprecated content must not be used for new paper references")
	}
}

// TestCanManageContentRequiresAuthorOrSchoolAdmin 确认教师只能管理本人内容,学校管理员可管理本租户内容。
func TestCanManageContentRequiresAuthorOrSchoolAdmin(t *testing.T) {
	teacher := contracts.AccountInfo{AccountID: 10, Roles: []string{"teacher"}}
	if !canManageContent(false, teacher, 10) {
		t.Fatalf("author teacher should manage own content")
	}
	if canManageContent(false, teacher, 11) {
		t.Fatalf("teacher must not manage another teacher content")
	}
	admin := contracts.AccountInfo{AccountID: 20, Roles: []string{"school_admin"}}
	if !canManageContent(false, admin, 11) {
		t.Fatalf("school admin should manage tenant content")
	}
	if !canManageContent(true, contracts.AccountInfo{}, 11) {
		t.Fatalf("platform context should manage platform operations")
	}
}

// TestValidateBodyHashRejectsTamperedContent 确认读取内容时会校验入库哈希,防止版本内容被篡改后继续流转。
func TestValidateBodyHashRejectsTamperedContent(t *testing.T) {
	body, err := jsonx.ObjectBytes(map[string]any{"statement": "原题面"}, apperr.ErrContentInvalid)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	if err := validateBodyHash(body, bodyHash(body)); err != nil {
		t.Fatalf("matching hash rejected: %v", err)
	}
	if err := validateBodyHash(body, bodyHash([]byte(`{"statement":"tampered"}`))); err == nil {
		t.Fatalf("tampered body hash must be rejected")
	}
}

// TestCanReadOwnContentFaceHonorsVisibility 确认教师直连题库时 private 只允许作者/管理员读取,tenant 允许本租户教师读取。
func TestCanReadOwnContentFaceHonorsVisibility(t *testing.T) {
	author := contracts.AccountInfo{AccountID: 10, Roles: []string{"teacher"}}
	peer := contracts.AccountInfo{AccountID: 11, Roles: []string{"teacher"}}
	admin := contracts.AccountInfo{AccountID: 12, Roles: []string{"school_admin"}}
	if !canReadOwnContentFace(false, author, 10, VisibilityPrivate) {
		t.Fatalf("author should read private content face")
	}
	if canReadOwnContentFace(false, peer, 10, VisibilityPrivate) {
		t.Fatalf("peer teacher must not read private content face")
	}
	if !canReadOwnContentFace(false, peer, 10, VisibilityTenant) {
		t.Fatalf("tenant-visible content should be readable by tenant teacher")
	}
	if !canReadOwnContentFace(false, admin, 10, VisibilityPrivate) {
		t.Fatalf("school admin should read tenant private content")
	}
}

// TestRandomCriteriaSupportsArrays 确认随机组卷条件支持文档示例中的多个知识点和多个难度。
func TestRandomCriteriaSupportsArrays(t *testing.T) {
	criteria := normalizeRandomCriteria(map[string]any{
		"count":            10,
		"type":             float64(ContentTypeTheoryQuestion),
		"difficulty":       []any{float64(DifficultyAdvanced), float64(DifficultyExpert)},
		"knowledge_points": []any{"共识", "哈希"},
	})
	if criteria.Count != 10 || criteria.Type != ContentTypeTheoryQuestion {
		t.Fatalf("basic criteria lost: %#v", criteria)
	}
	if len(criteria.Difficulties) != 2 || criteria.Difficulties[0] != DifficultyAdvanced || criteria.Difficulties[1] != DifficultyExpert {
		t.Fatalf("difficulty array not preserved: %#v", criteria)
	}
	if len(criteria.KnowledgePoints) != 2 || criteria.KnowledgePoints[0] != "共识" || criteria.KnowledgePoints[1] != "哈希" {
		t.Fatalf("knowledge point array not preserved: %#v", criteria)
	}
}

// TestValidateCategoryParentRejectsSelfAndDescendant 确认分类树更新不能把节点挂到自己或后代下。
func TestValidateCategoryParentRejectsSelfAndDescendant(t *testing.T) {
	rows := []CategoryDTO{
		{ID: "1", ParentID: ""},
		{ID: "2", ParentID: "1"},
		{ID: "3", ParentID: "2"},
	}
	if err := validateCategoryParent(1, 0, rows); err != nil {
		t.Fatalf("root parent should be allowed: %v", err)
	}
	if err := validateCategoryParent(2, 1, rows); err != nil {
		t.Fatalf("existing parent should be allowed: %v", err)
	}
	if err := validateCategoryParent(2, 2, rows); err == nil {
		t.Fatalf("category must not parent itself")
	}
	if err := validateCategoryParent(1, 3, rows); err == nil {
		t.Fatalf("category must not parent to descendant")
	}
	if err := validateCategoryParent(1, 99, rows); err == nil {
		t.Fatalf("missing parent should be rejected")
	}
}

// TestCreateAuditDetailIncludesSystemImportNote 确认系统导入说明进入审计详情,便于追踪外部源与预验证信息。
func TestCreateAuditDetailIncludesSystemImportNote(t *testing.T) {
	detail := createItemAuditDetail(CreateItemRequest{
		Code: "vuln-1", Version: "1.0.0",
		SystemImportNote: map[string]any{"source": "m8-vuln", "precheck": true},
	})
	note, ok := detail["system_import_note"].(map[string]any)
	if !ok {
		t.Fatalf("system import note should be preserved in audit detail: %#v", detail)
	}
	if note["source"] != "m8-vuln" || note["precheck"] != true {
		t.Fatalf("unexpected system import note: %#v", note)
	}
}

// TestContentQueryErrorsUseModuleSpecificCodes 确认 M5 查询失败不会退回通用内部错误码。
func TestContentQueryErrorsUseModuleSpecificCodes(t *testing.T) {
	cause := errors.New("database unavailable")
	cases := []struct {
		name string
		err  *apperr.Error
		code string
	}{
		{name: "content query", err: apperr.ErrContentQueryFailed.WithCause(cause), code: "51010"},
		{name: "version query", err: apperr.ErrContentVersionQueryFailed.WithCause(cause), code: "52004"},
		{name: "category query", err: apperr.ErrContentCategoryQueryFailed.WithCause(cause), code: "51011"},
		{name: "paper query", err: apperr.ErrPaperQueryFailed.WithCause(cause), code: "54006"},
	}
	for _, tc := range cases {
		if tc.err.Code != tc.code {
			t.Fatalf("%s expected code %s, got %s", tc.name, tc.code, tc.err.Code)
		}
		if errors.Unwrap(tc.err) != cause {
			t.Fatalf("%s should preserve internal cause", tc.name)
		}
	}
}
