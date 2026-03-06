package models

import (
	"time"

	"github.com/ccfos/nightingale/v6/pkg/ctx"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

type ManagedHost struct {
	ID            int64  `json:"id" gorm:"primaryKey;type:bigint;autoIncrement;comment:'unique identifier'"`
	HostIdent     string `json:"host_ident" gorm:"column:host_ident;type:varchar(191);not null;uniqueIndex;comment:'host identifier (IP or hostname)'"`
	SSHIp         string `json:"ssh_ip" gorm:"column:ssh_ip;type:varchar(15);not null;comment:'SSH IP address'"`
	SSHPort       int    `json:"ssh_port" gorm:"column:ssh_port;type:int;not null;default:22;comment:'SSH port'"`
	SSHUser       string `json:"ssh_user" gorm:"column:ssh_user;type:varchar(64);not null;comment:'SSH username'"`
	AuthMethod    string `json:"auth_method" gorm:"column:auth_method;type:varchar(10);not null;comment:'authentication method: key or password'"`
	CredentialRef string `json:"credential_ref" gorm:"column:credential_ref;type:varchar(191);not null;comment:'credential reference'"`
	Status        string `json:"status" gorm:"column:status;type:varchar(20);not null;default:'pending';comment:'host status: pending, active, failed, disabled'"`
	Note          string `json:"note" gorm:"column:note;type:varchar(1024);default:'';comment:'host description'"`
	SudoRequired  bool   `json:"sudo_required" gorm:"column:sudo_required;type:boolean;not null;default:false;comment:'whether sudo is required'"`
	CreateAt      int64  `json:"create_at" gorm:"column:create_at;type:bigint;not null;default:0;comment:'create time'"`
	UpdateAt      int64  `json:"update_at" gorm:"column:update_at;type:bigint;not null;default:0;comment:'update time'"`
	CreateBy      string `json:"create_by" gorm:"column:create_by;type:varchar(64);not null;default:'';comment:'creator'"`
	UpdateBy      string `json:"update_by" gorm:"column:update_by;type:varchar(64);not null;default:'';comment:'updater'"`

	// Related objects (not stored in DB)
	HostAgents []*HostAgent `json:"host_agents" gorm:"-"`
	Target     *Target      `json:"target" gorm:"-"`
}

func (m *ManagedHost) TableName() string {
	return "managed_hosts"
}

// BeforeCreate hook to set create_at and update_at
func (m *ManagedHost) BeforeCreate(tx *gorm.DB) error {
	now := time.Now().Unix()
	m.CreateAt = now
	m.UpdateAt = now
	return nil
}

// BeforeUpdate hook to set update_at
func (m *ManagedHost) BeforeUpdate(tx *gorm.DB) error {
	m.UpdateAt = time.Now().Unix()
	return nil
}

// --- CRUD Methods ---

// ManagedHostGet gets a single ManagedHost by ID
func ManagedHostGet(ctx *ctx.Context, id int64) (*ManagedHost, error) {
	var obj ManagedHost
	err := DB(ctx).Where("id = ?", id).First(&obj).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &obj, nil
}

// ManagedHostGetByIdent gets a single ManagedHost by host_ident
func ManagedHostGetByIdent(ctx *ctx.Context, hostIdent string) (*ManagedHost, error) {
	var obj ManagedHost
	err := DB(ctx).Where("host_ident = ?", hostIdent).First(&obj).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &obj, nil
}

// ManagedHostGets gets a list of ManagedHosts with pagination and optional query
func ManagedHostGets(ctx *ctx.Context, limit, offset int, query string) ([]ManagedHost, error) {
	session := DB(ctx).Model(&ManagedHost{}).Limit(limit).Offset(offset).Order("create_at desc")

	if query != "" {
		queryStr := "%" + query + "%"
		session = session.Where("host_ident like ? or ssh_ip like ? or note like ?", queryStr, queryStr, queryStr)
	}

	var lst []ManagedHost
	err := session.Find(&lst).Error
	return lst, err
}

// ManagedHostCount counts the number of ManagedHosts matching the query
func ManagedHostCount(ctx *ctx.Context, query string) (int64, error) {
	session := DB(ctx).Model(&ManagedHost{})

	if query != "" {
		queryStr := "%" + query + "%"
		session = session.Where("host_ident like ? or ssh_ip like ? or note like ?", queryStr, queryStr, queryStr)
	}

	var count int64
	err := session.Count(&count).Error
	return count, err
}

// ManagedHostAdd adds a new ManagedHost
func ManagedHostAdd(ctx *ctx.Context, obj *ManagedHost) error {
	// Check if host_ident already exists
	exists, err := ManagedHostExistsByIdent(ctx, obj.HostIdent)
	if err != nil {
		return errors.Wrap(err, "failed to check host existence")
	}
	if exists {
		return errors.New("host_ident already exists")
	}

	return DB(ctx).Create(obj).Error
}

// ManagedHostUpdate updates an existing ManagedHost
func ManagedHostUpdate(ctx *ctx.Context, id int64, updates map[string]interface{}) error {
	updates["update_at"] = time.Now().Unix()
	return DB(ctx).Model(&ManagedHost{}).Where("id = ?", id).Updates(updates).Error
}

// ManagedHostUpdateByIdent updates an existing ManagedHost by host_ident
func ManagedHostUpdateByIdent(ctx *ctx.Context, hostIdent string, updates map[string]interface{}) error {
	updates["update_at"] = time.Now().Unix()
	return DB(ctx).Model(&ManagedHost{}).Where("host_ident = ?", hostIdent).Updates(updates).Error
}

// ManagedHostDel deletes ManagedHosts by IDs
func ManagedHostDel(ctx *ctx.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	return DB(ctx).Where("id in ?", ids).Delete(&ManagedHost{}).Error
}

// ManagedHostDelByIdents deletes ManagedHosts by host_idents
func ManagedHostDelByIdents(ctx *ctx.Context, hostIdents []string) error {
	if len(hostIdents) == 0 {
		return nil
	}
	return DB(ctx).Where("host_ident in ?", hostIdents).Delete(&ManagedHost{}).Error
}

// ManagedHostExists checks if a ManagedHost exists by ID
func ManagedHostExists(ctx *ctx.Context, id int64) (bool, error) {
	var count int64
	err := DB(ctx).Model(&ManagedHost{}).Where("id = ?", id).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// ManagedHostExistsByIdent checks if a ManagedHost exists by host_ident
func ManagedHostExistsByIdent(ctx *ctx.Context, hostIdent string) (bool, error) {
	var count int64
	err := DB(ctx).Model(&ManagedHost{}).Where("host_ident = ?", hostIdent).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
