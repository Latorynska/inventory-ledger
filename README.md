# Inventory Ledger API

Inventory Ledger API adalah **side project backend** berbasis **Go (Golang)** yang berfungsi untuk mencatat dan mengelola pergerakan stok (inventory ledger) secara akurat, terstruktur, dan dapat diaudit.

Project ini cocok untuk studi kasus **inventory management**, **stock mutation**, dan **audit trail** menggunakan pendekatan transaksi ledger.

---

## ✨ Fitur Utama

* 📦 **Pencatatan Transaksi Inventory**

  * First stock
  * Transaction (in / out)
  * Mutation (antar organisasi)
  * Stock opname

* 📊 **Perhitungan Saldo Stok**

  * Current balance
  * Historical balance (saldo pada waktu tertentu)

* 🧾 **Audit & Riwayat**

  * Riwayat transaksi
  * Summary per organisasi
  * Summary per item

* 🔁 **Rollback Transaksi**

  * Membatalkan transaksi dengan aman tanpa merusak histori

* 🛠️ **REST API**

  * Menggunakan **Gin**
  * ORM dengan **GORM**

---

## 🏗️ Tech Stack

* **Language**: Go
* **Framework**: Gin
* **ORM**: GORM
* **Database**: PostgreSQL / MySQL (via GORM)
* **UUID**: google/uuid

---

## 📂 Struktur Project

```text
.
├── main.go
├── go.mod
├── go.sum
└── src
    ├── config        # Konfigurasi aplikasi & database
    ├── handlers      # HTTP handlers (controller layer)
    ├── models        # Model database (GORM)
    ├── repositories  # Data access layer
    ├── services      # Business logic
    └── routes        # Routing API
```

---

## 🚀 Cara Menjalankan Project

### 1️⃣ Clone Repository

```bash
git clone <repo-url>
cd inventory-ledger
```

### 2️⃣ Konfigurasi Environment

Pastikan database sudah berjalan, lalu set environment variable (contoh):

```env
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=inventory_ledger
```

> Penyesuaian bisa dilihat di folder `src/config`

### 3️⃣ Install Dependency

```bash
go mod tidy
```

### 4️⃣ Jalankan Aplikasi

```bash
go run main.go
```

Server akan berjalan di:

```text
http://localhost:8080
```

---

## 🔗 Daftar Endpoint Utama

Base path:

```text
/api/inventory
```

### GET

* `GET /balance/current`
* `GET /balance/historical`
* `GET /transactions`
* `GET /summary/org`
* `GET /summary/item`
* `GET /history`

### POST

* `POST /transaction`
* `POST /mutation`
* `POST /opname`
* `POST /rollback`

### PUT

* `PUT /transaction`

### DELETE

* `DELETE /transaction`

---

## 🧠 Konsep yang Digunakan

* **Ledger-based inventory** (tidak update stok langsung)
* **Immutability** (rollback dibuat sebagai transaksi baru)
* **Audit trail friendly**
* **Separation of concerns** (handler, service, repository)

---

## 📌 Catatan

* Project ini dibuat sebagai **side project / learning project**

