# Agent版本管理功能

## 概述

Agent版本管理功能允许您为每个内置组件管理多个版本的agent二进制文件、配置模板和部署脚本。每个组件同时只能有一个活跃版本，新创建的版本可以手动激活。

## 功能特性

### 1. 版本管理
- **版本列表**：查看组件的所有版本
- **版本创建**：创建新版本，包含二进制文件、配置模板、部署脚本等
- **版本编辑**：修改现有版本的信息
- **版本删除**：删除非活跃版本
- **版本激活**：将指定版本设为活跃版本

### 2. 自动激活逻辑
- **第一个版本**：创建的第一个版本自动设置为活跃版本
- **唯一活跃**：同时只能有一个版本处于活跃状态
- **激活切换**：激活新版本时，自动将其他版本设为非活跃

### 3. 版本信息
每个版本包含以下信息：
- **版本号**：如 v1.0.0, v1.1.0
- **二进制文件**：下载URL、文件大小、SHA256哈希值
- **配置模板**：agent配置文件模板
- **部署脚本**：Ansible部署脚本
- **额外变量**：JSON格式的默认变量
- **发布说明**：版本更新说明

## 数据库表结构

### agent_versions 表
```sql
CREATE TABLE `agent_versions` (
    `id` bigint unsigned not null auto_increment,
    `component_id` bigint not null comment 'builtin component ID',
    `version` varchar(50) not null comment '版本号，如v1.0.0',
    `binary_url` varchar(500) comment '二进制文件下载URL',
    `binary_hash` varchar(64) comment '文件SHA256哈希值',
    `binary_size` bigint comment '文件大小(字节)',
    `config_template` text comment '配置模板内容',
    `ansible_script` text comment 'Ansible部署脚本',
    `extra_vars` text comment '默认变量JSON格式',
    `release_notes` text comment '发布说明',
    `is_active` boolean default true comment '是否为当前活跃版本',
    `create_at` bigint not null default 0 comment '创建时间',
    `create_by` varchar(64) not null default '' comment '创建者',
    PRIMARY KEY (`id`),
    KEY `idx_component_id` (`component_id`),
    KEY `idx_is_active` (`is_active`)
);
```

### agent_deployments 表
```sql
CREATE TABLE `agent_deployments` (
    `id` bigint unsigned not null auto_increment,
    `host_id` bigint not null comment 'managed host ID',
    `component_id` bigint not null comment 'builtin component ID',
    `version_id` bigint not null comment 'agent version ID',
    `status` varchar(20) not null default 'pending' comment '部署状态',
    `config_data` text comment '实际部署配置JSON',
    `deployed_at` bigint not null default 0 comment '部署时间',
    `last_heartbeat` bigint not null default 0 comment '最后心跳时间',
    `error_message` text comment '错误信息',
    `create_at` bigint not null default 0 comment '创建时间',
    `create_by` varchar(64) not null default '' comment '创建者',
    `update_at` bigint not null default 0 comment '更新时间',
    `update_by` varchar(64) not null default '' comment '更新者',
    PRIMARY KEY (`id`),
    KEY `idx_host_id` (`host_id`),
    KEY `idx_component_id` (`component_id`),
    KEY `idx_version_id` (`version_id`),
    KEY `idx_status` (`status`)
);
```

## API接口

### 版本管理API

#### 1. 获取版本列表
```
GET /api/n9e/agent-versions/component/{component_id}
```

#### 2. 获取活跃版本
```
GET /api/n9e/agent-versions/component/{component_id}/active
```

#### 3. 创建新版本
```
POST /api/n9e/agent-versions
Content-Type: application/json

{
    "component_id": 1,
    "version": "v1.0.0",
    "binary_url": "https://example.com/agent-v1.0.0.tar.gz",
    "config_template": "server_addr: {{server_addr}}\nport: {{port}}",
    "ansible_script": "---\n- name: Deploy agent\n  copy:\n    src: agent.tar.gz\n    dest: /opt/agent/",
    "extra_vars": "{\"server_addr\": \"localhost\", \"port\": 8080}",
    "release_notes": "Initial release",
    "is_active": true
}
```

#### 4. 更新版本
```
PUT /api/n9e/agent-versions/version/{id}
Content-Type: application/json

{
    "version": "v1.0.1",
    "binary_url": "https://example.com/agent-v1.0.1.tar.gz",
    "release_notes": "Bug fixes and improvements"
}
```

#### 5. 删除版本
```
DELETE /api/n9e/agent-versions/version/{id}
```

#### 6. 激活版本
```
POST /api/n9e/agent-versions/component/{component_id}/activate/{version_id}
```

## 前端使用

### 1. 访问版本管理
1. 进入内置组件管理页面
2. 选择要管理的组件
3. 点击"Agent管理"标签页
4. 切换到"版本管理"子标签

### 2. 创建新版本
1. 点击"创建新版本"按钮
2. 填写版本信息：
   - 版本号（如 v1.0.0）
   - 二进制文件URL
   - 配置模板
   - Ansible部署脚本
   - 额外变量（JSON格式）
   - 发布说明
   - 是否设为活跃版本
3. 点击"保存"

### 3. 管理版本
- **查看详情**：点击版本行的信息图标
- **编辑版本**：点击版本行的编辑图标
- **激活版本**：点击版本行的激活图标（仅非活跃版本）
- **删除版本**：点击版本行的删除图标（仅非活跃版本）

## 最佳实践

### 1. 版本命名
- 使用语义化版本号：v1.0.0, v1.1.0, v2.0.0
- 保持版本号的一致性

### 2. 二进制文件
- 使用HTTPS URL确保安全性
- 提供文件大小和哈希值用于验证
- 支持常见的压缩格式：.tar.gz, .zip

### 3. 配置模板
- 使用模板变量：{{variable_name}}
- 提供合理的默认值
- 包含必要的配置项

### 4. 部署脚本
- 使用Ansible playbook格式
- 包含错误处理和回滚机制
- 测试脚本的正确性

### 5. 版本管理
- 保持一个稳定的活跃版本
- 在测试环境验证新版本
- 及时更新发布说明

## 测试

运行测试脚本验证功能：
```bash
chmod +x scripts/test_agent_version.sh
./scripts/test_agent_version.sh
```

## 故障排除

### 常见问题

1. **版本创建失败**
   - 检查版本号格式是否正确
   - 确认组件ID存在
   - 验证二进制URL可访问

2. **激活版本失败**
   - 确认版本存在且属于指定组件
   - 检查数据库连接

3. **删除版本失败**
   - 确认版本不是当前活跃版本
   - 检查是否有部署记录引用该版本

### 日志查看
查看后端日志获取详细错误信息：
```bash
tail -f logs/n9e.log
```
