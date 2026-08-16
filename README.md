# go-hris-payroll-system

Studi kasus sistem HRIS (Human Resource Information System) & Payroll Terintegrasi — dibuat menggunakan Go (Golang) dengan integrasi database PostgreSQL.

## Fitur
- Interface `PayrollCalculator` dengan 3 tipe karyawan: FullTime, Contract, Freelancer
- Validasi pendaftaran karyawan (ID unik, nama tidak kosong, nilai keuangan tidak negatif)
- Perhitungan total anggaran gaji dan laporan payroll per karyawan
- Penyimpanan data ke database PostgreSQL (opsional, otomatis membuat tabel)

## Cara Menjalankan
1. Buat database PostgreSQL: `CREATE DATABASE hris_db;`
2. Sesuaikan kredensial koneksi di bagian atas `main.go`
3. Jalankan `go mod tidy`
4. Jalankan `go run main.go`

## Struktur
- `main.go` — seluruh kode program
- `go.mod` — konfigurasi module Go
