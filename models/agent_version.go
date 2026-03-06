package models

import (
	"time"

	"github.com/ccfos/nightingale/v6/pkg/ctx"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

// AgentVersion 表示agent的版本信息
type AgentVersion struct {
	ID             int64  `json:"id" gorm:"primaryKey;type:bigint;autoIncrement"`
	ComponentID    int64  `json:"component_id" gorm:"type:bigint;not null;index:idx_component_id"`
	Version        string `json:"version" gorm:"type:varchar(50);not null;comment:'版本号，如v1.0.0'"`
	BinaryURL      string `json:"binary_url" gorm:"type:varchar(500);comment:'二进制文件下载URL'"`
	BinaryHash     string `json:"binary_hash" gorm:"type:varchar(64);comment:'文件SHA256哈希值'"`
	BinarySize     int64  `json:"binary_size" gorm:"type:bigint;comment:'文件大小(字节)'"`
	ConfigTemplate string `json:"config_template" gorm:"type:text;comment:'配置模板内容'"`
	AnsibleScript  string `json:"ansible_script" gorm:"type:text;comment:'Ansible部署脚本'"`
	ExtraVars      string `json:"extra_vars" gorm:"type:text;comment:'默认变量JSON格式'"`
	ReleaseNotes   string `json:"release_notes" gorm:"type:text;comment:'发布说明'"`
	IsActive       bool   `json:"is_active" gorm:"type:boolean;default:true;comment:'是否为当前活跃版本'"`
	CreateAt       int64  `json:"create_at" gorm:"type:bigint;not null;default:0"`
	CreateBy       string `json:"create_by" gorm:"type:varchar(64);not null;default:''"`

	// 关联对象
	Component *BuiltinComponent `json:"component" gorm:"-"`
}

func (av *AgentVersion) TableName() string {
	return "agent_versions"
}

// BeforeCreate hook
func (av *AgentVersion) BeforeCreate(tx *gorm.DB) error {
	now := time.Now().Unix()
	av.CreateAt = now
	return nil
}

// AgentDeployment 表示agent部署记录
type AgentDeployment struct {
	ID            int64  `json:"id" gorm:"primaryKey;type:bigint;autoIncrement"`
	HostID        int64  `json:"host_id" gorm:"type:bigint;not null;index:idx_host_id"`
	ComponentID   int64  `json:"component_id" gorm:"type:bigint;not null;index:idx_component_id"`
	VersionID     int64  `json:"version_id" gorm:"type:bigint;not null;index:idx_version_id"`
	Status        string `json:"status" gorm:"type:varchar(20);not null;default:'pending';comment:'部署状态'"`
	ConfigData    string `json:"config_data" gorm:"type:text;comment:'实际部署配置JSON'"`
	DeployedAt    int64  `json:"deployed_at" gorm:"type:bigint;not null;default:0"`
	LastHeartbeat int64  `json:"last_heartbeat" gorm:"type:bigint;not null;default:0"`
	ErrorMessage  string `json:"error_message" gorm:"type:text;comment:'错误信息'"`
	CreateAt      int64  `json:"create_at" gorm:"type:bigint;not null;default:0"`
	CreateBy      string `json:"create_by" gorm:"type:varchar(64);not null;default:''"`
	UpdateAt      int64  `json:"update_at" gorm:"type:bigint;not null;default:0"`
	UpdateBy      string `json:"update_by" gorm:"type:varchar(64);not null;default:''"`

	// 关联对象
	Host      *ManagedHost      `json:"host" gorm:"-"`
	Component *BuiltinComponent `json:"component" gorm:"-"`
	Version   *AgentVersion     `json:"version" gorm:"-"`
}

func (ad *AgentDeployment) TableName() string {
	return "agent_deployments"
}

// BeforeCreate hook
func (ad *AgentDeployment) BeforeCreate(tx *gorm.DB) error {
	now := time.Now().Unix()
	ad.CreateAt = now
	ad.UpdateAt = now
	return nil
}

// BeforeUpdate hook
func (ad *AgentDeployment) BeforeUpdate(tx *gorm.DB) error {
	ad.UpdateAt = time.Now().Unix()
	return nil
}

// --- CRUD Methods for AgentVersion ---

// AgentVersionGet gets a single AgentVersion by ID
func AgentVersionGet(ctx *ctx.Context, id int64) (*AgentVersion, error) {
	var obj AgentVersion
	err := DB(ctx).Where("id = ?", id).First(&obj).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &obj, nil
}

// AgentVersionGetsByComponent gets all AgentVersions for a specific component
func AgentVersionGetsByComponent(ctx *ctx.Context, componentID int64) ([]AgentVersion, error) {
	var lst []AgentVersion
	err := DB(ctx).Where("component_id = ?", componentID).Order("create_at DESC").Find(&lst).Error
	return lst, err
}

// AgentVersionGetActive gets the active version for a component
func AgentVersionGetActive(ctx *ctx.Context, componentID int64) (*AgentVersion, error) {
	var obj AgentVersion
	err := DB(ctx).Where("component_id = ? AND is_active = ?", componentID, true).First(&obj).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &obj, nil
}

// AgentVersionAdd adds a new AgentVersion
func AgentVersionAdd(ctx *ctx.Context, obj *AgentVersion) error {
	// Check if component exists
	component, err := BuiltinComponentGet(ctx, "id = ?", obj.ComponentID)
	if err != nil {
		return errors.Wrap(err, "failed to get component")
	}
	if component == nil {
		return errors.New("component does not exist")
	}

	// If this version is active, deactivate other versions
	if obj.IsActive {
		err = AgentVersionDeactivateOthers(ctx, obj.ComponentID)
		if err != nil {
			return errors.Wrap(err, "failed to deactivate other versions")
		}
	}

	return DB(ctx).Create(obj).Error
}

// AgentVersionUpdate updates an existing AgentVersion
func AgentVersionUpdate(ctx *ctx.Context, id int64, updates map[string]interface{}) error {
	// If setting as active, deactivate other versions
	if isActive, ok := updates["is_active"].(bool); ok && isActive {
		var version AgentVersion
		err := DB(ctx).Where("id = ?", id).First(&version).Error
		if err != nil {
			return err
		}
		err = AgentVersionDeactivateOthers(ctx, version.ComponentID)
		if err != nil {
			return errors.Wrap(err, "failed to deactivate other versions")
		}
	}

	updates["update_at"] = time.Now().Unix()
	return DB(ctx).Model(&AgentVersion{}).Where("id = ?", id).Updates(updates).Error
}

// AgentVersionDel deletes AgentVersions by IDs
func AgentVersionDel(ctx *ctx.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	return DB(ctx).Where("id in ?", ids).Delete(&AgentVersion{}).Error
}

// AgentVersionDeactivateOthers deactivates all other versions for a component
func AgentVersionDeactivateOthers(ctx *ctx.Context, componentID int64) error {
	return DB(ctx).Model(&AgentVersion{}).Where("component_id = ?", componentID).Update("is_active", false).Error
}

// AgentVersionActivate activates a specific version
func AgentVersionActivate(c *ctx.Context, componentID, versionID int64) error {
	return DB(c).Transaction(func(tx *gorm.DB) error {
		newCtx := &ctx.Context{DB: tx, IsCenter: c.IsCenter}

		// Deactivate all versions for this component
		err := AgentVersionDeactivateOthers(newCtx, componentID)
		if err != nil {
			return err
		}

		// Activate the specified version
		return tx.Model(&AgentVersion{}).Where("id = ?", versionID).Update("is_active", true).Error
	})
}

// --- CRUD Methods for AgentDeployment ---

// AgentDeploymentGet gets a single AgentDeployment by ID
func AgentDeploymentGet(ctx *ctx.Context, id int64) (*AgentDeployment, error) {
	var obj AgentDeployment
	err := DB(ctx).Where("id = ?", id).First(&obj).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &obj, nil
}

// AgentDeploymentGetByHostAndComponent gets deployment by host_id and component_id
func AgentDeploymentGetByHostAndComponent(ctx *ctx.Context, hostID, componentID int64) (*AgentDeployment, error) {
	var obj AgentDeployment
	err := DB(ctx).Where("host_id = ? AND component_id = ?", hostID, componentID).First(&obj).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &obj, nil
}

// AgentDeploymentGetsByHost gets all deployments for a specific host
func AgentDeploymentGetsByHost(ctx *ctx.Context, hostID int64) ([]AgentDeployment, error) {
	var lst []AgentDeployment
	err := DB(ctx).Where("host_id = ?", hostID).Order("create_at DESC").Find(&lst).Error
	return lst, err
}

// AgentDeploymentGetsByComponent gets all deployments for a specific component
func AgentDeploymentGetsByComponent(ctx *ctx.Context, componentID int64) ([]AgentDeployment, error) {
	var lst []AgentDeployment
	err := DB(ctx).Where("component_id = ?", componentID).Order("create_at DESC").Find(&lst).Error
	return lst, err
}

// AgentDeploymentAdd adds a new AgentDeployment
func AgentDeploymentAdd(ctx *ctx.Context, obj *AgentDeployment) error {
	// Check if host exists
	host, err := ManagedHostGet(ctx, obj.HostID)
	if err != nil {
		return errors.Wrap(err, "failed to get host")
	}
	if host == nil {
		return errors.New("host does not exist")
	}

	// Check if component exists
	component, err := BuiltinComponentGet(ctx, "id = ?", obj.ComponentID)
	if err != nil {
		return errors.Wrap(err, "failed to get component")
	}
	if component == nil {
		return errors.New("component does not exist")
	}

	// Check if version exists
	version, err := AgentVersionGet(ctx, obj.VersionID)
	if err != nil {
		return errors.Wrap(err, "failed to get version")
	}
	if version == nil {
		return errors.New("version does not exist")
	}

	return DB(ctx).Create(obj).Error
}

// AgentDeploymentUpdate updates an existing AgentDeployment
func AgentDeploymentUpdate(ctx *ctx.Context, id int64, updates map[string]interface{}) error {
	updates["update_at"] = time.Now().Unix()
	return DB(ctx).Model(&AgentDeployment{}).Where("id = ?", id).Updates(updates).Error
}

// AgentDeploymentDel deletes AgentDeployments by IDs
func AgentDeploymentDel(ctx *ctx.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	return DB(ctx).Where("id in ?", ids).Delete(&AgentDeployment{}).Error
}

// AgentDeploymentDelByHost deletes all deployments for a specific host
func AgentDeploymentDelByHost(ctx *ctx.Context, hostID int64) error {
	return DB(ctx).Where("host_id = ?", hostID).Delete(&AgentDeployment{}).Error
}
