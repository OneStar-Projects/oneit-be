# Agent Version Management Test Script (PowerShell)
# 测试agent版本管理功能

# 配置
$API_BASE = "http://localhost:18000/api/n9e"
$COMPONENT_ID = 1

Write-Host "=== Agent Version Management Test ===" -ForegroundColor Green

# 1. 创建第一个版本（应该自动设置为活跃版本）
Write-Host "1. Creating first version (should be auto-activated)..." -ForegroundColor Yellow
$firstVersionBody = @{
    component_id = $COMPONENT_ID
    version = "v1.0.0"
    binary_url = "https://example.com/agent-v1.0.0.tar.gz"
    config_template = "server_addr: {{server_addr}}`nport: {{port}}"
    ansible_script = "---`n- name: Deploy agent`n  copy:`n    src: agent.tar.gz`n    dest: /opt/agent/"
    extra_vars = '{"server_addr": "localhost", "port": 8080}'
    release_notes = "Initial release with basic monitoring capabilities"
} | ConvertTo-Json

try {
    $firstVersionResponse = Invoke-RestMethod -Uri "$API_BASE/agent-versions" -Method POST -Body $firstVersionBody -ContentType "application/json"
    Write-Host "Response: $($firstVersionResponse | ConvertTo-Json)" -ForegroundColor Green
} catch {
    Write-Host "Error creating first version: $($_.Exception.Message)" -ForegroundColor Red
}

# 2. 获取版本列表
Write-Host "`n2. Getting version list..." -ForegroundColor Yellow
try {
    $versionsResponse = Invoke-RestMethod -Uri "$API_BASE/agent-versions/component/$COMPONENT_ID" -Method GET
    Write-Host "Versions: $($versionsResponse | ConvertTo-Json)" -ForegroundColor Green
} catch {
    Write-Host "Error getting versions: $($_.Exception.Message)" -ForegroundColor Red
}

# 3. 获取活跃版本
Write-Host "`n3. Getting active version..." -ForegroundColor Yellow
try {
    $activeVersionResponse = Invoke-RestMethod -Uri "$API_BASE/agent-versions/component/$COMPONENT_ID/active" -Method GET
    Write-Host "Active version: $($activeVersionResponse | ConvertTo-Json)" -ForegroundColor Green
} catch {
    Write-Host "Error getting active version: $($_.Exception.Message)" -ForegroundColor Red
}

# 4. 创建第二个版本（非活跃）
Write-Host "`n4. Creating second version (should not be active)..." -ForegroundColor Yellow
$secondVersionBody = @{
    component_id = $COMPONENT_ID
    version = "v1.1.0"
    binary_url = "https://example.com/agent-v1.1.0.tar.gz"
    config_template = "server_addr: {{server_addr}}`nport: {{port}}`nenable_ssl: {{enable_ssl}}"
    ansible_script = "---`n- name: Deploy agent v1.1.0`n  copy:`n    src: agent-v1.1.0.tar.gz`n    dest: /opt/agent/"
    extra_vars = '{"server_addr": "localhost", "port": 8080, "enable_ssl": true}'
    release_notes = "Added SSL support and improved performance"
    is_active = $false
} | ConvertTo-Json

try {
    $secondVersionResponse = Invoke-RestMethod -Uri "$API_BASE/agent-versions" -Method POST -Body $secondVersionBody -ContentType "application/json"
    Write-Host "Response: $($secondVersionResponse | ConvertTo-Json)" -ForegroundColor Green
} catch {
    Write-Host "Error creating second version: $($_.Exception.Message)" -ForegroundColor Red
}

# 5. 再次获取版本列表
Write-Host "`n5. Getting updated version list..." -ForegroundColor Yellow
try {
    $versionsResponse2 = Invoke-RestMethod -Uri "$API_BASE/agent-versions/component/$COMPONENT_ID" -Method GET
    Write-Host "Updated versions: $($versionsResponse2 | ConvertTo-Json)" -ForegroundColor Green
} catch {
    Write-Host "Error getting updated versions: $($_.Exception.Message)" -ForegroundColor Red
}

# 6. 激活第二个版本
Write-Host "`n6. Activating second version..." -ForegroundColor Yellow
try {
    # 首先获取第二个版本的ID
    $versionId = ($versionsResponse2 | Where-Object { $_.version -eq "v1.1.0" }).id
    if ($versionId) {
        $activateResponse = Invoke-RestMethod -Uri "$API_BASE/agent-versions/component/$COMPONENT_ID/activate/$versionId" -Method POST
        Write-Host "Activate response: $($activateResponse | ConvertTo-Json)" -ForegroundColor Green
    } else {
        Write-Host "Could not find version ID for v1.1.0" -ForegroundColor Red
    }
} catch {
    Write-Host "Error activating version: $($_.Exception.Message)" -ForegroundColor Red
}

# 7. 验证活跃版本已更改
Write-Host "`n7. Verifying active version changed..." -ForegroundColor Yellow
try {
    $activeVersionResponse2 = Invoke-RestMethod -Uri "$API_BASE/agent-versions/component/$COMPONENT_ID/active" -Method GET
    Write-Host "New active version: $($activeVersionResponse2 | ConvertTo-Json)" -ForegroundColor Green
} catch {
    Write-Host "Error getting new active version: $($_.Exception.Message)" -ForegroundColor Red
}

Write-Host "`n=== Test completed ===" -ForegroundColor Green
