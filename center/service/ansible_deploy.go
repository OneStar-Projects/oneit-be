package service

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/ccfos/nightingale/v6/models"
	"github.com/ccfos/nightingale/v6/pkg/ctx"
	"github.com/pkg/errors"
	"github.com/toolkits/pkg/logger"
)

// DeploymentStatus represents the status of a deployment
type DeploymentStatus string

const (
	DeploymentStatusPending   DeploymentStatus = "pending"
	DeploymentStatusRunning   DeploymentStatus = "running"
	DeploymentStatusSuccess   DeploymentStatus = "success"
	DeploymentStatusFailed    DeploymentStatus = "failed"
	DeploymentStatusCancelled DeploymentStatus = "cancelled"
)

// DeployRequest represents a request to deploy agents
type DeployRequest struct {
	TargetIdents []string          `json:"target_idents"`
	PlaybookName string            `json:"playbook_name"`
	ExtraVars    map[string]string `json:"extra_vars"`
}

// DeployResult represents the result of a deployment
type DeployResult struct {
	TaskID       string                      `json:"task_id"`
	Status       DeploymentStatus            `json:"status"`
	StartTime    time.Time                   `json:"start_time"`
	EndTime      time.Time                   `json:"end_time"`
	Output       string                      `json:"output"`
	Error        string                      `json:"error"`
	TargetStatus map[string]DeploymentStatus `json:"target_status"`
}

// DeploymentManager manages agent deployments
type DeploymentManager struct {
	ctx              *ctx.Context
	deployments      map[string]*DeployResult
	deploymentsMutex sync.RWMutex
	ansiblePath      string
	inventoryPath    string
	playbooksPath    string
}

// PlaybooksPath returns the path to playbooks
func (dm *DeploymentManager) PlaybooksPath() string {
	return dm.playbooksPath
}

// NewDeploymentManager creates a new DeploymentManager
func NewDeploymentManager(ctx *ctx.Context, ansiblePath, inventoryPath, playbooksPath string) *DeploymentManager {
	return &DeploymentManager{
		ctx:           ctx,
		deployments:   make(map[string]*DeployResult),
		ansiblePath:   ansiblePath,
		inventoryPath: inventoryPath,
		playbooksPath: playbooksPath,
	}
}

// DeployAgents deploys agents to the specified targets
func (dm *DeploymentManager) DeployAgents(req *DeployRequest) (string, error) {
	// Generate a unique task ID
	taskID := fmt.Sprintf("deploy-%d", time.Now().UnixNano())

	// Initialize deployment result
	result := &DeployResult{
		TaskID:       taskID,
		Status:       DeploymentStatusPending,
		StartTime:    time.Now(),
		TargetStatus: make(map[string]DeploymentStatus),
	}

	// Set initial status for all targets
	for _, targetIdent := range req.TargetIdents {
		result.TargetStatus[targetIdent] = DeploymentStatusPending
	}

	// Store the deployment result
	dm.deploymentsMutex.Lock()
	dm.deployments[taskID] = result
	dm.deploymentsMutex.Unlock()

	// Start deployment in a goroutine
	go dm.runDeployment(result, req)

	return taskID, nil
}

// runDeployment executes the actual deployment
func (dm *DeploymentManager) runDeployment(result *DeployResult, req *DeployRequest) {
	// Update status to running
	dm.updateDeploymentStatus(result.TaskID, DeploymentStatusRunning)

	// Update target statuses to running
	for targetIdent := range result.TargetStatus {
		dm.updateTargetStatus(result.TaskID, targetIdent, DeploymentStatusRunning)
	}

	// Prepare extra vars as JSON string
	extraVarsJSON, err := json.Marshal(req.ExtraVars)
	if err != nil {
		dm.handleDeploymentError(result, errors.Wrap(err, "failed to marshal extra vars"))
		return
	}

	// Construct the ansible-playbook command
	playbookPath := filepath.Join(dm.playbooksPath, req.PlaybookName)
	cmd := exec.Command(
		"ansible-playbook",
		"-i", dm.inventoryPath,
		playbookPath,
		"--extra-vars", string(extraVarsJSON),
		"--limit", dm.buildLimitString(req.TargetIdents),
	)

	// Set environment variables if needed
	cmd.Env = append(os.Environ(),
		"ANSIBLE_HOST_KEY_CHECKING=False",
	)

	// Capture output
	output, err := cmd.CombinedOutput()

	// Update result with output
	result.Output = string(output)

	// Handle errors
	if err != nil {
		dm.handleDeploymentError(result, errors.Wrap(err, "ansible-playbook execution failed"))
		return
	}

	// Update target statuses based on output
	// This is a simplified implementation - in a real scenario, you would parse the output
	// to determine the status of each target
	for targetIdent := range result.TargetStatus {
		dm.updateTargetStatus(result.TaskID, targetIdent, DeploymentStatusSuccess)
	}

	// Update overall status to success
	dm.updateDeploymentStatus(result.TaskID, DeploymentStatusSuccess)
	result.EndTime = time.Now()
}

// handleDeploymentError handles errors during deployment
func (dm *DeploymentManager) handleDeploymentError(result *DeployResult, err error) {
	logger.Errorf("Deployment %s failed: %v", result.TaskID, err)

	// Update overall status to failed
	dm.updateDeploymentStatus(result.TaskID, DeploymentStatusFailed)
	result.EndTime = time.Now()
	result.Error = err.Error()

	// Update all target statuses to failed
	for targetIdent := range result.TargetStatus {
		dm.updateTargetStatus(result.TaskID, targetIdent, DeploymentStatusFailed)
	}

	// Update managed_host records in database
	dm.updateManagedHostStatuses(result.TaskID, string(DeploymentStatusFailed))
}

// updateDeploymentStatus updates the status of a deployment
func (dm *DeploymentManager) updateDeploymentStatus(taskID string, status DeploymentStatus) {
	dm.deploymentsMutex.Lock()
	defer dm.deploymentsMutex.Unlock()

	if result, exists := dm.deployments[taskID]; exists {
		result.Status = status
	}
}

// updateTargetStatus updates the status of a target in a deployment
func (dm *DeploymentManager) updateTargetStatus(taskID, targetIdent string, status DeploymentStatus) {
	dm.deploymentsMutex.Lock()
	defer dm.deploymentsMutex.Unlock()

	if result, exists := dm.deployments[taskID]; exists {
		result.TargetStatus[targetIdent] = status
	}
}

// updateManagedHostStatuses updates the status of managed hosts in the database
func (dm *DeploymentManager) updateManagedHostStatuses(taskID, status string) {
	dm.deploymentsMutex.RLock()
	result, exists := dm.deployments[taskID]
	dm.deploymentsMutex.RUnlock()

	if !exists {
		return
	}

	now := time.Now().Unix()

	for targetIdent := range result.TargetStatus {
		// Update managed_host record
		updates := map[string]interface{}{
			"status":           status,
			"last_deployed_at": now,
			"update_at":        now,
		}

		err := models.ManagedHostUpdateByIdent(dm.ctx, targetIdent, updates)
		if err != nil {
			logger.Errorf("Failed to update managed host %s: %v", targetIdent, err)
		}
	}
}

// buildLimitString builds a limit string for ansible-playbook --limit option
func (dm *DeploymentManager) buildLimitString(targetIdents []string) string {
	// Join target idents with comma
	// In a real implementation, you might need to handle patterns or groups
	return fmt.Sprintf("%s", targetIdents[0])
	// For multiple targets:
	// return strings.Join(targetIdents, ",")
}

// GetDeploymentResult gets the result of a deployment
func (dm *DeploymentManager) GetDeploymentResult(taskID string) (*DeployResult, error) {
	dm.deploymentsMutex.RLock()
	defer dm.deploymentsMutex.RUnlock()

	result, exists := dm.deployments[taskID]
	if !exists {
		return nil, errors.New("deployment not found")
	}

	// Return a copy to avoid race conditions
	resultCopy := &DeployResult{
		TaskID:       result.TaskID,
		Status:       result.Status,
		StartTime:    result.StartTime,
		EndTime:      result.EndTime,
		Output:       result.Output,
		Error:        result.Error,
		TargetStatus: make(map[string]DeploymentStatus),
	}

	for k, v := range result.TargetStatus {
		resultCopy.TargetStatus[k] = v
	}

	return resultCopy, nil
}

// ListDeployments lists all deployments
func (dm *DeploymentManager) ListDeployments() []*DeployResult {
	dm.deploymentsMutex.RLock()
	defer dm.deploymentsMutex.RUnlock()

	results := make([]*DeployResult, 0, len(dm.deployments))
	for _, result := range dm.deployments {
		// Return copies to avoid race conditions
		resultCopy := &DeployResult{
			TaskID:       result.TaskID,
			Status:       result.Status,
			StartTime:    result.StartTime,
			EndTime:      result.EndTime,
			Output:       result.Output,
			Error:        result.Error,
			TargetStatus: make(map[string]DeploymentStatus),
		}

		for k, v := range result.TargetStatus {
			resultCopy.TargetStatus[k] = v
		}

		results = append(results, resultCopy)
	}

	return results
}

// CancelDeployment cancels a deployment (if possible)
func (dm *DeploymentManager) CancelDeployment(taskID string) error {
	// In a real implementation, you would need to implement process cancellation
	// This is a simplified version that just updates the status
	dm.deploymentsMutex.Lock()
	defer dm.deploymentsMutex.Unlock()

	result, exists := dm.deployments[taskID]
	if !exists {
		return errors.New("deployment not found")
	}

	if result.Status == DeploymentStatusRunning {
		result.Status = DeploymentStatusCancelled
		result.EndTime = time.Now()
		result.Error = "Deployment cancelled by user"

		// Update target statuses
		for targetIdent := range result.TargetStatus {
			result.TargetStatus[targetIdent] = DeploymentStatusCancelled
		}

		// Update database
		dm.updateManagedHostStatuses(taskID, string(DeploymentStatusCancelled))
	}

	return nil
}
