package router

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/ccfos/nightingale/v6/center/service"
	"github.com/ccfos/nightingale/v6/pkg/ctx"

	"github.com/gin-gonic/gin"
	"github.com/toolkits/pkg/ginx"
	"github.com/toolkits/pkg/logger"
)

// deployAgents 部署Agents
func (rt *Router) deployAgents(deploymentManager *service.DeploymentManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req service.DeployRequest
		ginx.BindJSON(c, &req)

		// Validate request
		if len(req.TargetIdents) == 0 {
			ginx.Bomb(http.StatusBadRequest, "target_idents cannot be empty")
		}

		if req.PlaybookName == "" {
			ginx.Bomb(http.StatusBadRequest, "playbook_name cannot be empty")
		}

		// Check if playbook exists
		playbookPath := filepath.Join(deploymentManager.PlaybooksPath(), req.PlaybookName)
		if !fileIsExist(playbookPath) {
			ginx.Bomb(http.StatusBadRequest, "playbook not found: %s", req.PlaybookName)
		}

		// Trigger deployment
		taskID, err := deploymentManager.DeployAgents(&req)
		if err != nil {
			ginx.Bomb(http.StatusInternalServerError, "failed to start deployment: %v", err)
		}

		ginx.NewRender(c).Data(gin.H{
			"task_id": taskID,
		}, nil)
	}
}

// getDeploymentStatus 获取部署状态
func (rt *Router) getDeploymentStatus(deploymentManager *service.DeploymentManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := ginx.UrlParamStr(c, "task_id")

		result, err := deploymentManager.GetDeploymentResult(taskID)
		if err != nil {
			ginx.Bomb(http.StatusNotFound, "deployment not found: %v", err)
		}

		ginx.NewRender(c).Data(result, nil)
	}
}

// listDeployments 列出所有部署
func (rt *Router) listDeployments(deploymentManager *service.DeploymentManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		results := deploymentManager.ListDeployments()
		ginx.NewRender(c).Data(results, nil)
	}
}

// cancelDeployment 取消部署
func (rt *Router) cancelDeployment(deploymentManager *service.DeploymentManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := ginx.UrlParamStr(c, "task_id")

		err := deploymentManager.CancelDeployment(taskID)
		if err != nil {
			ginx.Bomb(http.StatusNotFound, "failed to cancel deployment: %v", err)
		}

		ginx.NewRender(c).Message("deployment cancelled successfully")
	}
}

// InitializeDeploymentManager 初始化部署管理器
func InitializeDeploymentManager(ctx *ctx.Context, config *DeploymentConfig) *service.DeploymentManager {
	// Create deployment manager
	deploymentManager := service.NewDeploymentManager(
		ctx,
		config.AnsiblePath,
		config.InventoryPath,
		config.PlaybooksPath,
	)

	// Log initialization
	logger.Infof("Deployment manager initialized with ansible path: %s, inventory path: %s, playbooks path: %s",
		config.AnsiblePath, config.InventoryPath, config.PlaybooksPath)

	return deploymentManager
}

// DeploymentConfig 部署配置
type DeploymentConfig struct {
	AnsiblePath   string `json:"ansible_path"`
	InventoryPath string `json:"inventory_path"`
	PlaybooksPath string `json:"playbooks_path"`
}

// fileIsExist is a helper function to check if a file exists
func fileIsExist(path string) bool {
	_, err := os.Stat(path)
	return err == nil || os.IsExist(err)
}

// RegisterDeployRoutes 注册部署相关路由
func (rt *Router) RegisterDeployRoutes(r *gin.Engine, deploymentManager *service.DeploymentManager) {
	// Only admin users can access these endpoints
	deploy := r.Group("/api/n9e/deploy").Use(rt.auth())
	{
		deploy.POST("/agents", rt.deployAgents(deploymentManager))
		deploy.GET("/status/:task_id", rt.getDeploymentStatus(deploymentManager))
		deploy.GET("/list", rt.listDeployments(deploymentManager))
		deploy.POST("/cancel/:task_id", rt.cancelDeployment(deploymentManager))
	}
}