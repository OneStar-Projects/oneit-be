# Agent部署功能设计文档

## 概述

本文档描述了Nightingale系统中新增的Agent自动部署功能的设计和实现。该功能允许通过Web界面自动部署各种类型的监控Agent到目标主机。

## 设计目标

1. **自动化部署**：将手动部署agent改为页面自动下发部署
2. **模板化管理**：利用现有的BuiltinComponent功能管理agent配置模板
3. **多Agent支持**：支持一个主机部署多个不同类型的agent
4. **状态跟踪**：完整的部署状态管理和历史记录

## 数据模型设计

### 1. 扩展BuiltinComponent模型

扩展现有的`BuiltinComponent`模型，添加agent部署相关字段：

```go
type BuiltinComponent struct {
    // ... 现有字段 ...
    
    // Agent deployment related fields
    AgentType       string `json:"agent_type" gorm:"type:varchar(50);default:'categraf';comment:'agent type'"`
    AgentVersion    string `json:"agent_version" gorm:"type:varchar(50);comment:'agent version'"`
    AgentBinaryURL  string `json:"agent_binary_url" gorm:"type:varchar(500);comment:'agent binary file download URL'"`
    AnsibleScript   string `json:"ansible_script" gorm:"type:text;comment:'ansible deployment script content'"`
    ConfigTemplate  string `json:"config_template" gorm:"type:text;comment:'configuration template'"`
    ExtraVars       string `json:"extra_vars" gorm:"type:text;comment:'default extra variables in JSON format'"`
}
```

### 2. 重构ManagedHost模型

将`ManagedHost`重新定位为"待部署主机"：

```go
type ManagedHost struct {
    ID             int64  `json:"id" gorm:"primaryKey;type:bigint;autoIncrement"`
    HostIdent      string `json:"host_ident" gorm:"column:host_ident;type:varchar(191);not null;uniqueIndex"`
    SSHIp          string `json:"ssh_ip" gorm:"column:ssh_ip;type:varchar(15);not null"`
    SSHPort        int    `json:"ssh_port" gorm:"column:ssh_port;type:int;not null;default:22"`
    SSHUser        string `json:"ssh_user" gorm:"column:ssh_user;type:varchar(64);not null"`
    AuthMethod     string `json:"auth_method" gorm:"column:auth_method;type:varchar(10);not null"`
    CredentialRef  string `json:"credential_ref" gorm:"column:credential_ref;type:varchar(191);not null"`
    Status         string `json:"status" gorm:"column:status;type:varchar(20);not null;default:'pending'"`
    Note           string `json:"note" gorm:"column:note;type:varchar(1024);default:''"`
    SudoRequired   bool   `json:"sudo_required" gorm:"column:sudo_required;type:boolean;not null;default:false"`
    CreateAt       int64  `json:"create_at" gorm:"column:create_at;type:bigint;not null;default:0"`
    UpdateAt       int64  `json:"update_at" gorm:"column:update_at;type:bigint;not null;default:0"`
    CreateBy       string `json:"create_by" gorm:"column:create_by;type:varchar(64);not null;default:''"`
    UpdateBy       string `json:"update_by" gorm:"column:update_by;type:varchar(64);not null;default:''"`
}
```

### 3. 新增HostAgent模型

创建`HostAgent`模型管理主机上的Agent部署记录：

```go
type HostAgent struct {
    ID             int64  `json:"id" gorm:"primaryKey;type:bigint;autoIncrement"`
    HostID         int64  `json:"host_id" gorm:"type:bigint;not null;index:idx_host_id"`
    ComponentID    int64  `json:"component_id" gorm:"type:bigint;not null;index:idx_component_id"`
    Status         string `json:"status" gorm:"type:varchar(20);not null;default:'pending'"`
    ConfigData     string `json:"config_data" gorm:"type:text;comment:'actual deployment configuration'"`
    DeployedAt     int64  `json:"deployed_at" gorm:"type:bigint;not null;default:0"`
    LastHeartbeat  int64  `json:"last_heartbeat" gorm:"type:bigint;not null;default:0"`
    ErrorMessage   string `json:"error_message" gorm:"type:text;comment:'error message if deployment failed'"`
    CreateAt       int64  `json:"create_at" gorm:"type:bigint;not null;default:0"`
    UpdateAt       int64  `json:"update_at" gorm:"type:bigint;not null;default:0"`
    CreateBy       string `json:"create_by" gorm:"type:varchar(64);not null;default:''"`
    UpdateBy       string `json:"update_by" gorm:"type:varchar(64);not null;default:''"`
}
```

## 关系设计

```
ManagedHost (1) ←→ (N) HostAgent (N) ←→ (1) BuiltinComponent
```

- 一个ManagedHost可以部署多个HostAgent
- 一个BuiltinComponent可以被多个HostAgent使用
- HostAgent是ManagedHost和BuiltinComponent的多对多关系表

## 业务流程

### 1. 主机管理流程
1. 添加待部署主机（ManagedHost）
2. 配置SSH连接信息
3. 测试SSH连接
4. 主机状态：pending → active

### 2. Agent部署流程
1. 选择主机和Agent类型（BuiltinComponent）
2. 配置部署参数
3. 执行Ansible部署脚本
4. 部署状态：pending → deploying → success/failed
5. 部署成功后，agent开始heartbeat，自动创建Target记录

### 3. 状态管理
- **ManagedHost状态**：pending, active, failed, disabled
- **HostAgent状态**：pending, deploying, success, failed
- **Target状态**：通过agent heartbeat自动管理

## 技术实现

### 1. 数据库迁移
- 通过Go语言自动实施数据库修改
- 确保模型和数据库表一致
- 支持MySQL、PostgreSQL、SQLite

### 2. 现有功能复用
- 复用现有的BuiltinComponent模板功能
- 复用现有的Ansible部署功能（ansible_deploy.go）
- 复用现有的SSH凭证管理功能

### 3. 部署脚本存储
- 选择存储脚本内容而非文件路径
- 优势：版本控制、部署便利、一致性、审计追踪
- 支持模板变量和参数替换

## API设计

### 主机管理API
```
POST /api/n9e/managed-hosts          # 添加待部署主机
PUT /api/n9e/managed-hosts/:id       # 更新主机信息
DELETE /api/n9e/managed-hosts        # 删除主机
GET /api/n9e/managed-hosts           # 获取主机列表
POST /api/n9e/managed-hosts/test-ssh # 测试SSH连接
```

### Agent部署API
```
POST /api/n9e/host-agents            # 添加Agent部署记录
PUT /api/n9e/host-agents/:id         # 更新Agent配置
DELETE /api/n9e/host-agents          # 删除Agent部署
GET /api/n9e/host-agents             # 获取Agent部署列表
POST /api/n9e/deploy/agents          # 执行部署
GET /api/n9e/deploy/status/:task_id  # 获取部署状态
```

## 部署数据准备

### 具备的数据
✅ **主机信息**：SSH连接信息、认证方式  
✅ **Agent信息**：二进制文件路径、ansible脚本、配置模板  
✅ **关联关系**：主机与Agent的多对多关系  
✅ **部署状态**：每个Agent的部署状态和配置  

### 部署执行数据流
1. **部署准备阶段**：验证SSH连接、检查目标设备环境
2. **文件部署阶段**：下载、传输、解压agent二进制文件
3. **配置部署阶段**：生成、传输、验证配置文件
4. **服务启动阶段**：创建服务文件、注册系统服务、启动agent
5. **部署验证阶段**：验证agent连接、确认心跳上报、验证数据上报

## 优势

1. **解决流程问题**：先有ManagedHost，后有Target
2. **简化操作**：通过页面完成agent部署
3. **提高效率**：批量部署和状态跟踪
4. **降低门槛**：减少手动操作，提高自动化程度
5. **复用现有功能**：充分利用现有的模板和部署基础设施

## 总结

这个设计将ManagedHost重新定位为"部署管理"功能，而不是"已部署主机管理"功能。通过扩展现有的BuiltinComponent模型和新增HostAgent关联表，实现了完整的Agent自动部署功能，满足了"先有管理，后有agent"的业务需求。
