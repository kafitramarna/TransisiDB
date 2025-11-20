# TransisiDB Cleanup Script
# Stop all services and clean up

Write-Host "🧹 Cleaning up TransisiDB..." -ForegroundColor Cyan

# Stop any running jobs (API server)
Write-Host "Stopping background jobs..." -ForegroundColor Yellow
Get-Job | Where-Object { $_.Command -like "*transisidb*" } | Stop-Job
Get-Job | Where-Object { $_.Command -like "*transisidb*" } | Remove-Job
Write-Host "✓ Jobs stopped" -ForegroundColor Green

# Stop Docker containers
Write-Host "Stopping Docker containers..." -ForegroundColor Yellow
docker-compose down

if ($LASTEXITCODE -eq 0) {
    Write-Host "✓ Docker containers stopped" -ForegroundColor Green
} else {
    Write-Host "⚠ Could not stop Docker (might not be running)" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "✅ Cleanup complete!" -ForegroundColor Green
Write-Host ""
Write-Host "To remove volumes as well (reset database), run:" -ForegroundColor Gray
Write-Host "  docker-compose down -v" -ForegroundColor Gray
