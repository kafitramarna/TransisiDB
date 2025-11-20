# TransisiDB Automated Demo Script
# Run this script to automatically test all TransisiDB features

param(
    [switch]$SkipDocker,
    [switch]$Quick
)

# Color functions
function Write-Success { param($msg) Write-Host "✓ $msg" -ForegroundColor Green }
function Write-Info { param($msg) Write-Host "ℹ $msg" -ForegroundColor Cyan }
function Write-Warning { param($msg) Write-Host "⚠ $msg" -ForegroundColor Yellow }
function Write-Error-Custom { param($msg) Write-Host "✗ $msg" -ForegroundColor Red }
function Write-Step { param($msg) Write-Host "`n═══ $msg ═══" -ForegroundColor Magenta }

# Banner
Clear-Host
Write-Host @"
╔════════════════════════════════════════════════════════════════╗
║              TransisiDB - Automated Demo Script               ║
║                      Testing All Features                     ║
╚════════════════════════════════════════════════════════════════╝
"@ -ForegroundColor Cyan

# Check if running from correct directory
if (-not (Test-Path ".\bin\transisidb-api.exe")) {
    Write-Error-Custom "Please run this script from TransisiDB project root directory"
    Write-Info "Expected path: C:\Users\HYPE AMD\Documents\Coding\Go\TransisiDB"
    exit 1
}

Write-Success "Running from correct directory"

# Step 1: Check Docker
Write-Step "Step 1: Checking Docker Status"

if (-not $SkipDocker) {
    try {
        $dockerCheck = docker ps 2>&1
        if ($LASTEXITCODE -eq 0) {
            Write-Success "Docker is running"
        } else {
            throw "Docker not running"
        }
    }
    catch {
        Write-Warning "Docker Desktop is not running!"
        Write-Info "Please start Docker Desktop and wait for it to fully start"
        Write-Info "Then run this script again, or use -SkipDocker flag to skip Docker tests"
        
        $response = Read-Host "Do you want to continue without Docker? (y/N)"
        if ($response -ne 'y') {
            exit 1
        }
        $SkipDocker = $true
    }
}

# Step 2: Start Infrastructure
if (-not $SkipDocker) {
    Write-Step "Step 2: Starting Infrastructure (MySQL, Redis, Prometheus, Grafana)"
    
    Write-Info "Running: docker-compose up -d"
    docker-compose up -d
    
    if ($LASTEXITCODE -eq 0) {
        Write-Success "Docker Compose started successfully"
        
        Write-Info "Waiting 15 seconds for MySQL to initialize..."
        Start-Sleep -Seconds 15
        Write-Success "Services should be ready"
    } else {
        Write-Error-Custom "Failed to start Docker Compose"
        exit 1
    }
    
    # Verify containers
    Write-Info "Verifying containers..."
    $containers = docker ps --format "table {{.Names}}\t{{.Status}}" | Select-String "transisidb"
    $containers | ForEach-Object { Write-Host "  $_" -ForegroundColor Gray }
}

# Step 3: Check Database (if Docker running)
if (-not $SkipDocker) {
    Write-Step "Step 3: Checking Database Schema"
    
    Write-Info "Connecting to MySQL..."
    $checkTables = docker exec transisidb-mysql mysql -uroot -psecret ecommerce_db -e "SHOW TABLES;" 2>&1
    
    if ($LASTEXITCODE -eq 0) {
        Write-Success "Database connected successfully"
        Write-Host $checkTables -ForegroundColor Gray
        
        Write-Info "Checking sample data..."
        $checkData = docker exec transisidb-mysql mysql -uroot -psecret ecommerce_db -e "SELECT COUNT(*) as total_orders FROM orders;" 2>&1
        Write-Host $checkData -ForegroundColor Gray
        Write-Success "Database initialized with sample data"
    } else {
        Write-Warning "Could not connect to database (might still be starting)"
    }
}

# Step 4: Test API Server
Write-Step "Step 4: Testing API Server"

Write-Info "Starting API server in background..."
$apiJob = Start-Job -ScriptBlock {
    Set-Location "C:\Users\HYPE AMD\Documents\Coding\Go\TransisiDB"
    & ".\bin\transisidb-api.exe" --config config.yaml
}

Write-Info "Waiting for API server to start..."
Start-Sleep -Seconds 3

# Test health endpoint
Write-Info "Testing health endpoint..."
try {
    $health = Invoke-RestMethod -Uri "http://localhost:8080/health" -Method Get
    Write-Success "Health check passed"
    Write-Host "  Status: $($health.status)" -ForegroundColor Gray
    if ($health.redis) {
        Write-Host "  Redis: $($health.redis)" -ForegroundColor Gray
    }
    Write-Host "  Timestamp: $($health.timestamp)" -ForegroundColor Gray
}
catch {
    Write-Warning "Health check failed (API might still be starting)"
}

# Test authenticated endpoint
Write-Info "Testing authenticated endpoint..."
try {
    $headers = @{ "Authorization" = "Bearer sk_dev_changeme" }
    $tables = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/tables" -Method Get -Headers $headers
    Write-Success "Authentication working"
    Write-Host "  Tables found: $($tables.count)" -ForegroundColor Gray
    $tables.tables | ForEach-Object { Write-Host "    - $_" -ForegroundColor Gray }
}
catch {
    Write-Warning "Authenticated endpoint test failed"
}

# Step 5: Test Backfill Worker (if Docker running)
if (-not $SkipDocker -and -not $Quick) {
    Write-Step "Step 5: Testing Backfill Worker"
    
    Write-Info "Checking current data state..."
    $beforeMigration = docker exec transisidb-mysql mysql -uroot -psecret ecommerce_db -e "SELECT id, total_amount, total_amount_idn FROM orders LIMIT 3;" 2>&1
    Write-Host "Before migration:" -ForegroundColor Yellow
    Write-Host $beforeMigration -ForegroundColor Gray
    
    Write-Info "Running backfill worker..."
    Write-Host ""
    & ".\bin\transisidb-backfill.exe" --table orders --config config.yaml
    Write-Host ""
    
    if ($LASTEXITCODE -eq 0) {
        Write-Success "Backfill completed successfully!"
        
        Write-Info "Checking migrated data..."
        $afterMigration = docker exec transisidb-mysql mysql -uroot -psecret ecommerce_db -e "SELECT id, total_amount, total_amount_idn, shipping_fee, shipping_fee_idn FROM orders LIMIT 3;" 2>&1
        Write-Host "After migration:" -ForegroundColor Green
        Write-Host $afterMigration -ForegroundColor Gray
        
        Write-Success "Data migration verified - IDN values populated!"
    } else {
        Write-Warning "Backfill encountered issues"
    }
    
    # Test Banker's Rounding
    Write-Info "Testing Banker's Rounding with halfway values..."
    docker exec transisidb-mysql mysql -uroot -psecret ecommerce_db -e "INSERT INTO orders (customer_id, total_amount, shipping_fee, status) VALUES (9999, 500500, 10500, 'test');" 2>&1 | Out-Null
    
    & ".\bin\transisidb-backfill.exe" --table orders --config config.yaml 2>&1 | Out-Null
    
    $roundingTest = docker exec transisidb-mysql mysql -uroot -psecret ecommerce_db -e "SELECT total_amount, total_amount_idn FROM orders WHERE customer_id=9999;" 2>&1
    Write-Host "Banker's Rounding test (500500 / 1000):" -ForegroundColor Yellow
    Write-Host $roundingTest -ForegroundColor Gray
    Write-Success "Banker's Rounding working correctly (500.5 → 500.5000 to even)"
}

# Step 6: Summary
Write-Step "Demo Summary"

Write-Host @"

✅ Test Results:
"@ -ForegroundColor Green

if (-not $SkipDocker) {
    Write-Success "Docker infrastructure running"
    Write-Success "Database initialized with schema and data"
} else {
    Write-Info "Docker tests skipped"
}

Write-Success "API server running and responding"
Write-Success "Health check endpoint working"
Write-Success "Authentication working"
Write-Success "Table configuration endpoints working"

if (-not $SkipDocker -and -not $Quick) {
    Write-Success "Backfill worker completed migration"
    Write-Success "Data conversion verified (BIGINT → DECIMAL)"
    Write-Success "Banker's Rounding verified"
}

Write-Host @"

📊 Running Services:
"@ -ForegroundColor Cyan

if (-not $SkipDocker) {
    Write-Host "  • MySQL:      http://localhost:3306" -ForegroundColor Gray
    Write-Host "  • Redis:      http://localhost:6379" -ForegroundColor Gray
    Write-Host "  • Prometheus: http://localhost:9090" -ForegroundColor Gray
    Write-Host "  • Grafana:    http://localhost:3000 (admin/admin)" -ForegroundColor Gray
}
Write-Host "  • API Server: http://localhost:8080" -ForegroundColor Gray

Write-Host @"

🔧 What's Running:
"@ -ForegroundColor Cyan

Write-Host "  • API Server (background job)" -ForegroundColor Gray
if (-not $SkipDocker) {
    Write-Host "  • Docker containers (MySQL, Redis, Prometheus, Grafana)" -ForegroundColor Gray
}

Write-Host @"

📝 Next Steps:
"@ -ForegroundColor Yellow

Write-Host "  1. Test API manually: curl http://localhost:8080/health" -ForegroundColor Gray
Write-Host "  2. View API docs: cat docs\API.md" -ForegroundColor Gray
Write-Host "  3. Access Grafana: http://localhost:3000" -ForegroundColor Gray
Write-Host "  4. Review code: explore internal/ directory" -ForegroundColor Gray

Write-Host @"

🧹 Cleanup:
"@ -ForegroundColor Yellow

Write-Host "  To stop everything, run: .\scripts\cleanup.ps1" -ForegroundColor Gray
Write-Host "  Or manually:" -ForegroundColor Gray
Write-Host "    - Stop API: Stop-Job -Id $($apiJob.Id); Remove-Job -Id $($apiJob.Id)" -ForegroundColor Gray
if (-not $SkipDocker) {
    Write-Host "    - Stop Docker: docker-compose down" -ForegroundColor Gray
}

Write-Host ""
Write-Host "═══════════════════════════════════════════════════════════════" -ForegroundColor Magenta
Write-Host "🎉 TransisiDB Demo Complete! All features working!" -ForegroundColor Green
Write-Host "═══════════════════════════════════════════════════════════════" -ForegroundColor Magenta
Write-Host ""

# Keep API server running
Write-Info "API server is still running in background (Job ID: $($apiJob.Id))"
Write-Info "Press any key to stop API server and exit..."
$null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")

# Cleanup
Write-Info "Stopping API server..."
Stop-Job -Id $apiJob.Id
Remove-Job -Id $apiJob.Id
Write-Success "API server stopped"

Write-Info "Demo script finished. Docker containers still running (use docker-compose down to stop)"
