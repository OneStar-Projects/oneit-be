package service

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"text/template"
	"time"

	"github.com/ccfos/nightingale/v6/models"
	"github.com/ccfos/nightingale/v6/pkg/ctx"
	"github.com/toolkits/pkg/logger"
	"gorm.io/gorm"
)

// AgentDeployService 处理agent部署
type AgentDeployService struct {
	ctx *ctx.Context
}

// NewAgentDeployService 创建部署服务
func NewAgentDeployService(ctx *ctx.Context) *AgentDeployService {
	return &AgentDeployService{ctx: ctx}
}

// AgentDeployRequest 部署请求
type AgentDeployRequest struct {
	HostIDs     []int64                `json:"host_ids"`
	ComponentID int64                  `json:"component_id"`
	VersionID   int64                  `json:"version_id"`
	ConfigData  map[string]interface{} `json:"config_data"`
	DeployBy    string                 `json:"deploy_by"`
}

// AgentDeployResult 部署结果
type AgentDeployResult struct {
	TaskID   string                      `json:"task_id"`
	Status   string                      `json:"status"`
	Progress int                         `json:"progress"`
	Message  string                      `json:"message"`
	Results  map[int64]AgentDeployStatus `json:"results"`
	CreateAt int64                       `json:"create_at"`
	UpdateAt int64                       `json:"update_at"`
}

// AgentDeployStatus 单个主机部署状态
type AgentDeployStatus struct {
	HostID     int64  `json:"host_id"`
	Status     string `json:"status"`
	Message    string `json:"message"`
	DeployedAt int64  `json:"deployed_at"`
}

// DeployAgents 批量部署agents
func (s *AgentDeployService) DeployAgents(req *AgentDeployRequest) (*AgentDeployResult, error) {
	// 验证请求
	if err := s.validateDeployRequest(req); err != nil {
		return nil, err
	}

	// 获取版本信息
	version, err := s.getVersion(req.VersionID)
	if err != nil {
		return nil, err
	}

	// 获取组件信息
	component, err := s.getComponent(req.ComponentID)
	if err != nil {
		return nil, err
	}

	// 创建部署任务
	taskID := s.generateTaskID()
	result := &AgentDeployResult{
		TaskID:   taskID,
		Status:   "pending",
		Progress: 0,
		Message:  "Deployment started",
		Results:  make(map[int64]AgentDeployStatus),
		CreateAt: time.Now().Unix(),
		UpdateAt: time.Now().Unix(),
	}

	// 异步执行部署
	go s.executeDeployment(taskID, req, version, component, result)

	return result, nil
}

// executeDeployment 执行部署
func (s *AgentDeployService) executeDeployment(taskID string, req *AgentDeployRequest, version *models.AgentVersion, component *models.BuiltinComponent, result *AgentDeployResult) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("Deployment panic: %v", r)
			result.Status = "failed"
			result.Message = fmt.Sprintf("Deployment panic: %v", r)
			result.UpdateAt = time.Now().Unix()
		}
	}()

	// 更新状态为运行中
	result.Status = "running"
	result.UpdateAt = time.Now().Unix()

	totalHosts := len(req.HostIDs)
	successCount := 0
	failedCount := 0

	for i, hostID := range req.HostIDs {
		// 更新进度
		result.Progress = (i * 100) / totalHosts
		result.UpdateAt = time.Now().Unix()

		// 部署单个主机
		status := s.deployToHost(hostID, req, version, component)
		result.Results[hostID] = status

		if status.Status == "success" {
			successCount++
		} else {
			failedCount++
		}
	}

	// 完成部署
	result.Progress = 100
	result.UpdateAt = time.Now().Unix()

	if failedCount == 0 {
		result.Status = "success"
		result.Message = fmt.Sprintf("All %d hosts deployed successfully", totalHosts)
	} else if successCount == 0 {
		result.Status = "failed"
		result.Message = fmt.Sprintf("All %d hosts failed to deploy", totalHosts)
	} else {
		result.Status = "partial_success"
		result.Message = fmt.Sprintf("%d hosts deployed successfully, %d hosts failed", successCount, failedCount)
	}
}

// deployToHost 部署到单个主机
func (s *AgentDeployService) deployToHost(hostID int64, req *AgentDeployRequest, version *models.AgentVersion, component *models.BuiltinComponent) AgentDeployStatus {
	status := AgentDeployStatus{
		HostID: hostID,
		Status: "pending",
	}

	// 获取主机信息
	host, err := s.getHost(hostID)
	if err != nil {
		status.Status = "failed"
		status.Message = fmt.Sprintf("Failed to get host: %v", err)
		return status
	}

	// 创建或更新部署记录
	deployment, err := s.createOrUpdateDeployment(hostID, req, version)
	if err != nil {
		status.Status = "failed"
		status.Message = fmt.Sprintf("Failed to create deployment record: %v", err)
		return status
	}

	// 生成部署配置
	config, err := s.generateDeployConfig(version, req.ConfigData)
	if err != nil {
		status.Status = "failed"
		status.Message = fmt.Sprintf("Failed to generate config: %v", err)
		return status
	}

	// 执行ansible部署
	err = s.executeAnsibleDeploy(host, version, config)
	if err != nil {
		status.Status = "failed"
		status.Message = fmt.Sprintf("Ansible deployment failed: %v", err)
		s.updateDeploymentStatus(deployment.ID, "failed", err.Error())
		return status
	}

	// 更新部署状态
	status.Status = "success"
	status.Message = "Deployment completed successfully"
	status.DeployedAt = time.Now().Unix()
	s.updateDeploymentStatus(deployment.ID, "success", "")

	return status
}

// generateDeployConfig 生成部署配置
func (s *AgentDeployService) generateDeployConfig(version *models.AgentVersion, customConfig map[string]interface{}) (map[string]interface{}, error) {
	config := map[string]interface{}{
		"agent_type":       version.Component.AgentType,
		"agent_version":    version.Version,
		"agent_binary_url": version.BinaryURL,
		"install_path":     fmt.Sprintf("/opt/%s", version.Component.AgentType),
		"config_path":      fmt.Sprintf("/opt/%s/conf", version.Component.AgentType),
		"log_path":         fmt.Sprintf("/var/log/%s", version.Component.AgentType),
		"service_name":     version.Component.AgentType,
	}

	// 解析默认变量
	if version.ExtraVars != "" {
		var defaultVars map[string]interface{}
		if err := json.Unmarshal([]byte(version.ExtraVars), &defaultVars); err == nil {
			for k, v := range defaultVars {
				config[k] = v
			}
		}
	}

	// 合并自定义配置
	for k, v := range customConfig {
		config[k] = v
	}

	return config, nil
}

// executeAnsibleDeploy 执行ansible部署
func (s *AgentDeployService) executeAnsibleDeploy(host *models.ManagedHost, version *models.AgentVersion, config map[string]interface{}) error {
	// 创建临时ansible playbook
	playbookPath, err := s.createTempPlaybook(version)
	if err != nil {
		return fmt.Errorf("failed to create playbook: %v", err)
	}
	defer os.Remove(playbookPath)

	// 创建临时inventory
	inventoryPath, err := s.createTempInventory(host)
	if err != nil {
		return fmt.Errorf("failed to create inventory: %v", err)
	}
	defer os.Remove(inventoryPath)

	// 创建临时配置文件
	configPath, err := s.createTempConfig(version, config)
	if err != nil {
		return fmt.Errorf("failed to create config: %v", err)
	}
	defer os.Remove(configPath)

	// 执行ansible命令
	cmd := exec.Command("ansible-playbook",
		"-i", inventoryPath,
		playbookPath,
		"--extra-vars", fmt.Sprintf("'%s'", s.configToJSON(config)),
		"-v")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ansible execution failed: %v, output: %s", err, string(output))
	}

	logger.Infof("Ansible deployment completed for host %s: %s", host.HostIdent, string(output))
	return nil
}

// createTempPlaybook 创建临时playbook文件
func (s *AgentDeployService) createTempPlaybook(version *models.AgentVersion) (string, error) {
	// 使用版本的ansible_script作为模板
	tmpl, err := template.New("playbook").Parse(version.AnsibleScript)
	if err != nil {
		return "", fmt.Errorf("failed to parse ansible script template: %v", err)
	}

	// 创建临时文件
	tmpFile, err := os.CreateTemp("", "agent-deploy-*.yml")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %v", err)
	}
	defer tmpFile.Close()

	// 执行模板
	err = tmpl.Execute(tmpFile, nil)
	if err != nil {
		return "", fmt.Errorf("failed to execute template: %v", err)
	}

	return tmpFile.Name(), nil
}

// createTempInventory 创建临时inventory文件
func (s *AgentDeployService) createTempInventory(host *models.ManagedHost) (string, error) {
	inventoryContent := fmt.Sprintf(`[all]
%s ansible_host=%s ansible_user=%s ansible_port=%d

[all:vars]
ansible_connection=ssh
ansible_ssh_private_key_file=%s
ansible_become=yes
ansible_become_method=sudo
`,
		host.HostIdent,
		host.SSHIp,
		host.SSHUser,
		host.SSHPort,
		host.CredentialRef)

	tmpFile, err := os.CreateTemp("", "inventory-*.ini")
	if err != nil {
		return "", fmt.Errorf("failed to create temp inventory file: %v", err)
	}
	defer tmpFile.Close()

	_, err = tmpFile.WriteString(inventoryContent)
	if err != nil {
		return "", fmt.Errorf("failed to write inventory content: %v", err)
	}

	return tmpFile.Name(), nil
}

// createTempConfig 创建临时配置文件
func (s *AgentDeployService) createTempConfig(version *models.AgentVersion, config map[string]interface{}) (string, error) {
	// 使用版本的config_template作为模板
	tmpl, err := template.New("config").Parse(version.ConfigTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse config template: %v", err)
	}

	// 创建临时文件
	tmpFile, err := os.CreateTemp("", "agent-config-*.conf")
	if err != nil {
		return "", fmt.Errorf("failed to create temp config file: %v", err)
	}
	defer tmpFile.Close()

	// 执行模板
	err = tmpl.Execute(tmpFile, config)
	if err != nil {
		return "", fmt.Errorf("failed to execute config template: %v", err)
	}

	return tmpFile.Name(), nil
}

// updateDeploymentStatus 更新部署状态
func (s *AgentDeployService) updateDeploymentStatus(deploymentID int64, status string, errorMessage string) {
	updates := map[string]interface{}{
		"status":      status,
		"deployed_at": time.Now().Unix(),
		"update_at":   time.Now().Unix(),
	}

	if errorMessage != "" {
		updates["error_message"] = errorMessage
	}

	err := models.AgentDeploymentUpdate(s.ctx, deploymentID, updates)
	if err != nil {
		logger.Errorf("Failed to update deployment status: %v", err)
	}
}

// configToJSON 将配置转换为JSON字符串
func (s *AgentDeployService) configToJSON(config map[string]interface{}) string {
	jsonBytes, err := json.Marshal(config)
	if err != nil {
		return "{}"
	}
	return string(jsonBytes)
}

// 其他辅助方法
func (s *AgentDeployService) validateDeployRequest(req *AgentDeployRequest) error {
	if len(req.HostIDs) == 0 {
		return fmt.Errorf("host_ids cannot be empty")
	}
	if req.ComponentID == 0 {
		return fmt.Errorf("component_id is required")
	}
	if req.VersionID == 0 {
		return fmt.Errorf("version_id is required")
	}
	return nil
}

func (s *AgentDeployService) getVersion(versionID int64) (*models.AgentVersion, error) {
	var version models.AgentVersion
	err := models.DB(s.ctx).Where("id = ?", versionID).First(&version).Error
	if err != nil {
		return nil, err
	}
	return &version, nil
}

func (s *AgentDeployService) getComponent(componentID int64) (*models.BuiltinComponent, error) {
	var component models.BuiltinComponent
	err := models.DB(s.ctx).Where("id = ?", componentID).First(&component).Error
	if err != nil {
		return nil, err
	}
	return &component, nil
}

func (s *AgentDeployService) getHost(hostID int64) (*models.ManagedHost, error) {
	var host models.ManagedHost
	err := models.DB(s.ctx).Where("id = ?", hostID).First(&host).Error
	if err != nil {
		return nil, err
	}
	return &host, nil
}

func (s *AgentDeployService) generateTaskID() string {
	return fmt.Sprintf("deploy_%d", time.Now().UnixNano())
}

func (s *AgentDeployService) createOrUpdateDeployment(hostID int64, req *AgentDeployRequest, version *models.AgentVersion) (*models.AgentDeployment, error) {
	var deployment models.AgentDeployment

	err := models.DB(s.ctx).Where("host_id = ? AND component_id = ?", hostID, req.ComponentID).First(&deployment).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	if err == gorm.ErrRecordNotFound {
		// 创建新部署记录
		deployment = models.AgentDeployment{
			HostID:      hostID,
			ComponentID: req.ComponentID,
			VersionID:   req.VersionID,
			Status:      "pending",
			CreateBy:    req.DeployBy,
			UpdateBy:    req.DeployBy,
		}
		err = models.AgentDeploymentAdd(s.ctx, &deployment)
	} else {
		// 更新现有部署记录
		updates := map[string]interface{}{
			"version_id": req.VersionID,
			"status":     "pending",
			"update_by":  req.DeployBy,
		}
		err = models.AgentDeploymentUpdate(s.ctx, deployment.ID, updates)
	}

	if err != nil {
		return nil, err
	}

	return &deployment, nil
}
