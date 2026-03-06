#!/bin/bash

# Agent Version Management Test Script
# 测试agent版本管理功能

set -e

# 配置
API_BASE="http://localhost:18000/api/n9e"
COMPONENT_ID=1

echo "=== Agent Version Management Test ==="

# 1. 创建第一个版本（应该自动设置为活跃版本）
echo "1. Creating first version (should be auto-activated)..."
FIRST_VERSION_RESPONSE=$(curl -s -X POST "${API_BASE}/agent-versions" \
  -H "Content-Type: application/json" \
  -d '{
    "component_id": '"$COMPONENT_ID"',
    "version": "v1.0.0",
    "binary_url": "https://example.com/agent-v1.0.0.tar.gz",
    "config_template": "server_addr: {{server_addr}}\nport: {{port}}",
    "ansible_script": "---\n- name: Deploy agent\n  copy:\n    src: agent.tar.gz\n    dest: /opt/agent/",
    "extra_vars": "{\"server_addr\": \"localhost\", \"port\": 8080}",
    "release_notes": "Initial release with basic monitoring capabilities"
  }')

echo "Response: $FIRST_VERSION_RESPONSE"

# 2. 获取版本列表
echo -e "\n2. Getting version list..."
VERSIONS_RESPONSE=$(curl -s -X GET "${API_BASE}/agent-versions/component/${COMPONENT_ID}")
echo "Versions: $VERSIONS_RESPONSE"

# 3. 获取活跃版本
echo -e "\n3. Getting active version..."
ACTIVE_VERSION_RESPONSE=$(curl -s -X GET "${API_BASE}/agent-versions/component/${COMPONENT_ID}/active")
echo "Active version: $ACTIVE_VERSION_RESPONSE"

# 4. 创建第二个版本（非活跃）
echo -e "\n4. Creating second version (should not be active)..."
SECOND_VERSION_RESPONSE=$(curl -s -X POST "${API_BASE}/agent-versions" \
  -H "Content-Type: application/json" \
  -d '{
    "component_id": '"$COMPONENT_ID"',
    "version": "v1.1.0",
    "binary_url": "https://example.com/agent-v1.1.0.tar.gz",
    "config_template": "server_addr: {{server_addr}}\nport: {{port}}\nenable_ssl: {{enable_ssl}}",
    "ansible_script": "---\n- name: Deploy agent v1.1.0\n  copy:\n    src: agent-v1.1.0.tar.gz\n    dest: /opt/agent/",
    "extra_vars": "{\"server_addr\": \"localhost\", \"port\": 8080, \"enable_ssl\": true}",
    "release_notes": "Added SSL support and improved performance",
    "is_active": false
  }')

echo "Response: $SECOND_VERSION_RESPONSE"

# 5. 再次获取版本列表
echo -e "\n5. Getting updated version list..."
VERSIONS_RESPONSE2=$(curl -s -X GET "${API_BASE}/agent-versions/component/${COMPONENT_ID}")
echo "Updated versions: $VERSIONS_RESPONSE2"

# 6. 激活第二个版本
echo -e "\n6. Activating second version..."
# 首先获取第二个版本的ID
VERSION_ID=$(echo "$VERSIONS_RESPONSE2" | jq -r '.[] | select(.version == "v1.1.0") | .id')
if [ "$VERSION_ID" != "null" ] && [ "$VERSION_ID" != "" ]; then
  ACTIVATE_RESPONSE=$(curl -s -X POST "${API_BASE}/agent-versions/component/${COMPONENT_ID}/activate/${VERSION_ID}")
  echo "Activate response: $ACTIVATE_RESPONSE"
else
  echo "Could not find version ID for v1.1.0"
fi

# 7. 验证活跃版本已更改
echo -e "\n7. Verifying active version changed..."
ACTIVE_VERSION_RESPONSE2=$(curl -s -X GET "${API_BASE}/agent-versions/component/${COMPONENT_ID}/active")
echo "New active version: $ACTIVE_VERSION_RESPONSE2"

echo -e "\n=== Test completed ==="
