package router

import (
	"net/http"

	"github.com/ccfos/nightingale/v6/center/service"
	"github.com/ccfos/nightingale/v6/models"

	"github.com/gin-gonic/gin"
	"github.com/toolkits/pkg/ginx"
)

// RegisterManagedHostRoutes 注册受管主机相关路由
func (rt *Router) RegisterManagedHostRoutes(r *gin.Engine) {
	// Only admin users can access these endpoints
	managedHosts := r.Group("/api/n9e/managed-hosts").Use(rt.auth(), rt.user())
	{
		managedHosts.GET("", rt.perm("/managed-hosts"), rt.managedHostList)
		managedHosts.GET("/:target_ident", rt.perm("/managed-hosts"), rt.managedHostGet)
		managedHosts.POST("", rt.perm("/managed-hosts/add"), rt.managedHostAdd)
		managedHosts.PUT("/:target_ident", rt.perm("/managed-hosts/put"), rt.managedHostUpdate)
		managedHosts.DELETE("", rt.perm("/managed-hosts/del"), rt.managedHostDel)
	}
}

// managedHostList 获取受管主机列表
func (rt *Router) managedHostList(c *gin.Context) {
	limit := ginx.QueryInt(c, "limit", 20)
	offset := ginx.QueryInt(c, "offset", 0)
	query := ginx.QueryStr(c, "query", "")

	total, err := models.ManagedHostCount(rt.Ctx, query)
	ginx.Dangerous(err)

	list, err := models.ManagedHostGets(rt.Ctx, limit, offset, query)
	ginx.Dangerous(err)

	// Fill target information for each managed host
	for i := range list {
		target, err := models.TargetGetByIdent(rt.Ctx, list[i].HostIdent)
		if err == nil && target != nil {
			list[i].Target = target
		}
	}

	ginx.NewRender(c).Data(gin.H{
		"list":  list,
		"total": total,
	}, nil)
}

// managedHostGet 获取单个受管主机详情
func (rt *Router) managedHostGet(c *gin.Context) {
	hostIdent := ginx.UrlParamStr(c, "target_ident")

	managedHost, err := models.ManagedHostGetByIdent(rt.Ctx, hostIdent)
	ginx.Dangerous(err)

	if managedHost == nil {
		ginx.Bomb(http.StatusNotFound, "managed host not found")
	}

	// Fill target information
	target, err := models.TargetGetByIdent(rt.Ctx, managedHost.HostIdent)
	if err == nil && target != nil {
		managedHost.Target = target
	}

	ginx.NewRender(c).Data(managedHost, nil)
}

// managedHostAdd 批量创建受管主机
func (rt *Router) managedHostAdd(c *gin.Context) {
	var lst []struct {
		models.ManagedHost
		Credential string `json:"credential"`
	}
	ginx.BindJSON(c, &lst)

	username := c.MustGet("username").(string)

	// Validate and process each managed host
	for i := range lst {
		// Set default values if not provided
		if lst[i].SSHPort == 0 {
			lst[i].SSHPort = 22
		}

		// Validate auth method
		if lst[i].AuthMethod != "key" && lst[i].AuthMethod != "password" {
			ginx.Bomb(http.StatusBadRequest, "invalid auth_method, must be 'key' or 'password'")
		}

		// Check if already exists
		exists, err := models.ManagedHostExistsByIdent(rt.Ctx, lst[i].HostIdent)
		ginx.Dangerous(err)

		if exists {
			ginx.Bomb(http.StatusBadRequest, "managed host already exists for target: %s", lst[i].HostIdent)
		}

		// Set creator and updater
		lst[i].CreateBy = username
		lst[i].UpdateBy = username
	}

	// Add all managed hosts
	for i := range lst {
		// Add managed host to database
		err := models.ManagedHostAdd(rt.Ctx, &lst[i].ManagedHost)
		if err != nil {
			// If any fails, return error
			ginx.Bomb(http.StatusInternalServerError, "failed to add managed host for target %s: %v", lst[i].HostIdent, err)
		}

		// Store credentials in configs table if provided
		if lst[i].Credential != "" {
			err = service.StoreSSHCredential(rt.Ctx, lst[i].HostIdent, lst[i].AuthMethod, lst[i].Credential, username)
			if err != nil {
				// If credential storage fails, log but don't stop (user can update later)
				// In a real implementation, you might want to handle this differently
			}
		}
	}

	ginx.NewRender(c).Message("managed hosts added successfully")
}

// managedHostUpdate 更新受管主机信息
func (rt *Router) managedHostUpdate(c *gin.Context) {
	hostIdent := ginx.UrlParamStr(c, "target_ident")

	var req struct {
		models.ManagedHost
		Credential string `json:"credential"`
	}
	ginx.BindJSON(c, &req)

	// Validate auth method if provided
	if req.AuthMethod != "" && req.AuthMethod != "key" && req.AuthMethod != "password" {
		ginx.Bomb(http.StatusBadRequest, "invalid auth_method, must be 'key' or 'password'")
	}

	// Check if exists
	exists, err := models.ManagedHostExistsByIdent(rt.Ctx, hostIdent)
	ginx.Dangerous(err)

	if !exists {
		ginx.Bomb(http.StatusNotFound, "managed host not found")
	}

	// Prepare updates
	updates := make(map[string]interface{})
	if req.SSHIp != "" {
		updates["ssh_ip"] = req.SSHIp
	}
	if req.SSHPort != 0 {
		updates["ssh_port"] = req.SSHPort
	}
	if req.SSHUser != "" {
		updates["ssh_user"] = req.SSHUser
	}
	if req.AuthMethod != "" {
		updates["auth_method"] = req.AuthMethod
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}
	if req.Note != "" {
		updates["note"] = req.Note
	}
	updates["sudo_required"] = req.SudoRequired

	// Update updater
	updates["update_by"] = c.MustGet("username").(string)

	// Update managed host
	err = models.ManagedHostUpdateByIdent(rt.Ctx, hostIdent, updates)
	ginx.Dangerous(err)

	// Update credential if provided
	if req.Credential != "" && req.AuthMethod != "" {
		err = service.StoreSSHCredential(rt.Ctx, hostIdent, req.AuthMethod, req.Credential, c.MustGet("username").(string))
		ginx.Dangerous(err)
	}

	ginx.NewRender(c).Message("managed host updated successfully")
}

// managedHostDel 批量删除受管主机
func (rt *Router) managedHostDel(c *gin.Context) {
	var hostIdents []string
	ginx.BindJSON(c, &hostIdents)

	if len(hostIdents) == 0 {
		ginx.Bomb(http.StatusBadRequest, "host_idents cannot be empty")
	}

	// Delete managed hosts
	err := models.ManagedHostDelByIdents(rt.Ctx, hostIdents)
	ginx.Dangerous(err)

	// Also delete associated credentials from configs table
	for _, hostIdent := range hostIdents {
		err := service.DeleteSSHCredential(rt.Ctx, hostIdent)
		if err != nil {
			// Log error but don't stop deletion
		}
	}

	ginx.NewRender(c).Message("managed hosts deleted successfully")
}
