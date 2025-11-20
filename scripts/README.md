# TransisiDB Demo Scripts

Scripts untuk automated testing dan demo.

## 🚀 Quick Start

### Run Full Demo (Recommended)

```powershell
# Make sure Docker Desktop is running first!
.\scripts\demo.ps1
```

Script ini akan otomatis:
1. ✅ Check Docker status
2. ✅ Start semua services (MySQL, Redis, Prometheus, Grafana)
3. ✅ Verify database initialization
4. ✅ Test API server (health check, authentication)
5. ✅ Run backfill worker
6. ✅ Verify data migration
7. ✅ Test Banker's Rounding
8. ✅ Show summary & running services

### Run Quick Demo (Skip Backfill)

```powershell
.\scripts\demo.ps1 -Quick
```

### Run Without Docker

```powershell
.\scripts\demo.ps1 -SkipDocker
```

## 🧹 Cleanup

Stop semua services:

```powershell
.\scripts\cleanup.ps1
```

Reset database (remove volumes):

```powershell
docker-compose down -v
```

## 📋 Manual Testing

Jika ingin test manual step-by-step, lihat:
- **TESTING_GUIDE.md** - Panduan lengkap testing manual

## 🎯 What Gets Tested

Script demo akan test:
- ✅ Docker infrastructure
- ✅ MySQL database & schema
- ✅ Redis connection
- ✅ API server endpoints
- ✅ Authentication
- ✅ Backfill worker
- ✅ Data conversion (BIGINT → DECIMAL)
- ✅ Banker's Rounding

## 📊 Expected Output

Demo script akan show:
- Colored output (Green = success, Yellow = warning, Red = error)
- Progress untuk setiap step
- Test results dengan actual data
- Summary of running services
- Cleanup instructions

## ⚠️ Prerequisites

- Docker Desktop (running)
- PowerShell
- TransisiDB sudah di-build (`bin/` directory ada)

## 🐛 Troubleshooting

**Docker tidak running:**
```
⚠ Docker Desktop is not running!
```
→ Start Docker Desktop dulu, tunggu sampai ready

**Port already in use:**
```
Error: ... port is already allocated
```
→ Run cleanup script, atau check port conflicts

**Permission denied:**
```
cannot be loaded because running scripts is disabled
```
→ Run PowerShell as Administrator:
```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
```

## 📖 More Info

- Full testing guide: `TESTING_GUIDE.md`
- API documentation: `docs/API.md`
- Project summary: `PROJECT_SUMMARY.md`
