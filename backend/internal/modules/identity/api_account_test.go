// identity api_account_test 文件校验账号管理 HTTP 参数绑定遵守接口文档。
package identity

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestBindAccountQueryParsesClassID 验证账号列表支持按班级过滤。
func TestBindAccountQueryParsesClassID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest(http.MethodGet, "/accounts?class_id=123&page=2&size=10", nil)
	ctx.Request = req

	query, ok := bindAccountQuery(ctx)
	if !ok {
		t.Fatalf("期望 class_id 查询参数绑定成功")
	}
	if query.ClassID != 123 {
		t.Fatalf("class_id 未写入账号查询条件: %d", query.ClassID)
	}
	if query.Page != 2 || query.Size != 10 {
		t.Fatalf("分页参数不应受 class_id 影响: page=%d size=%d", query.Page, query.Size)
	}
}
