package router

import (
	"net/http"

	"github.com/ccfos/nightingale/v6/center/service"
	"github.com/ccfos/nightingale/v6/models"

	"github.com/gin-gonic/gin"
	"github.com/toolkits/pkg/ginx"
)

// RegisterAgentVersionRoutes 注册agent版本管理相关路由
func (rt *Router) RegisterAgentVersionRoutes(pages *gin.RouterGroup) {
	agentVersions := pages.Group("/agent-versions").Use(rt.auth(), rt.user())
	{
		// 组件版本管理路由
		agentVersions.GET("/component/:component_id", rt.perm("/components"), rt.agentVersionList)
		agentVersions.POST("/component/:component_id/activate/:version_id", rt.perm("/components/put"), rt.agentVersionActivate)
		agentVersions.GET("/component/:component_id/active", rt.perm("/components"), rt.agentVersionGetActive)

		// 版本管理路由
		agentVersions.POST("", rt.perm("/components/put"), rt.agentVersionAdd)
		agentVersions.PUT("/version/:id", rt.perm("/components/put"), rt.agentVersionUpdate)
		agentVersions.DELETE("/version/:id", rt.perm("/components/del"), rt.agentVersionDelete)
	}

	agentDeployments := pages.Group("/agent-deployments").Use(rt.auth(), rt.user())
	{
		agentDeployments.GET("", rt.perm("/components"), rt.agentDeploymentList)
		agentDeployments.GET("/:id", rt.perm("/components"), rt.agentDeploymentGet)
		agentDeployments.POST("/deploy", rt.perm("/components/put"), rt.agentDeploymentDeploy)
		agentDeployments.GET("/status/:task_id", rt.perm("/components"), rt.agentDeploymentStatus)
	}
}

// agentVersionList 获取组件的版本列表
func (rt *Router) agentVersionList(c *gin.Context) {
	componentID := ginx.UrlParamInt64(c, "component_id")

	versionService := service.NewAgentVersionService(rt.Ctx)
	versions, err := versionService.ListVersions(componentID)
	if err != nil {
		ginx.Bomb(http.StatusInternalServerError, "Failed to get versions: %v", err)
	}

	ginx.NewRender(c).Data(versions, nil)
}

// agentVersionAdd 创建新版本
func (rt *Router) agentVersionAdd(c *gin.Context) {
	var req models.AgentVersion
	ginx.BindJSON(c, &req)

	username := Username(c)
	req.CreateBy = username

	versionService := service.NewAgentVersionService(rt.Ctx)
	err := versionService.CreateVersion(&req)
	if err != nil {
		ginx.Bomb(http.StatusBadRequest, "Failed to create version: %v", err)
	}

	ginx.NewRender(c).Message("Version created successfully")
}

// agentVersionUpdate 更新版本
func (rt *Router) agentVersionUpdate(c *gin.Context) {
	versionID := ginx.UrlParamInt64(c, "id")

	var updates map[string]interface{}
	ginx.BindJSON(c, &updates)

	username := Username(c)
	updates["update_by"] = username

	versionService := service.NewAgentVersionService(rt.Ctx)
	err := versionService.UpdateVersion(versionID, updates)
	if err != nil {
		ginx.Bomb(http.StatusBadRequest, "Failed to update version: %v", err)
	}

	ginx.NewRender(c).Message("Version updated successfully")
}

// agentVersionDelete 删除版本
func (rt *Router) agentVersionDelete(c *gin.Context) {
	versionID := ginx.UrlParamInt64(c, "id")

	versionService := service.NewAgentVersionService(rt.Ctx)
	err := versionService.DeleteVersion(versionID)
	if err != nil {
		ginx.Bomb(http.StatusBadRequest, "Failed to delete version: %v", err)
	}

	ginx.NewRender(c).Message("Version deleted successfully")
}

// agentVersionActivate 激活版本
func (rt *Router) agentVersionActivate(c *gin.Context) {
	componentID := ginx.UrlParamInt64(c, "component_id")
	versionID := ginx.UrlParamInt64(c, "version_id")

	versionService := service.NewAgentVersionService(rt.Ctx)
	err := versionService.ActivateVersion(componentID, versionID)
	if err != nil {
		ginx.Bomb(http.StatusBadRequest, "Failed to activate version: %v", err)
	}

	ginx.NewRender(c).Message("Version activated successfully")
}

// agentVersionGetActive 获取活跃版本
func (rt *Router) agentVersionGetActive(c *gin.Context) {
	componentID := ginx.UrlParamInt64(c, "component_id")

	versionService := service.NewAgentVersionService(rt.Ctx)
	version, err := versionService.GetActiveVersion(componentID)
	if err != nil {
		ginx.Bomb(http.StatusInternalServerError, "Failed to get active version: %v", err)
	}

	ginx.NewRender(c).Data(version, nil)
}

// agentDeploymentList 获取部署列表
func (rt *Router) agentDeploymentList(c *gin.Context) {
	// 这里需要实现分页查询，暂时返回空列表
	ginx.NewRender(c).Data(gin.H{
		"list":  []interface{}{},
		"total": 0,
	}, nil)
}

// agentDeploymentGet 获取单个部署详情
func (rt *Router) agentDeploymentGet(c *gin.Context) {
	deploymentID := ginx.UrlParamInt64(c, "id")

	deployment, err := models.AgentDeploymentGet(rt.Ctx, deploymentID)
	if err != nil {
		ginx.Bomb(http.StatusInternalServerError, "Failed to get deployment: %v", err)
	}

	if deployment == nil {
		ginx.Bomb(http.StatusNotFound, "deployment not found")
	}

	// Fill related information
	host, err := models.ManagedHostGet(rt.Ctx, deployment.HostID)
	if err == nil && host != nil {
		deployment.Host = host
	}

	component, err := models.BuiltinComponentGet(rt.Ctx, "id = ?", deployment.ComponentID)
	if err == nil && component != nil {
		deployment.Component = component
	}

	version, err := models.AgentVersionGet(rt.Ctx, deployment.VersionID)
	if err == nil && version != nil {
		deployment.Version = version
	}

	ginx.NewRender(c).Data(deployment, nil)
}

// agentDeploymentDeploy 执行部署
func (rt *Router) agentDeploymentDeploy(c *gin.Context) {
	var req service.AgentDeployRequest
	ginx.BindJSON(c, &req)

	username := Username(c)
	req.DeployBy = username

	deployService := service.NewAgentDeployService(rt.Ctx)
	result, err := deployService.DeployAgents(&req)
	if err != nil {
		ginx.Bomb(http.StatusBadRequest, "Failed to deploy agents: %v", err)
	}

	ginx.NewRender(c).Data(result, nil)
}

// agentDeploymentStatus 获取部署状态
func (rt *Router) agentDeploymentStatus(c *gin.Context) {
	taskID := ginx.UrlParamStr(c, "task_id")

	// 这里需要实现任务状态查询，暂时返回模拟数据
	ginx.NewRender(c).Data(gin.H{
		"task_id":   taskID,
		"status":    "running",
		"progress":  50,
		"message":   "Deployment in progress",
		"results":   map[string]interface{}{},
		"create_at": 0,
		"update_at": 0,
	}, nil)
}
