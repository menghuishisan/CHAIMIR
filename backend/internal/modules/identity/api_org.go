// M1 组织架构 HTTP 处理器。
package identity

import (
	"chaimir/internal/platform/httpx"
	"chaimir/internal/platform/ids"
	"chaimir/pkg/apperr"
	"chaimir/pkg/response"

	"github.com/gin-gonic/gin"
)

// listDepartments 列院系。
func (a *API) listDepartments(c *gin.Context) {
	rows, err := a.svc.ListDepartments(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, rows)
}

// createDepartment 建院系。
func (a *API) createDepartment(c *gin.Context) {
	var req CreateDepartmentRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrDepartmentCreateInvalid) {
		return
	}
	node, err := a.svc.CreateDepartment(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, node)
}

// updateDepartment 改院系。
func (a *API) updateDepartment(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req UpdateDepartmentRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrDepartmentUpdateInvalid) {
		return
	}
	if err := a.svc.UpdateDepartment(c.Request.Context(), id, req); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"updated": true})
}

// deleteDepartment 删院系。
func (a *API) deleteDepartment(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	if err := a.svc.DeleteDepartment(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

// listMajors 按院系列专业(?department_id=)。
func (a *API) listMajors(c *gin.Context) {
	deptID, ok := ids.Parse(c.Query("department_id"))
	if !ok {
		response.Fail(c, apperr.ErrOrgParentIDInvalid)
		return
	}
	rows, err := a.svc.ListMajorsByDepartment(c.Request.Context(), deptID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, rows)
}

// createMajor 建专业。
func (a *API) createMajor(c *gin.Context) {
	var req CreateMajorRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrMajorCreateInvalid) {
		return
	}
	node, err := a.svc.CreateMajor(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, node)
}

// updateMajor 改专业名称。
func (a *API) updateMajor(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req UpdateMajorRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrMajorUpdateInvalid) {
		return
	}
	if err := a.svc.UpdateMajor(c.Request.Context(), id, req); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"updated": true})
}

// deleteMajor 删专业。
func (a *API) deleteMajor(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	if err := a.svc.DeleteMajor(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

// listClasses 按专业列班级(?major_id=)。
func (a *API) listClasses(c *gin.Context) {
	majorID, ok := ids.Parse(c.Query("major_id"))
	if !ok {
		response.Fail(c, apperr.ErrOrgParentIDInvalid)
		return
	}
	rows, err := a.svc.ListClassesByMajor(c.Request.Context(), majorID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, rows)
}

// createClass 建班级。
func (a *API) createClass(c *gin.Context) {
	var req CreateClassRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrClassCreateInvalid) {
		return
	}
	node, err := a.svc.CreateClass(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, node)
}

// updateClass 改班级名称与入学年份。
func (a *API) updateClass(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req UpdateClassRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrClassUpdateInvalid) {
		return
	}
	if err := a.svc.UpdateClass(c.Request.Context(), id, req); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"updated": true})
}

// deleteClass 删班级。
func (a *API) deleteClass(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	if err := a.svc.DeleteClass(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

// importOrg 批量导入组织结构。
func (a *API) importOrg(c *gin.Context) {
	id, ok := currentID(c)
	if !ok {
		response.Fail(c, apperr.ErrUnauthorized)
		return
	}
	var req OrgImportRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrOrgImportRequestInvalid) {
		return
	}
	res, err := a.svc.ImportOrg(c.Request.Context(), id.AccountID, req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// batchArchiveClasses 批量归档班级并级联归档学生账号。
func (a *API) batchArchiveClasses(c *gin.Context) {
	var req BatchClassArchiveRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrBatchClassIDsInvalid) {
		return
	}
	res, err := a.svc.BatchArchiveClasses(c.Request.Context(), req.ClassIDs)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// batchPromoteClasses 批量升级班级。
func (a *API) batchPromoteClasses(c *gin.Context) {
	var req BatchClassPromoteRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrBatchClassPromoteInvalid) {
		return
	}
	res, err := a.svc.BatchPromoteClasses(c.Request.Context(), req.Rows)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}
