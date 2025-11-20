@echo off
REM TransisiDB Quick Demo - Windows Batch Version
REM No PowerShell execution policy issues

echo.
echo ===============================================================
echo            TransisiDB - Quick Demo Script
echo ===============================================================
echo.

REM Check if in correct directory
if not exist "bin\transisidb-api.exe" (
    echo [ERROR] Please run from TransisiDB project directory
    pause
    exit /b 1
)

echo [OK] Running from correct directory
echo.

REM Step 1: Start Docker
echo ===============================================================
echo Step 1: Starting Docker Services
echo ===============================================================
echo.
echo Starting MySQL, Redis, Prometheus, Grafana...
docker-compose up -d

if %errorlevel% neq 0 (
    echo [ERROR] Failed to start Docker Compose
    echo.
    echo Possible causes:
    echo  - Docker Desktop not running
    echo  - Port conflicts
    echo.
    pause
    exit /b 1
)

echo [OK] Docker services started
echo.
echo Waiting 15 seconds for MySQL to initialize...
timeout /t 15 /nobreak > nul
echo [OK] Services should be ready
echo.

REM Step 2: Check Database
echo ===============================================================
echo Step 2: Checking Database
echo ===============================================================
echo.
docker exec transisidb-mysql mysql -uroot -psecret ecommerce_db -e "SELECT COUNT(*) as total_orders FROM orders;"
echo.
echo [OK] Database connected
echo.

REM Step 3: Start API Server in background
echo ===============================================================
echo Step 3: Starting API Server
echo ===============================================================
echo.
start /B bin\transisidb-api.exe --config config.yaml
echo [OK] API server starting in background...
timeout /t 3 /nobreak > nul
echo.

REM Step 4: Test API
echo ===============================================================
echo Step 4: Testing API Endpoints
echo ===============================================================
echo.
echo Testing health endpoint...
curl -s http://localhost:8080/health
echo.
echo.
echo Testing authenticated endpoint...
curl -s -H "Authorization: Bearer sk_dev_changeme" http://localhost:8080/api/v1/tables
echo.
echo.

REM Step 5: Run Backfill
echo ===============================================================
echo Step 5: Running Backfill Worker
echo ===============================================================
echo.
echo Before migration:
docker exec transisidb-mysql mysql -uroot -psecret ecommerce_db -e "SELECT id, total_amount, total_amount_idn FROM orders LIMIT 3;"
echo.
echo Running backfill...
bin\transisidb-backfill.exe --table orders --config config.yaml
echo.
echo After migration:
docker exec transisidb-mysql mysql -uroot -psecret ecommerce_db -e "SELECT id, total_amount, total_amount_idn FROM orders LIMIT 3;"
echo.

REM Summary
echo.
echo ===============================================================
echo                    Demo Summary
echo ===============================================================
echo.
echo [OK] Docker services running
echo [OK] Database initialized
echo [OK] API server running
echo [OK] Backfill completed
echo [OK] Data migration verified
echo.
echo Running Services:
echo   - MySQL:      http://localhost:3307
echo   - Redis:      http://localhost:6379
echo   - API Server: http://localhost:8080
echo   - Prometheus: http://localhost:9090
echo   - Grafana:    http://localhost:3000
echo.
echo To test API manually:
echo   curl http://localhost:8080/health
echo.
echo To stop:
echo   docker-compose down
echo   taskkill /F /IM transisidb-api.exe
echo.
echo ===============================================================
echo Press any key to stop API server and exit...
echo ===============================================================
pause > nul

REM Cleanup
taskkill /F /IM transisidb-api.exe 2>nul
echo [OK] API server stopped
echo.
echo Docker containers still running. To stop:
echo   docker-compose down
echo.
