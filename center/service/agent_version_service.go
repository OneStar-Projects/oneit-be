package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/ccfos/nightingale/v6/models"
	"github.com/ccfos/nightingale/v6/pkg/ctx"
)

// AgentVersionService 处理agent版本管理
type AgentVersionService struct {
	ctx *ctx.Context
}

// NewAgentVersionService 创建版本管理服务
func NewAgentVersionService(ctx *ctx.Context) *AgentVersionService {
	return &AgentVersionService{ctx: ctx}
}

// CreateVersion 创建新版本
func (s *AgentVersionService) CreateVersion(req *models.AgentVersion) error {
	// 验证版本号格式
	if err := s.validateVersion(req.Version); err != nil {
		return err
	}

	// 检查版本是否已存在
	exists, err := s.versionExists(req.ComponentID, req.Version)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("version %s already exists for component %d", req.Version, req.ComponentID)
	}

	// 检查是否为第一个版本
	isFirstVersion, err := s.isFirstVersion(req.ComponentID)
	if err != nil {
		return err
	}

	// 如果是第一个版本，自动设置为激活状态
	if isFirstVersion {
		req.IsActive = true
	} else if req.IsActive {
		// 如果不是第一个版本但设置为激活，需要将其他版本设为非活跃
		err = s.deactivateOtherVersions(req.ComponentID)
		if err != nil {
			return err
		}
	}

	// 计算文件哈希和大小
	if req.BinaryURL != "" {
		hash, size, err := s.calculateFileInfo(req.BinaryURL)
		if err != nil {
			return err
		}
		req.BinaryHash = hash
		req.BinarySize = size
	}

	return models.AgentVersionAdd(s.ctx, req)
}

// GetActiveVersion 获取组件的活跃版本
func (s *AgentVersionService) GetActiveVersion(componentID int64) (*models.AgentVersion, error) {
	return models.AgentVersionGetActive(s.ctx, componentID)
}

// ListVersions 列出组件的所有版本
func (s *AgentVersionService) ListVersions(componentID int64) ([]models.AgentVersion, error) {
	return models.AgentVersionGetsByComponent(s.ctx, componentID)
}

// ActivateVersion 激活指定版本
func (s *AgentVersionService) ActivateVersion(componentID, versionID int64) error {
	// 验证版本是否存在且属于该组件
	version, err := models.AgentVersionGet(s.ctx, versionID)
	if err != nil {
		return err
	}
	if version == nil {
		return fmt.Errorf("version %d not found", versionID)
	}
	if version.ComponentID != componentID {
		return fmt.Errorf("version %d does not belong to component %d", versionID, componentID)
	}

	return models.AgentVersionActivate(s.ctx, componentID, versionID)
}

// UpdateVersion 更新版本信息
func (s *AgentVersionService) UpdateVersion(versionID int64, updates map[string]interface{}) error {
	// 如果设置版本为激活状态，需要将其他版本设为非活跃
	if isActive, ok := updates["is_active"].(bool); ok && isActive {
		var version models.AgentVersion
		err := models.DB(s.ctx).Where("id = ?", versionID).First(&version).Error
		if err != nil {
			return err
		}
		err = s.deactivateOtherVersions(version.ComponentID)
		if err != nil {
			return err
		}
	}

	// 如果更新了二进制URL，重新计算哈希和大小
	if binaryURL, ok := updates["binary_url"].(string); ok && binaryURL != "" {
		hash, size, err := s.calculateFileInfo(binaryURL)
		if err != nil {
			return err
		}
		updates["binary_hash"] = hash
		updates["binary_size"] = size
	}

	return models.AgentVersionUpdate(s.ctx, versionID, updates)
}

// DeleteVersion 删除版本
func (s *AgentVersionService) DeleteVersion(versionID int64) error {
	// 检查版本是否存在
	version, err := models.AgentVersionGet(s.ctx, versionID)
	if err != nil {
		return err
	}
	if version == nil {
		return fmt.Errorf("version %d not found", versionID)
	}

	// 如果是活跃版本，不允许删除
	if version.IsActive {
		return fmt.Errorf("cannot delete active version %s", version.Version)
	}

	return models.AgentVersionDel(s.ctx, []int64{versionID})
}

// validateVersion 验证版本号格式
func (s *AgentVersionService) validateVersion(version string) error {
	// 支持格式: v1.0.0, 1.0.0, v1.0, 1.0
	if version == "" {
		return fmt.Errorf("version cannot be empty")
	}

	// 移除v前缀
	version = strings.TrimPrefix(version, "v")

	parts := strings.Split(version, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return fmt.Errorf("invalid version format, expected format: v1.0.0 or 1.0.0")
	}

	// 验证每个部分都是数字
	for _, part := range parts {
		if part == "" {
			return fmt.Errorf("version part cannot be empty")
		}
		// 这里可以添加更严格的数字验证
	}

	return nil
}

// versionExists 检查版本是否存在
func (s *AgentVersionService) versionExists(componentID int64, version string) (bool, error) {
	var count int64
	err := models.DB(s.ctx).Model(&models.AgentVersion{}).Where("component_id = ? AND version = ?", componentID, version).Count(&count).Error
	return count > 0, err
}

// isFirstVersion 检查是否为第一个版本
func (s *AgentVersionService) isFirstVersion(componentID int64) (bool, error) {
	var count int64
	err := models.DB(s.ctx).Model(&models.AgentVersion{}).Where("component_id = ?", componentID).Count(&count).Error
	return count == 0, err
}

// deactivateOtherVersions 将其他版本设为非活跃
func (s *AgentVersionService) deactivateOtherVersions(componentID int64) error {
	return models.AgentVersionDeactivateOthers(s.ctx, componentID)
}

// calculateFileInfo 计算文件哈希值和大小
func (s *AgentVersionService) calculateFileInfo(filePath string) (string, int64, error) {
	// 如果是HTTP URL，下载文件后计算哈希
	if strings.HasPrefix(filePath, "http://") || strings.HasPrefix(filePath, "https://") {
		return s.calculateURLFileInfo(filePath)
	}

	// 本地文件
	file, err := os.Open(filePath)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()

	// 获取文件信息
	fileInfo, err := file.Stat()
	if err != nil {
		return "", 0, err
	}

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", 0, err
	}

	return hex.EncodeToString(hash.Sum(nil)), fileInfo.Size(), nil
}

// calculateURLFileInfo 计算URL文件的哈希值和大小
func (s *AgentVersionService) calculateURLFileInfo(url string) (string, int64, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("failed to download file: HTTP %d", resp.StatusCode)
	}

	// 获取文件大小
	contentLength := resp.ContentLength
	if contentLength < 0 {
		contentLength = 0 // 如果无法获取大小，设为0
	}

	hash := sha256.New()
	written, err := io.Copy(hash, resp.Body)
	if err != nil {
		return "", 0, err
	}

	// 如果无法从header获取大小，使用实际读取的字节数
	if contentLength == 0 {
		contentLength = written
	}

	return hex.EncodeToString(hash.Sum(nil)), contentLength, nil
}

// extractVersionFromFilename 从文件名中提取版本信息
func (s *AgentVersionService) extractVersionFromFilename(filename string) string {
	// 常见的版本模式匹配
	patterns := []string{
		`v(\d+\.\d+\.\d+)`,
		`(\d+\.\d+\.\d+)`,
		`v(\d+\.\d+)`,
		`(\d+\.\d+)`,
	}

	for _, pattern := range patterns {
		if match := regexp.MustCompile(pattern).FindStringSubmatch(filename); len(match) > 1 {
			return match[1]
		}
	}
	return ""
}

// GetVersionByID 根据ID获取版本
func (s *AgentVersionService) GetVersionByID(versionID int64) (*models.AgentVersion, error) {
	return models.AgentVersionGet(s.ctx, versionID)
}
