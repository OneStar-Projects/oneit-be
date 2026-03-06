package router

import (
	"github.com/ccfos/nightingale/v6/center/service"

	"github.com/gin-gonic/gin"
	"github.com/toolkits/pkg/ginx"
)

// RegisterSSHTargetRoutes 注册SSH测试相关路由
func (rt *Router) RegisterSSHTargetRoutes(r *gin.Engine) {
	// Only admin users can access these endpoints
	sshTargets := r.Group("/api/n9e/ssh-targets").Use(rt.auth())
	{
		sshTargets.POST("/test-connection", rt.sshTestConnection)
	}
}

// sshTestConnection 测试SSH连接
func (rt *Router) sshTestConnection(c *gin.Context) {
	var req struct {
		TargetIdent string `json:"target_ident" binding:"required"`
	}
	
	ginx.BindJSON(c, &req)

	err := service.TestSSHConnection(rt.Ctx, req.TargetIdent)
	if err != nil {
		ginx.NewRender(c).Data(nil, err.Error())
		return
	}

	ginx.NewRender(c).Message("SSH connection test successful")
}