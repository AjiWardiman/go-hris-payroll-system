package main

import (
	"bufio"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	_ "github.com/lib/pq" // driver PostgreSQL. Diimpor pakai "_" karena
	// kita cuma butuh efek sampingnya (mendaftarkan diri ke database/sql),
	// bukan pakai fungsinya secara langsung.
)

// ============================================================
// KONFIGURASI KONEKSI DATABASE
// ============================================================
// GANTI nilai-nilai di bawah ini sesuai setting PostgreSQL kamu.
const (
	dbHost     = "localhost"
	dbPort     = 5432
	dbUser     = "postgres"
	dbPassword = "postgres" // ganti sesuai password PostgreSQL kamu
	dbName     = "hris_db"  // pastikan database ini sudah dibuat duluan
)

// ============================================================
// 1. INTERFACE PayrollCalculator
// ============================================================
// Interface di Go itu seperti "kontrak" / "daftar tugas wajib".
// Struct/tipe apapun yang punya SEMUA method di bawah ini otomatis
// dianggap "implement" interface ini — TANPA perlu tulis kata
// "implements" seperti di Java/C#. Ini disebut "implisit".
type PayrollCalculator interface {
	// Hitung total gaji bersih bulanan. Return (hasil, error).
	// Kalau perhitungan valid -> error-nya nil.
	CalculateSalary() (float64, error)

	// Kembalikan tipe karyawan sebagai string, contoh: "FullTime"
	GetEmployeeType() string
}

// ============================================================
// 2. CONCRETE STRUCTS (tipe-tipe karyawan konkret)
// ============================================================
// Ketiga struct di bawah ini masing-masing punya method
// CalculateSalary() dan GetEmployeeType() -> otomatis jadi
// "PayrollCalculator" secara implisit.

// --- Karyawan Tetap (Full Time) ---
type FullTimeEmployee struct {
	BaseSalary float64 // gaji pokok
	Allowance  float64 // tunjangan
	TaxRate    float64 // potongan pajak, misal 0.05 = 5%
}

// Rumus: (BaseSalary + Allowance) * (1.0 - TaxRate)
func (f FullTimeEmployee) CalculateSalary() (float64, error) {
	gajiKotor := f.BaseSalary + f.Allowance
	gajiBersih := gajiKotor * (1.0 - f.TaxRate)
	return gajiBersih, nil
}

func (f FullTimeEmployee) GetEmployeeType() string {
	return "FullTime"
}

// --- Karyawan Kontrak (Contract) ---
type ContractEmployee struct {
	MonthlyRate      float64 // rate bulanan
	PerformanceBonus float64 // bonus kinerja
}

// Rumus: MonthlyRate + PerformanceBonus
func (c ContractEmployee) CalculateSalary() (float64, error) {
	total := c.MonthlyRate + c.PerformanceBonus
	return total, nil
}

func (c ContractEmployee) GetEmployeeType() string {
	return "Contract"
}

// --- Karyawan Lepas (Freelancer) ---
type Freelancer struct {
	HourlyRate  float64 // upah per jam
	HoursWorked int     // jumlah jam kerja
}

// Rumus: HourlyRate * float64(HoursWorked)
// HoursWorked bertipe int, sedangkan HourlyRate float64.
// Go TIDAK bisa mengalikan int dan float64 secara langsung,
// makanya HoursWorked harus di-"convert" dulu pakai float64(...)
func (fl Freelancer) CalculateSalary() (float64, error) {
	total := fl.HourlyRate * float64(fl.HoursWorked)
	return total, nil
}

func (fl Freelancer) GetEmployeeType() string {
	return "Freelancer"
}

// ============================================================
// 3. DYNAMIC COLLECTION (Map) -> Struct HRIS
// ============================================================
// map[KeyType]ValueType adalah struktur data key-value di Go,
// mirip "dictionary" di Python atau "object" di JS.
type HRIS struct {
	// Employees: [EmployeeID] -> Nama Karyawan
	Employees map[string]string

	// Payrolls: [EmployeeID] -> data payroll (bertipe interface!)
	// Karena bertipe interface, isinya BISA berupa
	// FullTimeEmployee, ContractEmployee, ATAU Freelancer.
	// Inilah kekuatan interface: satu map bisa nampung banyak
	// tipe berbeda selama semuanya "PayrollCalculator".
	Payrolls map[string]PayrollCalculator
}

// Constructor helper (bukan wajib dari soal, tapi praktik yang baik)
// supaya map di dalam HRIS ter-inisialisasi (tidak nil) sebelum dipakai.
func NewHRIS() *HRIS {
	return &HRIS{
		Employees: make(map[string]string),
		Payrolls:  make(map[string]PayrollCalculator),
	}
}

// ============================================================
// FUNGSI BANTUAN: format angka jadi gaya Rupiah (titik ribuan)
// ============================================================
// Ini BUKAN bagian wajib dari soal, cuma tambahan biar output
// enak dibaca saat presentasi. Contoh: 8000000.00 -> 8.000.000,00
func formatRupiah(angka float64) string {
	// PENTING: jangan langsung potong (truncate) bagian desimalnya,
	// karena float64 kadang punya sisa pembulatan (misal 99.995 bisa
	// tersimpan sebagai 99.994999999...). Makanya kita bulatkan dulu
	// ke satuan "sen" (2 desimal) pakai math.Round, baru dipecah.
	totalSen := int64(math.Round(angka * 100))

	negatif := totalSen < 0
	if negatif {
		totalSen = -totalSen
	}

	bulat := totalSen / 100  // bagian rupiah
	desimal := totalSen % 100 // bagian sen (0-99)

	// Ubah bagian bulat ke string, lalu sisipkan titik tiap 3 digit
	// dari belakang (mulai dari satuan, ribuan, jutaan, dst).
	angkaStr := strconv.FormatInt(bulat, 10)

	var hasil []string
	for len(angkaStr) > 3 {
		hasil = append([]string{angkaStr[len(angkaStr)-3:]}, hasil...)
		angkaStr = angkaStr[:len(angkaStr)-3]
	}
	hasil = append([]string{angkaStr}, hasil...)
	gabung := strings.Join(hasil, ".")

	if negatif {
		gabung = "-" + gabung
	}

	return fmt.Sprintf("%s,%02d", gabung, desimal)
}

// ============================================================
// 4. METHOD PADA STRUCT HRIS (Pointer Receiver)
// ============================================================
// Kenapa pakai (h *HRIS) bukan (h HRIS)?
// Karena method ini MENGUBAH isi struct (menambah data ke map).
// Kalau pakai receiver biasa (bukan pointer), Go akan bekerja
// dengan SALINAN struct, jadi perubahan tidak "nempel" ke aslinya.
// Pointer receiver (*HRIS) memastikan kita mengubah data yang asli.

// --- RegisterEmployee: mendaftarkan karyawan baru ---
func (h *HRIS) RegisterEmployee(id string, name string, payroll PayrollCalculator) error {
	// Validasi 1: ID sudah terdaftar?
	if _, sudahAda := h.Employees[id]; sudahAda {
		return errors.New("karyawan dengan ID tersebut sudah terdaftar")
	}

	// Validasi 2: nama kosong?
	if name == "" {
		return errors.New("nama karyawan tidak boleh kosong")
	}

	// Validasi 3: nilai keuangan negatif?
	// Karena payroll bertipe interface, kita perlu "buka" tipe
	// aslinya dulu pakai TYPE SWITCH untuk mengecek field masing-masing.
	switch p := payroll.(type) {
	case FullTimeEmployee:
		if p.BaseSalary < 0 || p.Allowance < 0 || p.TaxRate < 0 {
			return errors.New("nilai keuangan tidak boleh negatif")
		}
	case ContractEmployee:
		if p.MonthlyRate < 0 || p.PerformanceBonus < 0 {
			return errors.New("nilai keuangan tidak boleh negatif")
		}
	case Freelancer:
		if p.HourlyRate < 0 || p.HoursWorked < 0 {
			return errors.New("nilai keuangan tidak boleh negatif")
		}
	}

	// Kalau semua validasi lolos -> simpan ke kedua map
	h.Employees[id] = name
	h.Payrolls[id] = payroll

	return nil // tidak ada error
}

// --- CalculateTotalPayout: total anggaran gaji semua karyawan ---
func (h *HRIS) CalculateTotalPayout() float64 {
	var total float64 = 0

	// "for range" dipakai untuk mengiterasi (looping) isi map.
	// _ dipakai karena kita tidak butuh key (employeeID) di sini,
	// cukup ambil value-nya (payroll).
	for _, payroll := range h.Payrolls {
		gaji, err := payroll.CalculateSalary()
		if err == nil {
			total += gaji
		}
	}

	return total
}

// --- PrintPayrollReport: cetak slip laporan keuangan tiap karyawan ---
func (h *HRIS) PrintPayrollReport() {
	fmt.Println("========== LAPORAN PAYROLL ==========")

	for id, payroll := range h.Payrolls {
		nama := h.Employees[id]
		gaji, err := payroll.CalculateSalary()

		fmt.Printf("\nID           : %s\n", id)
		fmt.Printf("Nama         : %s\n", nama)
		fmt.Printf("Tipe         : %s\n", payroll.GetEmployeeType())

		if err != nil {
			fmt.Printf("Status       : GAGAL HITUNG GAJI (%v)\n", err)
			continue
		}
		fmt.Printf("Gaji Bersih  : Rp %s\n", formatRupiah(gaji))

		// Type Switch lagi di sini untuk mencetak detail SPESIFIK
		// sesuai tipe konkretnya (soal poin 4 - PrintPayrollReport).
		switch p := payroll.(type) {
		case FullTimeEmployee:
			fmt.Printf("Detail       : Base=Rp %s, Allowance=Rp %s, TaxRate=%.0f%%\n",
				formatRupiah(p.BaseSalary), formatRupiah(p.Allowance), p.TaxRate*100)
		case ContractEmployee:
			fmt.Printf("Detail       : MonthlyRate=Rp %s, Bonus=Rp %s\n",
				formatRupiah(p.MonthlyRate), formatRupiah(p.PerformanceBonus))
		case Freelancer:
			fmt.Printf("Detail       : HourlyRate=Rp %s, Jam Kerja=%d jam\n",
				formatRupiah(p.HourlyRate), p.HoursWorked)
		}
	}

	fmt.Println("\n======================================")
}

// ============================================================
// FUNGSI BANTUAN INPUT (bukan bagian wajib soal, cuma alat bantu
// supaya program bisa menerima input dari user lewat terminal)
// ============================================================

var reader = bufio.NewReader(os.Stdin)

// Baca satu baris teks dari user, hilangkan spasi/enter di ujungnya
func bacaTeks(label string) string {
	fmt.Print(label)
	teks, _ := reader.ReadString('\n')
	return strings.TrimSpace(teks)
}

// Baca angka desimal (float64) dari user. Kalau salah ketik,
// user akan diminta mengulang sampai valid.
func bacaFloat(label string) float64 {
	for {
		teks := bacaTeks(label)
		nilai, err := strconv.ParseFloat(teks, 64)
		if err != nil {
			fmt.Println("Input tidak valid, masukkan angka (contoh: 1000000 atau 1000000.50)")
			continue
		}
		return nilai
	}
}

// Baca angka bulat (int) dari user, sama seperti bacaFloat tapi untuk int.
func bacaInt(label string) int {
	for {
		teks := bacaTeks(label)
		nilai, err := strconv.Atoi(teks)
		if err != nil {
			fmt.Println("Input tidak valid, masukkan angka bulat (contoh: 40)")
			continue
		}
		return nilai
	}
}

// ============================================================
// FUNGSI-FUNGSI DATABASE (PostgreSQL)
// ============================================================

// connectDB membuka koneksi ke PostgreSQL dan mengetesnya dengan Ping().
func connectDB() (*sql.DB, error) {
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	// sql.Open tidak langsung konek, cuma menyiapkan objeknya.
	// Ping() ini yang benar-benar mengetes apakah koneksinya berhasil.
	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}

// createTables membuat tabel employees & payrolls kalau belum ada.
// "IF NOT EXISTS" supaya aman dijalankan berkali-kali tanpa error.
func createTables(db *sql.DB) error {
	queryEmployees := `
	CREATE TABLE IF NOT EXISTS employees (
		id   TEXT PRIMARY KEY,
		name TEXT NOT NULL
	);`

	// Semua field dari ketiga tipe karyawan digabung dalam satu tabel,
	// kolom yang tidak dipakai oleh suatu tipe akan diisi NULL.
	queryPayrolls := `
	CREATE TABLE IF NOT EXISTS payrolls (
		employee_id       TEXT PRIMARY KEY REFERENCES employees(id),
		employee_type     TEXT NOT NULL,
		base_salary       DOUBLE PRECISION,
		allowance         DOUBLE PRECISION,
		tax_rate          DOUBLE PRECISION,
		monthly_rate      DOUBLE PRECISION,
		performance_bonus DOUBLE PRECISION,
		hourly_rate       DOUBLE PRECISION,
		hours_worked      INTEGER
	);`

	if _, err := db.Exec(queryEmployees); err != nil {
		return err
	}
	if _, err := db.Exec(queryPayrolls); err != nil {
		return err
	}
	return nil
}

// simpanKaryawanKeDB menyimpan satu karyawan + data payroll-nya ke database.
// Dipanggil setelah RegisterEmployee() berhasil di memori.
func simpanKaryawanKeDB(db *sql.DB, id string, name string, payroll PayrollCalculator) error {
	// Simpan ke tabel employees dulu
	_, err := db.Exec(
		`INSERT INTO employees (id, name) VALUES ($1, $2)`,
		id, name,
	)
	if err != nil {
		return err
	}

	// Simpan ke tabel payrolls sesuai tipe konkretnya.
	// Field yang tidak relevan untuk tipe tsb dibiarkan NULL (nil di Go).
	switch p := payroll.(type) {
	case FullTimeEmployee:
		_, err = db.Exec(
			`INSERT INTO payrolls (employee_id, employee_type, base_salary, allowance, tax_rate)
			 VALUES ($1, $2, $3, $4, $5)`,
			id, "FullTime", p.BaseSalary, p.Allowance, p.TaxRate,
		)
	case ContractEmployee:
		_, err = db.Exec(
			`INSERT INTO payrolls (employee_id, employee_type, monthly_rate, performance_bonus)
			 VALUES ($1, $2, $3, $4)`,
			id, "Contract", p.MonthlyRate, p.PerformanceBonus,
		)
	case Freelancer:
		_, err = db.Exec(
			`INSERT INTO payrolls (employee_id, employee_type, hourly_rate, hours_worked)
			 VALUES ($1, $2, $3, $4)`,
			id, "Freelancer", p.HourlyRate, p.HoursWorked,
		)
	}

	return err
}

// muatSemuaKaryawanDariDB membaca semua data dari database dan
// memasukkannya kembali ke dalam struct HRIS di memori.
func muatSemuaKaryawanDariDB(db *sql.DB, hris *HRIS) error {
	query := `
	SELECT e.id, e.name, p.employee_type,
	       p.base_salary, p.allowance, p.tax_rate,
	       p.monthly_rate, p.performance_bonus,
	       p.hourly_rate, p.hours_worked
	FROM employees e
	JOIN payrolls p ON e.id = p.employee_id;`

	rows, err := db.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close() // pastikan koneksi baris ini ditutup setelah selesai dipakai

	jumlahDimuat := 0

	for rows.Next() {
		var id, name, tipe string

		// Pakai sql.NullFloat64 / sql.NullInt64 karena kolom-kolom ini
		// bisa NULL di database (tergantung tipe karyawannya).
		var baseSalary, allowance, taxRate sql.NullFloat64
		var monthlyRate, performanceBonus sql.NullFloat64
		var hourlyRate sql.NullFloat64
		var hoursWorked sql.NullInt64

		err := rows.Scan(
			&id, &name, &tipe,
			&baseSalary, &allowance, &taxRate,
			&monthlyRate, &performanceBonus,
			&hourlyRate, &hoursWorked,
		)
		if err != nil {
			return err
		}

		var payroll PayrollCalculator

		switch tipe {
		case "FullTime":
			payroll = FullTimeEmployee{
				BaseSalary: baseSalary.Float64,
				Allowance:  allowance.Float64,
				TaxRate:    taxRate.Float64,
			}
		case "Contract":
			payroll = ContractEmployee{
				MonthlyRate:      monthlyRate.Float64,
				PerformanceBonus: performanceBonus.Float64,
			}
		case "Freelancer":
			payroll = Freelancer{
				HourlyRate:  hourlyRate.Float64,
				HoursWorked: int(hoursWorked.Int64),
			}
		default:
			continue // tipe tidak dikenali, lewati baris ini
		}

		// Masukkan langsung ke map (bukan lewat RegisterEmployee) karena
		// data ini sudah tervalidasi saat pertama kali disimpan dulu.
		hris.Employees[id] = name
		hris.Payrolls[id] = payroll
		jumlahDimuat++
	}

	fmt.Printf("%d data karyawan berhasil dimuat dari database.\n", jumlahDimuat)
	return nil
}

// ============================================================
// MAIN: menu interaktif untuk mendaftarkan karyawan lewat input user
// ============================================================
func main() {
	hris := NewHRIS()

	// --- Koneksi ke database ---
	db, err := connectDB()
	if err != nil {
		fmt.Println("Gagal terhubung ke database:", err)
		fmt.Println("Program tetap bisa jalan, tapi data TIDAK akan tersimpan permanen.")
		db = nil
	} else {
		defer db.Close() // pastikan koneksi ditutup saat program selesai
		if err := createTables(db); err != nil {
			fmt.Println("Gagal membuat tabel:", err)
		} else {
			fmt.Println("Berhasil terhubung ke database PostgreSQL.")
		}
	}

	for {
		fmt.Println("\n===== MENU SISTEM HRIS & PAYROLL =====")
		fmt.Println("1. Daftarkan Karyawan Baru")
		fmt.Println("2. Cetak Laporan Payroll")
		fmt.Println("3. Hitung Total Anggaran Gaji")
		fmt.Println("4. Muat Data dari Database")
		fmt.Println("5. Keluar")
		pilihan := bacaTeks("Pilih menu (1-5): ")

		switch pilihan {
		case "1":
			daftarkanKaryawanDariInput(hris, db)

		case "2":
			hris.PrintPayrollReport()

		case "3":
			total := hris.CalculateTotalPayout()
			fmt.Printf("\nTOTAL ANGGARAN GAJI BULAN INI: Rp %s\n", formatRupiah(total))

		case "4":
			if db == nil {
				fmt.Println("Tidak ada koneksi database.")
				continue
			}
			if err := muatSemuaKaryawanDariDB(db, hris); err != nil {
				fmt.Println("Gagal memuat data:", err)
			}

		case "5":
			fmt.Println("Program selesai. Terima kasih.")
			return

		default:
			fmt.Println("Pilihan tidak dikenali, coba lagi.")
		}
	}
}

// daftarkanKaryawanDariInput menanyakan data karyawan ke user lewat
// terminal, lalu memanggil RegisterEmployee sesuai tipe yang dipilih.
// Kalau koneksi database tersedia, data juga otomatis disimpan ke DB.
func daftarkanKaryawanDariInput(hris *HRIS, db *sql.DB) {
	id := bacaTeks("Masukkan ID Karyawan: ")
	nama := bacaTeks("Masukkan Nama Karyawan: ")

	fmt.Println("Pilih Tipe Karyawan:")
	fmt.Println("1. FullTime")
	fmt.Println("2. Contract")
	fmt.Println("3. Freelancer")
	tipe := bacaTeks("Pilihan (1-3): ")

	var payroll PayrollCalculator

	switch tipe {
	case "1":
		baseSalary := bacaFloat("Gaji Pokok (BaseSalary): ")
		allowance := bacaFloat("Tunjangan (Allowance): ")
		taxRate := bacaFloat("Tax Rate (contoh 0.05 untuk 5%): ")
		payroll = FullTimeEmployee{
			BaseSalary: baseSalary,
			Allowance:  allowance,
			TaxRate:    taxRate,
		}

	case "2":
		monthlyRate := bacaFloat("Rate Bulanan (MonthlyRate): ")
		bonus := bacaFloat("Bonus Kinerja (PerformanceBonus): ")
		payroll = ContractEmployee{
			MonthlyRate:      monthlyRate,
			PerformanceBonus: bonus,
		}

	case "3":
		hourlyRate := bacaFloat("Upah per Jam (HourlyRate): ")
		hoursWorked := bacaInt("Jumlah Jam Kerja (HoursWorked): ")
		payroll = Freelancer{
			HourlyRate:  hourlyRate,
			HoursWorked: hoursWorked,
		}

	default:
		fmt.Println("Tipe karyawan tidak dikenali. Pendaftaran dibatalkan.")
		return
	}

	err := hris.RegisterEmployee(id, nama, payroll)
	if err != nil {
		fmt.Println("Gagal mendaftarkan karyawan:", err)
		return
	}

	fmt.Println("Karyawan berhasil didaftarkan (di memori).")

	// Simpan juga ke database kalau koneksinya tersedia
	if db != nil {
		if err := simpanKaryawanKeDB(db, id, nama, payroll); err != nil {
			fmt.Println("Peringatan: gagal menyimpan ke database:", err)
		} else {
			fmt.Println("Data berhasil disimpan ke database.")
		}
	}
}
