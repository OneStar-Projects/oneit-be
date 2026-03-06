package models

import (
	"time"

	"github.com/ccfos/nightingale/v6/pkg/ctx"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

// HostAgent represents an agent deployment on a managed host
type HostAgent struct {
	ID            int64  `json:"id" gorm:"primaryKey;type:bigint;autoIncrement;comment:'unique identifier'"`
	HostID        int64  `json:"host_id" gorm:"type:bigint;not null;index:idx_host_id;comment:'managed host ID'"`
	ComponentID   int64  `json:"component_id" gorm:"type:bigint;not null;index:idx_component_id;comment:'builtin component ID'"`
	Status        string `json:"status" gorm:"type:varchar(20);not null;default:'pending';comment:'deployment status: pending, deploying, success, failed'"`
	ConfigData    string `json:"config_data" gorm:"type:text;comment:'actual deployment configuration in JSON format'"`
	DeployedAt    int64  `json:"deployed_at" gorm:"type:bigint;not null;default:0;comment:'last deployment time'"`
	LastHeartbeat int64  `json:"last_heartbeat" gorm:"type:bigint;not null;default:0;comment:'last heartbeat time'"`
	ErrorMessage  string `json:"error_message" gorm:"type:text;comment:'error message if deployment failed'"`
	CreateAt      int64  `json:"create_at" gorm:"type:bigint;not null;default:0;comment:'create time'"`
	UpdateAt      int64  `json:"update_at" gorm:"type:bigint;not null;default:0;comment:'update time'"`
	CreateBy      string `json:"create_by" gorm:"type:varchar(64);not null;default:'';comment:'creator'"`
	UpdateBy      string `json:"update_by" gorm:"type:varchar(64);not null;default:'';comment:'updater'"`

	// Related objects (not stored in DB)
	Host      *ManagedHost      `json:"host" gorm:"-"`
	Component *BuiltinComponent `json:"component" gorm:"-"`
}

func (ha *HostAgent) TableName() string {
	return "host_agents"
}

// BeforeCreate hook to set create_at and update_at
func (ha *HostAgent) BeforeCreate(tx *gorm.DB) error {
	now := time.Now().Unix()
	ha.CreateAt = now
	ha.UpdateAt = now
	return nil
}

// BeforeUpdate hook to set update_at
func (ha *HostAgent) BeforeUpdate(tx *gorm.DB) error {
	ha.UpdateAt = time.Now().Unix()
	return nil
}

// --- CRUD Methods ---

// HostAgentGet gets a single HostAgent by ID
func HostAgentGet(ctx *ctx.Context, id int64) (*HostAgent, error) {
	var obj HostAgent
	err := DB(ctx).Where("id = ?", id).First(&obj).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &obj, nil
}

// HostAgentGetByHostAndComponent gets a HostAgent by host_id and component_id
func HostAgentGetByHostAndComponent(ctx *ctx.Context, hostID, componentID int64) (*HostAgent, error) {
	var obj HostAgent
	err := DB(ctx).Where("host_id = ? AND component_id = ?", hostID, componentID).First(&obj).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &obj, nil
}

// HostAgentGetsByHost gets all HostAgents for a specific host
func HostAgentGetsByHost(ctx *ctx.Context, hostID int64) ([]HostAgent, error) {
	var lst []HostAgent
	err := DB(ctx).Where("host_id = ?", hostID).Order("create_at desc").Find(&lst).Error
	return lst, err
}

// HostAgentGetsByComponent gets all HostAgents for a specific component
func HostAgentGetsByComponent(ctx *ctx.Context, componentID int64) ([]HostAgent, error) {
	var lst []HostAgent
	err := DB(ctx).Where("component_id = ?", componentID).Order("create_at desc").Find(&lst).Error
	return lst, err
}

// HostAgentAdd adds a new HostAgent
func HostAgentAdd(ctx *ctx.Context, obj *HostAgent) error {
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

	// Check if host-agent combination already exists
	exists, err := HostAgentExistsByHostAndComponent(ctx, obj.HostID, obj.ComponentID)
	if err != nil {
		return errors.Wrap(err, "failed to check host-agent existence")
	}
	if exists {
		return errors.New("host-agent combination already exists")
	}

	return DB(ctx).Create(obj).Error
}

// HostAgentUpdate updates an existing HostAgent
func HostAgentUpdate(ctx *ctx.Context, id int64, updates map[string]interface{}) error {
	updates["update_at"] = time.Now().Unix()
	return DB(ctx).Model(&HostAgent{}).Where("id = ?", id).Updates(updates).Error
}

// HostAgentDel deletes HostAgents by IDs
func HostAgentDel(ctx *ctx.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	return DB(ctx).Where("id in ?", ids).Delete(&HostAgent{}).Error
}

// HostAgentDelByHost deletes all HostAgents for a specific host
func HostAgentDelByHost(ctx *ctx.Context, hostID int64) error {
	return DB(ctx).Where("host_id = ?", hostID).Delete(&HostAgent{}).Error
}

// HostAgentExists checks if a HostAgent exists by ID
func HostAgentExists(ctx *ctx.Context, id int64) (bool, error) {
	var count int64
	err := DB(ctx).Model(&HostAgent{}).Where("id = ?", id).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// HostAgentExistsByHostAndComponent checks if a HostAgent exists by host_id and component_id
func HostAgentExistsByHostAndComponent(ctx *ctx.Context, hostID, componentID int64) (bool, error) {
	var count int64
	err := DB(ctx).Model(&HostAgent{}).Where("host_id = ? AND component_id = ?", hostID, componentID).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// FillRelatedData fills related host and component data
func (ha *HostAgent) FillRelatedData(ctx *ctx.Context) error {
	// Fill host data
	host, err := ManagedHostGet(ctx, ha.HostID)
	if err != nil {
		return errors.Wrap(err, "failed to get host")
	}
	ha.Host = host

	// Fill component data
	component, err := BuiltinComponentGet(ctx, "id = ?", ha.ComponentID)
	if err != nil {
		return errors.Wrap(err, "failed to get component")
	}
	ha.Component = component

	return nil
}

// HostAgentGets gets HostAgents with pagination and filters
func HostAgentGets(ctx *ctx.Context, limit, offset int, hostID, componentID int64, status string) ([]HostAgent, error) {
	var lst []HostAgent
	query := DB(ctx)

	if hostID > 0 {
		query = query.Where("host_id = ?", hostID)
	}
	if componentID > 0 {
		query = query.Where("component_id = ?", componentID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	err := query.Order("create_at desc").Limit(limit).Offset(offset).Find(&lst).Error
	return lst, err
}

// HostAgentCount counts HostAgents with filters
func HostAgentCount(ctx *ctx.Context, hostID, componentID int64, status string) (int64, error) {
	var count int64
	query := DB(ctx).Model(&HostAgent{})

	if hostID > 0 {
		query = query.Where("host_id = ?", hostID)
	}
	if componentID > 0 {
		query = query.Where("component_id = ?", componentID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	err := query.Count(&count).Error
	return count, err
}
