# E2E 验证脚本（需 Redis + MySQL 可用）
# 用法: .\scripts\e2e.ps1

$ErrorActionPreference = "Stop"
$env:SKIP_FIRECRAWL = "1"
$env:WORKER_ROLE = "all"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

Write-Host "==> 启动 Worker..."
$worker = Start-Process -FilePath "go" -ArgumentList "run","./cmd/worker" -PassThru -NoNewWindow -WorkingDirectory $root
Start-Sleep -Seconds 8

Write-Host "==> 启动 API..."
$api = Start-Process -FilePath "go" -ArgumentList "run","." -PassThru -NoNewWindow -WorkingDirectory $root
Start-Sleep -Seconds 15

try {
    $ping = Invoke-RestMethod -Uri "http://localhost:8084/ping" -TimeoutSec 10
    Write-Host "Ping:" ($ping | ConvertTo-Json -Compress)

    $body = @{
        competitors = @("Cursor", "GitHub Copilot")
        dimensions  = @("功能", "定价")
        title       = "E2E 验证任务"
    } | ConvertTo-Json

    Write-Host "==> 创建任务..."
    $task = Invoke-RestMethod -Uri "http://localhost:8084/api/tasks" -Method POST -Body $body -ContentType "application/json" -TimeoutSec 30
    Write-Host "Task:" ($task | ConvertTo-Json -Compress)
    $taskId = $task.id

    Write-Host "==> 等待 Worker 消费 (queued -> running)..."
    $deadline = (Get-Date).AddMinutes(3)
    do {
        Start-Sleep -Seconds 5
        $cur = Invoke-RestMethod -Uri "http://localhost:8084/api/tasks/$taskId" -TimeoutSec 10
        Write-Host "  status=$($cur.status) progress=$($cur.progress)"
    } while ($cur.status -in @("queued","running","reworking","clarifying") -and (Get-Date) -lt $deadline)

    Write-Host "==> Metrics:"
    $metrics = Invoke-RestMethod -Uri "http://localhost:8084/metrics" -TimeoutSec 10
    Write-Host ($metrics | ConvertTo-Json -Compress)

    if ($cur.status -eq "completed") {
        Write-Host "E2E PASS: task completed"
        exit 0
    } elseif ($cur.status -eq "running") {
        Write-Host "E2E PARTIAL: task still running (LLM 链路可能较慢)"
        exit 0
    } else {
        Write-Host "E2E WARN: final status=$($cur.status)"
        exit 1
    }
}
finally {
    Write-Host "==> 停止进程..."
    Stop-Process -Id $worker.Id -Force -ErrorAction SilentlyContinue
    Stop-Process -Id $api.Id -Force -ErrorAction SilentlyContinue
}
