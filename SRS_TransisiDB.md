# DOKUMEN SPESIFIKASI KEBUTUHAN SISTEM
# TRANSISIDB: INTELLIGENT DATABASE PROXY FOR CURRENCY REDENOMINATION

**Versi Dokumen:** 1.0  
**Tanggal:** 20 November 2025  
**Status:** Draft untuk Review  
**Confidentiality:** Internal Use  

---

## DAFTAR ISI

1. [Pendahuluan](#1-pendahuluan)
   - 1.1 Tujuan Dokumen
   - 1.2 Lingkup Masalah
   - 1.3 Definisi, Akronim, dan Singkatan
   - 1.4 Referensi
2. [Deskripsi Umum](#2-deskripsi-umum)
   - 2.1 Perspektif Produk
   - 2.2 Karakteristik Pengguna
   - 2.3 Batasan Sistem
   - 2.4 Asumsi dan Ketergantungan
3. [Spesifikasi Kebutuhan Fungsional](#3-spesifikasi-kebutuhan-fungsional)
4. [Spesifikasi Data](#4-spesifikasi-data)
5. [Antarmuka Eksternal](#5-antarmuka-eksternal)
6. [Kebutuhan Non-Fungsional](#6-kebutuhan-non-fungsional)
7. [Arsitektur Teknis](#7-arsitektur-teknis)
8. [Roadmap Pengembangan](#8-roadmap-pengembangan)

---

## 1. PENDAHULUAN

### 1.1 Tujuan Dokumen

Dokumen Spesifikasi Kebutuhan Sistem (System Requirements Specification - SRS) ini mendefinisikan seluruh kebutuhan fungsional, non-fungsional, arsitektural, dan operasional untuk pengembangan **TransisiDB** — sebuah intelligent database proxy middleware yang dirancang untuk menangani migrasi tipe data mata uang dari representasi integer (BIGINT/INTEGER) ke desimal (DECIMAL) secara zero-downtime dalam konteks redenominasi mata uang Rupiah Indonesia 2027.

Dokumen ini ditujukan untuk:
- **Tim Pengembangan**: Sebagai blueprint teknis implementasi
- **Stakeholder Bisnis**: Untuk memahami cakupan dan value proposition produk
- **Quality Assurance**: Sebagai basis test case dan acceptance criteria
- **Technical Writers**: Untuk pembuatan dokumentasi eksternal dan user guide

### 1.2 Lingkup Masalah

#### 1.2.1 Konteks Bisnis

Pemerintah Indonesia telah mengumumkan rencana **redenominasi mata uang Rupiah** yang dijadwalkan berlaku pada tahun 2027, di mana nilai Rp 1.000 akan menjadi Rp 1 (rasio konversi 1:1000). Perubahan ini memerlukan penanganan presisi desimal untuk pecahan mata uang (sen), yang sebelumnya tidak diperlukan dalam sistem Rupiah saat ini.

#### 1.2.2 Tantangan Teknis

Sebagian besar sistem legacy di Indonesia menyimpan nilai mata uang dalam database menggunakan tipe data **BIGINT** atau **INTEGER** (dalam satuan Rupiah terkecil, tanpa desimal). Tantangan utama yang dihadapi:

1. **Schema Migration Risk**
   - Mengubah tipe kolom dari `BIGINT` ke `DECIMAL(19,4)` pada tabel dengan miliaran baris membutuhkan `ALTER TABLE` yang dapat menyebabkan **table-level locking** selama berjam-jam
   - Risiko downtime yang tidak dapat diterima untuk aplikasi mission-critical (e-commerce, perbankan, ERP)
   - Potensi data inconsistency jika migrasi gagal di tengah jalan

2. **Application Code Refactoring**
   - Jutaan baris kode aplikasi yang melakukan kalkulasi, validasi, dan formatting nilai integer harus diubah
   - Risiko regression bugs yang tinggi
   - Effort dan biaya pengembangan yang substansial

3. **Data Precision & Compliance**
   - Pembulatan nilai harus mengikuti standar akuntansi internasional (ISO 4217, GAAP)
   - Harus mendukung **Banker's Rounding** (Round Half to Even) untuk menghindari bias akumulatif
   - Audit trail untuk setiap transformasi nilai

4. **Backward Compatibility**
   - Sistem harus tetap dapat melayani request dalam format lama (IDR integer) dan format baru (IDN decimal) secara simultan selama periode transisi

#### 1.2.3 Solusi: TransisiDB

TransisiDB adalah **transparent database proxy** yang duduk di antara application layer dan database layer, bertindak sebagai:
- **Translation Layer**: Mengonversi query dan hasil secara real-time
- **Dual-Write Orchestrator**: Menulis data ke kolom lama dan kolom baru secara atomik
- **Background Migration Engine**: Memigrasikan data historis tanpa mengganggu operasional
- **Simulation Platform**: Memungkinkan testing aplikasi dengan data redenominasi sebelum go-live

### 1.3 Definisi, Akronim, dan Singkatan

| Istilah | Definisi |
|---------|----------|
| **IDR** | Indonesian Rupiah — kode mata uang ISO 4217 untuk Rupiah sebelum redenominasi |
| **IDN** | Indonesian Rupiah (New) — kode mata uang yang akan digunakan pasca-redenominasi |
| **Redenominasi** | Penyederhanaan nilai nominal mata uang dengan mengurangi jumlah digit (Rp 1.000 → Rp 1) |
| **BIGINT** | Tipe data integer 64-bit (-9,223,372,036,854,775,808 hingga 9,223,372,036,854,775,807) |
| **DECIMAL(p,s)** | Tipe data numerik presisi tetap dengan p = total digit, s = digit di belakang koma |
| **Dual-Write** | Strategi menulis data secara simultan ke dua lokasi (kolom lama + kolom baru) |
| **Shadow Column** | Kolom baru yang ditambahkan ke tabel untuk menyimpan nilai terkonversi |
| **Banker's Rounding** | Algoritma pembulatan IEEE 754 yang membulatkan nilai tengah (.5) ke angka genap terdekat |
| **Wire Protocol** | Protokol komunikasi low-level antara database client dan server (contoh: MySQL Protocol) |
| **TPS** | Transactions Per Second — metrik throughput database |
| **Backfill** | Proses mengisi data historis yang belum termigrasi |
| **Fail-Open** | Strategi fallback yang membiarkan traffic lewat tanpa transformasi jika proxy gagal |

### 1.4 Referensi

- ISO 4217:2015 — Codes for the representation of currencies
- IEEE 754-2008 — IEEE Standard for Floating-Point Arithmetic
- MySQL 8.0 Protocol Documentation
- PostgreSQL Wire Protocol v3.0
- RFC 7231 — HTTP/1.1 Semantics and Content
- GAAP Financial Reporting Standards

---

## 2. DESKRIPSI UMUM

### 2.1 Perspektif Produk

TransisiDB diposisikan sebagai **transparent middleware layer** yang menyediakan abstraksi untuk transformasi data mata uang tanpa memerlukan perubahan kode aplikasi. Produk ini bukan merupakan database management system baru, melainkan sebuah **intelligent proxy** yang kompatibel dengan database relasional standar.

#### 2.1.1 System Context Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                     APPLICATION LAYER                        │
│  (Legacy Apps, Microservices, APIs, Admin Panels)           │
└────────────────────────┬────────────────────────────────────┘
                         │ SQL Queries (MySQL/PostgreSQL Protocol)
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                      TRANSISIDB PROXY                        │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │ Query Parser │  │ Dual-Write   │  │ Rounding Engine  │  │
│  │ & Interceptor│─▶│ Orchestrator │─▶│ (Banker's Round) │  │
│  └──────────────┘  └──────────────┘  └──────────────────┘  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │ Config Store │  │ Time Travel  │  │ Backfill Worker  │  │
│  │ (Redis)      │  │ Simulator    │  │ (Background)     │  │
│  └──────────────┘  └──────────────┘  └──────────────────┘  │
└────────────────────────┬────────────────────────────────────┘
                         │ Modified SQL + Dual-Write
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                    DATABASE LAYER                            │
│         (MySQL, PostgreSQL, MariaDB)                         │
│  Kolom Lama: amount BIGINT  +  Kolom Baru: amount_idn DECIMAL│
└─────────────────────────────────────────────────────────────┘
```

#### 2.1.2 Value Proposition

| Manfaat | Deskripsi |
|---------|-----------|
| **Zero-Downtime Migration** | Migrasi tipe data tanpa table locking atau aplikasi downtime |
| **Backward Compatible** | Mendukung query format lama (BIGINT) dan baru (DECIMAL) secara simultan |
| **Non-Invasive** | Tidak memerlukan refactoring aplikasi secara masif |
| **Compliance-Ready** | Built-in Banker's Rounding sesuai standar ISO/GAAP |
| **Testing-Friendly** | Simulation mode untuk UAT tanpa merusak data produksi |
| **Cost-Effective** | Mengurangi effort engineering dari bulan menjadi minggu |

### 2.2 Karakteristik Pengguna

#### 2.2.1 Primary Users

| Persona | Karakteristik | Kebutuhan Utama |
|---------|--------------|-----------------|
| **CTO / VP Engineering** | - Decision maker teknis<br>- Background computer science<br>- Concern: business continuity, cost, risk | - Proof of zero-downtime<br>- ROI calculation<br>- Risk mitigation plan |
| **Database Administrator (DBA)** | - Expert database tuning<br>- Access production DB<br>- Concern: performance, data integrity | - Monitoring dashboard<br>- Rollback mechanism<br>- Query performance metrics |
| **Backend Engineer** | - Maintain business logic<br>- Familiar dengan ORM/SQL<br>- Concern: API compatibility | - API documentation<br>- SDK/library untuk integrasi<br>- Testing tools |
| **QA Engineer** | - Responsible untuk testing<br>- Knowledge SQL queries<br>- Concern: data accuracy | - Simulation mode access<br>- Test case generator<br>- Regression testing tools |

#### 2.2.2 Secondary Users

- **Compliance Officer**: Membutuhkan audit trail dan reportability
- **DevOps Engineer**: Membutuhkan deployment automation dan monitoring
- **Finance Team**: Memvalidasi presisi kalkulasi akuntansi

### 2.3 Batasan Sistem

1. **Database Support**: Versi awal mendukung MySQL 5.7+, PostgreSQL 11+, MariaDB 10.3+
2. **Data Type Scope**: Fokus pada transformasi tipe numerik untuk kolom mata uang
3. **Transaction Size**: Optimized untuk OLTP workload (< 1000 rows per transaction)
4. **Geographic**: Desain awal untuk zona waktu WIB (UTC+7)
5. **Language**: Pesan error dan logging dalam Bahasa Inggris

### 2.4 Asumsi dan Ketergantungan

**Asumsi:**
- Aplikasi client menggunakan standar SQL (bukan proprietary query syntax)
- Database target memiliki resource cukup untuk menambahkan shadow columns
- Periode transisi (dual-mode) maksimal 2 tahun

**Ketergantungan:**
- Redis 6.0+ untuk configuration store
- Prometheus + Grafana untuk monitoring (optional tapi recommended)
- Minimal 2 vCPU dan 4GB RAM untuk deployment proxy

---

## 3. SPESIFIKASI KEBUTUHAN FUNGSIONAL

### FR-01: Mekanisme Dual-Write Atomik

**Prioritas:** CRITICAL  
**Status:** MUST HAVE  

#### 3.1.1 Deskripsi

Proxy harus mampu mencegat setiap operasi `INSERT`, `UPDATE`, dan `DELETE` yang melibatkan kolom mata uang, kemudian secara otomatis:
1. Mengekstrak nilai dari query original
2. Menghitung nilai terkonversi (dibagi 1000 untuk redenominasi IDR → IDN)
3. Menulis **kedua nilai** (original + converted) dalam **satu transaksi atomik**
4. Memastikan konsistensi data bahkan jika terjadi kegagalan di tengah proses

#### 3.1.2 Flow Diagram

```
[App] INSERT INTO orders (customer_id, total_amount) VALUES (123, 500000);
   │
   ▼
[TransisiDB Proxy]
   │
   ├─▶ Parse Query
   │   └─▶ Detect kolom 'total_amount' sebagai currency column (via config)
   │
   ├─▶ Transform Query
   │   └─▶ INSERT INTO orders (customer_id, total_amount, total_amount_idn)
   │       VALUES (123, 500000, 500.0000);
   │       -- Konversi: 500000 / 1000 = 500.0000 IDN
   │
   ├─▶ Execute in Transaction
   │   BEGIN;
   │   └─▶ [Database Execute]
   │   COMMIT;
   │
   └─▶ Return Response ke App
       ✓ OK (1 row affected)
```

#### 3.1.3 Technical Specifications

| Aspect | Specification |
|--------|---------------|
| **Transaction Isolation** | `READ COMMITTED` (default), configurable hingga `SERIALIZABLE` |
| **Atomicity Guarantee** | Gunakan database-native transaction; rollback jika salah satu write gagal |
| **Query Rewrite Engine** | Parse SQL AST menggunakan library `vitess/sqlparser` (Go) atau `sqlparse` (Python) |
| **Shadow Column Naming** | Convention: `{original_column}_idn` (contoh: `price` → `price_idn`) |
| **Conversion Formula** | `new_value = FLOOR(old_value / 1000 * 10000) / 10000` (presisi 4 desimal) |
| **Error Handling** | Jika conversion gagal, batalkan seluruh transaksi dan return error ke client |

#### 3.1.4 Acceptance Criteria

- **AC-01.1**: Setiap `INSERT` yang berisi kolom mata uang menghasilkan 2 nilai (IDR + IDN) di database
- **AC-01.2**: Jika shadow column `amount_idn` sudah ada value, system tidak overwrite (idempotent)
- **AC-01.3**: Jika terjadi deadlock atau constraint violation, transaksi rollback sepenuhnya
- **AC-01.4**: Latency overhead maksimal 5ms untuk query dengan 1-10 rows

---

### FR-02: Strategi Pembulatan (Banker's Rounding Engine)

**Prioritas:** HIGH  
**Status:** MUST HAVE  

#### 3.2.1 Rasionalisasi

Dalam akuntansi, pembulatan nilai uang harus menghindari **systematic bias**. Pembulatan aritmatika standar (0.5 dibulatkan ke atas) menyebabkan akumulasi error positif dalam volume transaksi besar. **Banker's Rounding** (IEEE 754 Round Half to Even) mengeliminasi bias ini dengan membulatkan nilai tengah ke angka genap terdekat.

#### 3.2.2 Algoritma Implementasi

```go
func BankersRound(value float64, precision int) float64 {
    multiplier := math.Pow(10, float64(precision))
    adjusted := value * multiplier
    
    floor := math.Floor(adjusted)
    ceil := math.Ceil(adjusted)
    fraction := adjusted - floor
    
    if fraction < 0.5 {
        return floor / multiplier
    } else if fraction > 0.5 {
        return ceil / multiplier
    } else {
        // Fraction == 0.5: bulatkan ke genap
        if int(floor) % 2 == 0 {
            return floor / multiplier
        } else {
            return ceil / multiplier
        }
    }
}
```

#### 3.2.3 Contoh Kasus

| Nilai IDR (BIGINT) | Hasil Bagi 1000 | Banker's Round (4 desimal) | Arithmetic Round |
|--------------------|-----------------|----------------------------|------------------|
| 1234567            | 1234.567        | 1234.5670                  | 1234.5670        |
| 9876543            | 9876.543        | 9876.5430                  | 9876.5430        |
| 500500             | 500.500         | **500.5000** (genap)       | 500.5000         |
| 501500             | 501.500         | **501.5000** (genap)       | 501.5000         |
| 500000             | 500.000         | 500.0000                   | 500.0000         |

#### 3.2.4 Configuration

Sistem harus mendukung pemilihan strategi rounding per-kolom:

```json
{
  "tables": {
    "orders": {
      "columns": {
        "total_amount": {
          "rounding_strategy": "BANKERS_ROUND",
          "precision": 4
        },
        "tax_amount": {
          "rounding_strategy": "ARITHMETIC_ROUND",
          "precision": 2
        }
      }
    }
  }
}
```

#### 3.2.5 Acceptance Criteria

- **AC-02.1**: 100% compliance dengan IEEE 754 Round Half to Even
- **AC-02.2**: Benchmark: 1 juta operasi rounding dalam < 100ms
- **AC-02.3**: Unit test dengan 1000+ edge cases (0.5, negative numbers, extreme values)

---

### FR-03: Intelligent Backfill (Background Data Migration)

**Prioritas:** HIGH  
**Status:** MUST HAVE  

#### 3.3.1 Deskripsi

Worker service terpisah yang berjalan di background untuk mengisi nilai shadow column (`amount_idn`) untuk data historis yang belum terkonversi, tanpa membebani performa database produktif.

#### 3.3.2 Algoritma Backfill

```
1. Query batch kecil dari tabel target (contoh: 1000 rows)
   WHERE amount_idn IS NULL
   ORDER BY id ASC
   LIMIT 1000;

2. Untuk setiap row:
   - Hitung new_value = BANKERS_ROUND(amount / 1000, 4)
   - UPDATE SET amount_idn = new_value WHERE id = ?

3. Sleep interval (contoh: 100ms) untuk menghindari resource contention

4. Log progress: "Backfilled 50,000 / 10,000,000 rows (0.5%)"

5. Ulangi hingga semua rows termigrasi
```

#### 3.3.3 Technical Specifications

| Parameter | Value | Konfigurasi |
|-----------|-------|-------------|
| **Batch Size** | 1000 rows | Configurable via env var `BACKFILL_BATCH_SIZE` |
| **Sleep Interval** | 100ms | Configurable via env var `BACKFILL_SLEEP_MS` |
| **Max CPU Usage** | 20% | Auto-throttle jika CPU > threshold |
| **Retry Logic** | 3x dengan exponential backoff | Jika UPDATE gagal karena lock timeout |
| **Idempotency** | Skip rows jika `amount_idn IS NOT NULL` | |

#### 3.3.4 Monitoring Metrics

- **backfill_progress_percentage**: Persentase rows termigrasi
- **backfill_rows_per_second**: Throughput migrasi
- **backfill_errors_total**: Counter error yang terjadi
- **backfill_estimated_completion_time**: ETA berdasarkan throughput saat ini

#### 3.3.5 Acceptance Criteria

- **AC-03.1**: Backfill 1 juta rows dalam < 30 menit dengan throttling default
- **AC-03.2**: CPU usage tidak melebihi 25% selama proses backfill
- **AC-03.3**: Dapat pause/resume via API tanpa data corruption
- **AC-03.4**: Automated retry untuk transient errors (deadlock, connection timeout)

---

### FR-04: Time Travel / Simulation Mode

**Prioritas:** MEDIUM  
**Status:** SHOULD HAVE (Value Add untuk Portfolio)  

#### 3.4.1 Deskripsi

Fitur "wow factor" yang memungkinkan developer dan QA engineer untuk **melihat output API seolah-olah redenominasi sudah berlaku**, tanpa mengubah data aktual di database. Berguna untuk:
- Testing UI dengan nilai IDN sebelum go-live
- UAT (User Acceptance Testing) tanpa risiko
- Demo kepada stakeholders

#### 3.4.2 Flow Diagram

```
[App Request dengan Header]
GET /api/orders/12345
Headers:
  X-TransisiDB-Mode: SIMULATE_IDN
  X-TransisiDB-Conversion-Date: 2027-01-01

   │
   ▼
[TransisiDB Proxy]
   │
   ├─▶ Detect Simulation Mode dari Header
   │
   ├─▶ Execute Query Normal ke Database
   │   SELECT id, total_amount FROM orders WHERE id = 12345;
   │   Result: { id: 12345, total_amount: 500000 }
   │
   ├─▶ Transform Response Secara Virtual
   │   {
   │     id: 12345,
   │     total_amount: 500.0000,  ← Converted on-the-fly
   │     _metadata: {
   │       simulated: true,
   │       original_value: 500000,
   │       currency: "IDN"
   │     }
   │   }
   │
   └─▶ Return ke App
```

#### 3.4.3 Technical Specifications

| Aspect | Implementation |
|--------|----------------|
| **Activation Method** | HTTP Header `X-TransisiDB-Mode: SIMULATE_IDN` |
| **Scope** | Per-request (tidak persisten) |
| **Data Transformation** | Hanya pada response, database tidak terpengaruh |
| **Performance Impact** | Overhead maksimal 2ms per request |
| **Security** | Hanya aktif jika `SIMULATION_MODE_ENABLED=true` di config |

#### 3.4.4 Use Cases

**UC-04.1: UI Testing**
```javascript
// Frontend developer testing new UI dengan nilai IDN
fetch('/api/invoices', {
  headers: {
    'X-TransisiDB-Mode': 'SIMULATE_IDN'
  }
})
.then(res => res.json())
.then(data => {
  // Data sudah dalam format IDN (500.00) bukan IDR (500000)
  renderInvoice(data);
});
```

**UC-04.2: A/B Testing UI**
```javascript
// Menampilkan versi lama dan baru side-by-side
const [idrData, idnData] = await Promise.all([
  fetch('/api/orders/123'), // Normal mode
  fetch('/api/orders/123', { 
    headers: { 'X-TransisiDB-Mode': 'SIMULATE_IDN' } 
  })
]);
```

#### 3.4.5 Acceptance Criteria

- **AC-04.1**: Response JSON dengan simulation mode menambahkan field `_metadata.simulated = true`
- **AC-04.2**: Simulation mode TIDAK menulis ke database (read-only transformation)
- **AC-04.3**: Dapat diaktifkan/dinonaktifkan via feature flag tanpa restart proxy
- **AC-04.4**: Automated E2E test yang memverifikasi consistency antara mode normal dan simulated

---

## 4. SPESIFIKASI DATA

### 4.1 Strategi Transformasi Tipe Data

#### 4.1.1 Mapping Tipe Data

| Tipe Lama | Tipe Baru | Precision | Contoh |
|-----------|-----------|-----------|---------|
| `BIGINT` | `DECIMAL(19,4)` | 4 desimal | `500000` → `500.0000` |
| `INT` | `DECIMAL(12,4)` | 4 desimal | `50000` → `50.0000` |
| `DECIMAL(19,2)` | `DECIMAL(19,4)` | 4 desimal | `500.50` → `500.5000` |

**Rasionalisasi Precision 4 Desimal:**
- ISO 4217 mengharuskan presisi minimal 2 desimal untuk minor units (sen)
- Precision 4 desimal memberikan buffer untuk kalkulasi intermediate yang memerlukan rounding bertahap (contoh: tax calculation)
- Align dengan standar financial system internasional

#### 4.1.2 DDL Migration Script

Contoh script untuk menambahkan shadow column (dijalankan oleh DBA dengan monitoring):

```sql
-- MySQL
ALTER TABLE orders 
  ADD COLUMN total_amount_idn DECIMAL(19,4) DEFAULT NULL,
  ADD INDEX idx_amount_idn_backfill (id) 
  WHERE total_amount_idn IS NULL;

-- PostgreSQL
ALTER TABLE orders 
  ADD COLUMN total_amount_idn NUMERIC(19,4) DEFAULT NULL;

CREATE INDEX CONCURRENTLY idx_amount_idn_backfill 
  ON orders(id) 
  WHERE total_amount_idn IS NULL;
```

**Catatan Penting:**
- `ADD COLUMN` dengan `DEFAULT NULL` adalah operasi metadata-only di MySQL 8.0+ (instant)
- Index `CONCURRENTLY` di PostgreSQL menghindari table lock
- Partial index hanya untuk rows yang belum di-backfill (menghemat space)

### 4.2 Skema Konfigurasi Metadata

Konfigurasi disimpan di **Redis** dalam format JSON untuk performa tinggi.

#### 4.2.1 Configuration Schema

```json
{
  "version": "1.0",
  "last_updated": "2025-11-20T14:30:00Z",
  "conversion_ratio": 1000,
  "databases": {
    "ecommerce_db": {
      "tables": {
        "orders": {
          "enabled": true,
          "columns": {
            "total_amount": {
              "source_column": "total_amount",
              "target_column": "total_amount_idn",
              "source_type": "BIGINT",
              "target_type": "DECIMAL(19,4)",
              "rounding_strategy": "BANKERS_ROUND",
              "precision": 4,
              "conversion_formula": "source / 1000"
            },
            "shipping_fee": {
              "source_column": "shipping_fee",
              "target_column": "shipping_fee_idn",
              "source_type": "INT",
              "target_type": "DECIMAL(12,4)",
              "rounding_strategy": "BANKERS_ROUND",
              "precision": 4
            }
          }
        },
        "invoices": {
          "enabled": true,
          "columns": {
            "subtotal": { /* ... */ },
            "tax": { /* ... */ },
            "grand_total": { /* ... */ }
          }
        }
      }
    }
  },
  "backfill": {
    "enabled": true,
    "batch_size": 1000,
    "sleep_interval_ms": 100,
    "max_cpu_percent": 20,
    "retry_attempts": 3
  },
  "simulation_mode": {
    "enabled": true,
    "allowed_ips": ["10.0.0.0/8", "192.168.1.0/24"]
  }
}
```

#### 4.2.2 Redis Key Structure

```
transisidb:config:version              → "1.0"
transisidb:config:databases:ecommerce_db:tables:orders → {JSON object}
transisidb:backfill:progress:orders    → "50000/10000000"
transisidb:stats:dual_write:success    → Counter
transisidb:stats:dual_write:errors     → Counter
```

### 4.3 Data Validation Rules

| Rule ID | Deskripsi | Implementasi |
|---------|-----------|--------------|
| **DV-01** | Negative values allowed (untuk refund/reversal) | Validasi: `new_value < 0` is OK |
| **DV-02** | Zero values allowed | Validasi: `new_value == 0` is OK |
| **DV-03** | Max value: 99,999,999,999.9999 | Validasi: `new_value < 1e11` |
| **DV-04** | Precision enforcement | Validasi: `DECIMAL(19,4)` truncate di 4 desimal |

---

## 5. ANTARMUKA EKSTERNAL

### 5.1 Database Wire Protocol Interface

TransisiDB harus mengimplementasikan **MySQL Wire Protocol** dan **PostgreSQL Wire Protocol** agar dapat diterima oleh aplikasi sebagai "database asli".

#### 5.1.1 Connection Flow

```
[Application] ──TCP Socket (Port 3307)──▶ [TransisiDB Proxy]
                                              │
                                              ├─▶ Parse CLIENT_HANDSHAKE
                                              ├─▶ Authenticate (forward ke real DB)
                                              ├─▶ Maintain connection pool
                                              │
                                              └─▶ [MySQL/PostgreSQL Server]
```

#### 5.1.2 Supported Commands

| Command | Support Level | Notes |
|---------|---------------|-------|
| `COM_QUERY` | Full | Parse dan transform SQL |
| `COM_PREPARE` | Full | Prepared statements |
| `COM_EXECUTE` | Full | Execute prepared statements |
| `COM_INIT_DB` | Passthrough | Tidak memerlukan transformasi |
| `COM_PING` | Passthrough | Health check |
| `COM_QUIT` | Passthrough | Close connection |

#### 5.1.3 Connection String

Aplikasi hanya perlu mengganti host dan port:

**Before (Langsung ke MySQL):**
```
mysql://user:pass@db-prod.example.com:3306/ecommerce_db
```

**After (Via TransisiDB Proxy):**
```
mysql://user:pass@transisidb-proxy.example.com:3307/ecommerce_db
```

### 5.2 Management REST API

API untuk administrasi dan monitoring proxy.

#### 5.2.1 Endpoint Specification

**Base URL:** `http://transisidb-api.example.com:8080/api/v1`

| Endpoint | Method | Deskripsi | Auth |
|----------|--------|-----------|------|
| `/config` | GET | Retrieve current configuration | API Key |
| `/config` | PUT | Update configuration | API Key |
| `/config/reload` | POST | Reload config dari Redis | API Key |
| `/backfill/start` | POST | Start backfill job | API Key |
| `/backfill/pause` | POST | Pause running backfill | API Key |
| `/backfill/status` | GET | Get backfill progress | API Key |
| `/health` | GET | Proxy health check | Public |
| `/metrics` | GET | Prometheus metrics | Public |

#### 5.2.2 API Example: Update Configuration

**Request:**
```http
PUT /api/v1/config HTTP/1.1
Host: transisidb-api.example.com:8080
Authorization: Bearer sk_live_abc123xyz
Content-Type: application/json

{
  "databases": {
    "ecommerce_db": {
      "tables": {
        "orders": {
          "enabled": true,
          "columns": {
            "total_amount": {
              "target_column": "total_amount_idn",
              "rounding_strategy": "BANKERS_ROUND"
            }
          }
        }
      }
    }
  }
}
```

**Response:**
```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "status": "success",
  "message": "Configuration updated successfully",
  "version": "1.1",
  "applied_at": "2025-11-20T14:45:00Z"
}
```

#### 5.2.3 API Example: Backfill Status

**Request:**
```http
GET /api/v1/backfill/status?table=orders HTTP/1.1
Host: transisidb-api.example.com:8080
Authorization: Bearer sk_live_abc123xyz
```

**Response:**
```json
{
  "table": "orders",
  "status": "running",
  "total_rows": 10000000,
  "completed_rows": 2500000,
  "progress_percentage": 25.0,
  "rows_per_second": 833,
  "estimated_completion": "2025-11-20T18:30:00Z",
  "errors": 12,
  "last_error": "Deadlock detected, retrying...",
  "started_at": "2025-11-20T10:00:00Z"
}
```

### 5.3 Monitoring Interface (Prometheus Metrics)

#### 5.3.1 Key Metrics

```prometheus
# HELP transisidb_dual_write_total Total number of dual-write operations
# TYPE transisidb_dual_write_total counter
transisidb_dual_write_total{status="success"} 1543892
transisidb_dual_write_total{status="error"} 127

# HELP transisidb_query_duration_seconds Query execution duration
# TYPE transisidb_query_duration_seconds histogram
transisidb_query_duration_seconds_bucket{le="0.005"} 145023
transisidb_query_duration_seconds_bucket{le="0.010"} 189453
transisidb_query_duration_seconds_bucket{le="0.050"} 198234

# HELP transisidb_backfill_progress Backfill progress percentage
# TYPE transisidb_backfill_progress gauge
transisidb_backfill_progress{table="orders"} 25.0

# HELP transisidb_connection_pool_active Active database connections
# TYPE transisidb_connection_pool_active gauge
transisidb_connection_pool_active 45
```

---

## 6. KEBUTUHAN NON-FUNGSIONAL

### NFR-01: Performance - Low Latency

**Requirement:** Overhead latency proxy maksimal **10ms** untuk 95th percentile.

**Measurement:**
- Latency tanpa proxy: `t_direct`
- Latency dengan proxy: `t_proxy`
- Overhead: `t_overhead = t_proxy - t_direct`
- Target: `P95(t_overhead) ≤ 10ms`

**Implementation Strategy:**
- Zero-copy query forwarding untuk query yang tidak memerlukan transformasi
- Connection pooling untuk menghindari overhead TCP handshake
- In-memory caching untuk configuration (Redis)
- Compiled language seperti Go atau Rust (bukan interpreted language)

**Acceptance Criteria:**
- **AC-NFR-01.1**: Benchmark dengan 10,000 SELECT queries menunjukkan P95 overhead < 10ms
- **AC-NFR-01.2**: Benchmark dengan 10,000 INSERT queries menunjukkan P95 overhead < 15ms (karena dual-write)

---

### NFR-02: Performance - High Throughput

**Requirement:** Proxy harus mampu menangani minimal **10,000 TPS** (Transactions Per Second) pada hardware standar.

**Hardware Baseline:**
- CPU: 4 vCPU (Intel Xeon atau equivalent)
- RAM: 8GB
- Network: 1 Gbps
- Disk: SSD (untuk logging)

**Acceptance Criteria:**
- **AC-NFR-02.1**: Load test dengan `wrk` atau `k6` menunjukkan sustained 10,000 TPS selama 10 menit
- **AC-NFR-02.2**: CPU usage < 70% pada 10,000 TPS
- **AC-NFR-02.3**: Memory usage stable (tidak ada memory leak) selama 24 jam continuous load

---

### NFR-03: Reliability - Fail-Open Mechanism

**Requirement:** Jika proxy mengalami kegagalan kritis (crash, out of memory), traffic harus **failover ke database langsung** untuk menghindari total downtime.

**Implementation:**
- Deployment menggunakan **HAProxy** atau **ProxySQL** di depan TransisiDB
- Health check endpoint (`/health`) yang mengembalikan 503 jika proxy unhealthy
- Fallback routing: `HAProxy → TransisiDB (primary) → MySQL (fallback)`

**Trade-off:**
- Fail-open mode: Aplikasi tetap berjalan, tapi **dual-write tidak terjadi**
- Fail-closed mode: Aplikasi error jika proxy down (lebih aman tapi downtime)
- **Pilihan:** Fail-open untuk prioritas availability, dengan alert monitoring

**Acceptance Criteria:**
- **AC-NFR-03.1**: Chaos engineering test: kill proxy process → traffic auto-route ke DB dalam < 5 detik
- **AC-NFR-03.2**: Alert trigger dalam 30 detik jika proxy down

---

### NFR-04: Security - Zero-Knowledge Data Handling

**Requirement:** Proxy **tidak boleh menyimpan log atau cache** yang berisi nilai mata uang aktual (untuk compliance dengan PCI-DSS dan privacy regulations).

**Implementation:**
- Logging hanya mencatat metadata: query type, table name, row count, latency
- **TIDAK logging:** actual values, WHERE clause predicates, customer PII
- Encryption in-transit: TLS 1.3 untuk semua koneksi
- Secrets management: Gunakan HashiCorp Vault atau AWS Secrets Manager untuk database credentials

**Log Example (Compliant):**
```json
{
  "timestamp": "2025-11-20T14:50:00Z",
  "query_type": "INSERT",
  "table": "orders",
  "rows_affected": 1,
  "duration_ms": 12.5,
  "dual_write": true,
  "status": "success"
}
```

**Log Example (NON-Compliant - JANGAN LAKUKAN):**
```json
{
  "query": "INSERT INTO orders (customer_id, total_amount) VALUES (12345, 500000)",
  // ❌ Mengandung actual value
}
```

**Acceptance Criteria:**
- **AC-NFR-04.1**: Security audit dengan grep log files tidak menemukan angka mata uang atau PII
- **AC-NFR-04.2**: Penetration testing memverifikasi TLS enforcement
- **AC-NFR-04.3**: Compliance checklist PCI-DSS Level 1 terpenuhi

---

### NFR-05: Scalability - Horizontal Scaling

**Requirement:** Arsitektur harus mendukung **stateless horizontal scaling** untuk menangani growth traffic.

**Implementation:**
- Proxy instances stateless (semua state di Redis)
- Load balancer (Nginx/HAProxy) di depan multiple proxy instances
- Auto-scaling berdasarkan CPU atau TPS metrics

**Architecture:**
```
                    ┌──────────────┐
                    │ Load Balancer│
                    └───────┬──────┘
                            │
         ┌──────────────────┼──────────────────┐
         ▼                  ▼                  ▼
    [Proxy-01]         [Proxy-02]         [Proxy-03]
         │                  │                  │
         └──────────────────┴──────────────────┘
                            │
                    ┌───────▼────────┐
                    │ Redis Cluster  │
                    └────────────────┘
```

**Acceptance Criteria:**
- **AC-NFR-05.1**: Deployment dengan 3 proxy instances menangani 30,000 TPS (linear scaling)
- **AC-NFR-05.2**: Rolling deployment tanpa request drops

---

### NFR-06: Observability

**Requirement:** Full visibility untuk troubleshooting dan optimization.

**Metrics to Collect:**
- Request rate (per operation type: SELECT, INSERT, UPDATE)
- Error rate (per error type: timeout, deadlock, parse error)
- Latency distribution (P50, P95, P99)
- Backfill progress dan ETA
- Connection pool stats

**Logging Standards:**
- Structured logging (JSON format)
- Log levels: DEBUG, INFO, WARN, ERROR
- Correlation ID untuk tracing request end-to-end

**Acceptance Criteria:**
- **AC-NFR-06.1**: Grafana dashboard dengan 15+ panels untuk key metrics
- **AC-NFR-06.2**: Alert rules untuk: error rate > 1%, latency P95 > 50ms, backfill stalled

---

## 7. ARSITEKTUR TEKNIS

### 7.1 Technology Stack Recommendation

Berikut stack yang direkomendasikan untuk performa, maintainability, dan "impressiveness factor" untuk portfolio:

#### 7.1.1 Core Proxy Engine

**Pilihan 1: Go (Recommended)**

**Pros:**
- Performa tinggi (compiled, concurrent via goroutines)
- Built-in networking libraries yang mature
- Rich ecosystem untuk database drivers
- Mudah deploy (single binary)
- Banyak referensi implementasi proxy (Vitess, ProxySQL alternatives)

**Cons:**
- Garbage collector bisa menyebabkan latency spikes (mitigated dengan tuning)

**Key Libraries:**
- `vitessio/vitess/go/vt/sqlparser`: SQL parsing
- `go-sql-driver/mysql`: MySQL protocol
- `lib/pq`: PostgreSQL driver
- `go-redis/redis`: Redis client

**Pilihan 2: Rust**

**Pros:**
- Performa maksimal (zero-cost abstractions, no GC)
- Memory safety tanpa runtime overhead
- Sangat impressive untuk portfolio (menunjukkan skill advanced)

**Cons:**
- Learning curve lebih steep
- Ecosystem database proxy belum semature Go
- Development time lebih lama

**Key Libraries:**
- `sqlparser-rs`: SQL parsing
- `tokio`: Async runtime
- `redis-rs`: Redis client

**Rekomendasi Akhir:** **Go** untuk balance antara productivity dan performa.

#### 7.1.2 Configuration Store

**Redis 7.0+**

**Rationale:**
- In-memory speed untuk low-latency config lookup
- Pub/Sub untuk config reload tanpa restart
- Persistence (RDB + AOF) untuk disaster recovery
- Cluster mode untuk high availability

**Schema Design:**
- Key pattern: `transisidb:config:{database}:{table}:{column}`
- TTL: No expiry (manual invalidation via API)

#### 7.1.3 Monitoring Stack

**Prometheus + Grafana**

**Metrics Collection:**
- Prometheus client library di Go (`prometheus/client_golang`)
- Expose `/metrics` endpoint
- Scrape interval: 15 detik

**Alerting:**
- Prometheus Alertmanager untuk notifikasi (Slack, PagerDuty)

#### 7.1.4 Deployment Platform

**Docker + Kubernetes**

**Rationale:**
- Containerization untuk consistent deployment
- Kubernetes untuk auto-scaling dan self-healing
- Helm charts untuk infrastructure-as-code

**Kubernetes Resources:**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: transisidb-proxy
spec:
  replicas: 3
  selector:
    matchLabels:
      app: transisidb-proxy
  template:
    spec:
      containers:
      - name: proxy
        image: transisidb/proxy:v1.0.0
        ports:
        - containerPort: 3307  # MySQL protocol
        - containerPort: 8080  # Management API
        resources:
          requests:
            cpu: "1"
            memory: "2Gi"
          limits:
            cpu: "2"
            memory: "4Gi"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 5
```

### 7.2 Architecture Diagram

```
┌───────────────────────────────────────────────────────────────┐
│                         CLIENT LAYER                          │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │ Web App      │  │ Mobile API   │  │ Internal Tools   │   │
│  └──────┬───────┘  └──────┬───────┘  └────────┬─────────┘   │
└─────────┼──────────────────┼───────────────────┼─────────────┘
          │                  │                   │
          └──────────────────┴───────────────────┘
                             │
                    ┌────────▼────────┐
                    │  Load Balancer  │
                    │  (HAProxy)      │
                    └────────┬────────┘
                             │
          ┌──────────────────┼──────────────────┐
          ▼                  ▼                  ▼
┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
│ TransisiDB      │ │ TransisiDB      │ │ TransisiDB      │
│ Proxy Instance 1│ │ Proxy Instance 2│ │ Proxy Instance 3│
│                 │ │                 │ │                 │
│ ┌─────────────┐ │ │ ┌─────────────┐ │ │ ┌─────────────┐ │
│ │Query Parser │ │ │ │Query Parser │ │ │ │Query Parser │ │
│ └──────┬──────┘ │ │ └──────┬──────┘ │ │ └──────┬──────┘ │
│ ┌──────▼──────┐ │ │ ┌──────▼──────┐ │ │ ┌──────▼──────┐ │
│ │Dual-Write   │ │ │ │Dual-Write   │ │ │ │Dual-Write   │ │
│ │Orchestrator │ │ │ │Orchestrator │ │ │ │Orchestrator │ │
│ └──────┬──────┘ │ │ └──────┬──────┘ │ │ └──────┬──────┘ │
│ ┌──────▼──────┐ │ │ ┌──────▼──────┐ │ │ ┌──────▼──────┐ │
│ │Rounding     │ │ │ │Rounding     │ │ │ │Rounding     │ │
│ │Engine       │ │ │ │Engine       │ │ │ │Engine       │ │
│ └─────────────┘ │ │ └─────────────┘ │ │ └─────────────┘ │
│                 │ │                 │ │                 │
│ Port: 3307      │ │ Port: 3307      │ │ Port: 3307      │
│ API: 8080       │ │ API: 8080       │ │ API: 8080       │
└────────┬────────┘ └────────┬────────┘ └────────┬────────┘
         │                   │                   │
         └───────────────────┴───────────────────┘
                             │
                    ┌────────▼────────┐
                    │  Redis Cluster  │
                    │  (Config Store) │
                    └────────┬────────┘
                             │
          ┌──────────────────┼──────────────────┐
          ▼                  ▼                  ▼
┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
│ MySQL Primary   │ │ MySQL Replica 1 │ │ MySQL Replica 2 │
│ (Read/Write)    │ │ (Read-only)     │ │ (Read-only)     │
└─────────────────┘ └─────────────────┘ └─────────────────┘

┌───────────────────────────────────────────────────────────────┐
│                    MONITORING LAYER                           │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │ Prometheus   │─▶│ Grafana      │  │ Alertmanager     │   │
│  │ (Metrics)    │  │ (Dashboards) │  │ (Notifications)  │   │
│  └──────────────┘  └──────────────┘  └──────────────────┘   │
└───────────────────────────────────────────────────────────────┘
```

### 7.3 Deployment Checklist

- [ ] Provision Redis cluster (3 nodes, replication enabled)
- [ ] Deploy 3 proxy instances di Kubernetes
- [ ] Configure load balancer dengan health check
- [ ] Setup Prometheus scraping
- [ ] Import Grafana dashboards
- [ ] Configure Alertmanager rules
- [ ] Run smoke tests
- [ ] Gradual traffic migration (1% → 10% → 50% → 100%)

---

## 8. ROADMAP PENGEMBANGAN

### 8.1 MVP Timeline (12 Minggu)

#### Week 1-2: Research & Foundation
- [ ] **W1.1**: Riset MySQL/PostgreSQL wire protocol (baca dokumentasi resmi)
- [ ] **W1.2**: Setup development environment (Go 1.21+, Redis, Docker)
- [ ] **W1.3**: Proof of Concept: Simple TCP proxy yang forward query tanpa modifikasi
- [ ] **W2.1**: Implementasi SQL parser dengan `vitess/sqlparser`
- [ ] **W2.2**: Unit test untuk parsing INSERT/UPDATE/SELECT queries
- [ ] **W2.3**: Dokumentasi arsitektur teknis

#### Week 3-4: Core Dual-Write Engine
- [ ] **W3.1**: Implementasi query interceptor untuk deteksi currency columns
- [ ] **W3.2**: Implementasi dual-write logic (rewrite query untuk include shadow column)
- [ ] **W3.3**: Transaction management (atomic commit/rollback)
- [ ] **W4.1**: Integration test dengan MySQL testcontainer
- [ ] **W4.2**: Error handling untuk constraint violations
- [ ] **W4.3**: Performance benchmark (baseline latency)

#### Week 5-6: Rounding Engine & Configuration
- [ ] **W5.1**: Implementasi Banker's Rounding algorithm
- [ ] **W5.2**: Unit test dengan 1000+ test cases (edge cases)
- [ ] **W5.3**: Compliance verification (compare dengan IEEE 754 reference)
- [ ] **W6.1**: Design configuration schema (JSON format)
- [ ] **W6.2**: Implementasi Redis integration untuk config storage
- [ ] **W6.3**: Config hot-reload mechanism (Pub/Sub)

#### Week 7-8: Backfill Worker
- [ ] **W7.1**: Implementasi batch processing logic
- [ ] **W7.2**: Throttling mechanism (CPU-aware)
- [ ] **W7.3**: Progress tracking dan logging
- [ ] **W8.1**: Retry logic untuk transient errors
- [ ] **W8.2**: Integration test dengan large dataset (10M rows)
- [ ] **W8.3**: Performance tuning

#### Week 9-10: Time Travel Feature & Management API
- [ ] **W9.1**: Implementasi simulation mode (request header detection)
- [ ] **W9.2**: Response transformation layer
- [ ] **W9.3**: E2E test untuk simulation mode
- [ ] **W10.1**: Build REST API dengan Gin framework (Go)
- [ ] **W10.2**: Implementasi endpoints (config CRUD, backfill control)
- [ ] **W10.3**: API documentation dengan OpenAPI/Swagger

#### Week 11: Monitoring & Observability
- [ ] **W11.1**: Implementasi Prometheus metrics exporter
- [ ] **W11.2**: Build Grafana dashboards
- [ ] **W11.3**: Setup alerting rules
- [ ] **W11.4**: Structured logging implementation
- [ ] **W11.5**: Distributed tracing (OpenTelemetry - optional)

#### Week 12: Testing & Documentation
- [ ] **W12.1**: Load testing dengan k6 (target: 10,000 TPS)
- [ ] **W12.2**: Chaos engineering (kill proxy, network partition)
- [ ] **W12.3**: Security audit (TLS, secrets management)
- [ ] **W12.4**: Finalisasi dokumentasi (README, API docs, runbook)
- [ ] **W12.5**: Demo preparation & presentation slide

### 8.2 Post-MVP Roadmap

**Phase 2 (Optional Enhancements):**
- Support untuk database lain (Microsoft SQL Server, Oracle)
- GUI dashboard untuk non-technical users (DBA)
- Machine learning-based query optimization
- Multi-region deployment support

---

## APPENDIX A: GLOSSARY TAMBAHAN

| Term | Definition |
|------|------------|
| **Atomicity** | Properti transaksi di mana semua operasi sukses atau semua gagal (no partial success) |
| **Idempotency** | Operasi yang dapat dijalankan berulang kali dengan hasil yang sama |
| **Table Lock** | Mekanisme database yang mencegah concurrent access ke tabel selama operasi DDL |
| **Connection Pool** | Koleksi koneksi database yang di-reuse untuk menghindari overhead creating new connections |
| **Proxy** | Intermediary server yang meneruskan request dari client ke server |

---

## APPENDIX B: REFERENCE ARCHITECTURE

Studi kasus implementasi serupa di industri:

1. **Vitess (PlanetScale)**: Database proxy untuk horizontal sharding MySQL
2. **ProxySQL**: High-performance MySQL proxy dengan query caching
3. **PgBouncer**: Lightweight connection pooler untuk PostgreSQL
4. **AWS RDS Proxy**: Managed database proxy untuk RDS

---

## APPENDIX C: ACCEPTANCE SIGN-OFF

Dokumen ini telah direview dan disetujui oleh:

| Role | Name | Signature | Date |
|------|------|-----------|------|
| **Product Manager** | [Pending] | _________ | ______ |
| **Lead Engineer** | [Pending] | _________ | ______ |
| **DBA** | [Pending] | _________ | ______ |
| **Security Officer** | [Pending] | _________ | ______ |

---

**Document Version History:**

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2025-11-20 | AI Solutions Architect | Initial draft |

---

**END OF DOCUMENT**

---

> **Catatan untuk Portfolio:**  
> Dokumen ini mendemonstrasikan kompetensi dalam:
> - ✅ System design untuk high-availability systems
> - ✅ Database migration strategy (zero-downtime)
> - ✅ Compliance dengan standar internasional (ISO, IEEE)
> - ✅ Technical writing yang presisi dan terstruktur
> - ✅ Full-stack architecture (proxy, API, monitoring)
> 
> TransisiDB adalah showcase project yang menunjukkan kemampuan end-to-end dari requirement analysis hingga implementation roadmap untuk solusi enterprise-grade.
