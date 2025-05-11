package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time" // Untuk mengukur waktu eksekusi

	// Sesuaikan path import ini jika struktur direktori/nama modul Anda berbeda
	"github.com/Starath/Tubes2_BE_SayMyName/loadrecipes"
	"github.com/Starath/Tubes2_BE_SayMyName/pathfinding"
	"github.com/Starath/Tubes2_BE_SayMyName/pathfinding/bfs"
	// "github.com/Starath/Tubes2_BE_SayMyName/scrape" // Jika Anda ingin menjalankan scraper setiap kali
)

// graphInstance akan menyimpan data graf yang sudah dimuat.
var graphInstance *loadrecipes.BiGraphAlchemy

// Helper function untuk mencetak hasil MultipleResult dengan lebih rapi
func printMultipleResult(
	algorithmName string,
	target string,
	result *pathfinding.MultipleResult,
	maxPaths int,
	duration time.Duration,
	err error,
) {
	fmt.Printf("\n--- Hasil %s untuk: %s (Max Jalur Diminta: %d) ---\n", algorithmName, target, maxPaths)
	fmt.Printf("Waktu Eksekusi: %s\n", duration)

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		// Mungkin result.Results[0].NodesVisited masih ada jika error terjadi setelah beberapa eksplorasi
		if result != nil && len(result.Results) > 0 {
			fmt.Printf("Nodes Explored (Estimasi Global): %d\n", result.Results[0].NodesVisited)
		} else if result != nil && len(result.Results) == 0 { // Error sebelum ada hasil
			// Tidak ada NodesVisited yang bisa diandalkan dari result.Results jika kosong
			// Mungkin bisa ditambahkan field NodesExplored di MultipleResult itu sendiri untuk kasus error.
			// Untuk sekarang, kita tidak cetak jika error dan tidak ada hasil.
		}
		fmt.Println(strings.Repeat("-", 40))
		return
	}

	if result == nil || len(result.Results) == 0 {
		// nodesExplored := 0
		// Jika target adalah elemen dasar, BFSFindXDifferentPathsBackward yang asli akan mengembalikan 1 node.
		// Versi paralel juga harusnya begitu.
		if graphInstance.BaseElements[target] {
			fmt.Println("- Elemen dasar.")
			// nodesExplored = 1
		} else {
			fmt.Println("- Tidak ada jalur resep yang ditemukan.")
			// Jika result tidak nil tapi Results kosong, coba ambil NodesVisited dari suatu tempat jika ada.
			// Untuk kasus di mana tidak ada path, NodesVisited bisa jadi 0 atau >0 tergantung bagaimana algoritma menghitungnya.
			// Kita akan mengandalkan nilai yang dikembalikan oleh fungsi BFS.
			// Jika result.Results kosong, NodesVisited di sini mungkin sulit ditentukan secara konsisten
			// kecuali field NodesVisited ada di level MultipleResult.
			// Untuk sekarang, jika path tidak ada dan bukan base, kita tidak cetak NodesVisited dari sini.
		}
		// Jika ingin menampilkan nodesExplored bahkan saat tidak ada path (selain base element),
		// pastikan fungsi BFS mengembalikan nilai ini dengan konsisten.
		// BFSFindXDifferentPathsBackward yang asli mengisi Results[0].NodesVisited dengan total eksplorasi.
		// Jika Results kosong, maka tidak ada nilai tersebut.
		// Untuk BFSFindXDifferentPathsBackward_Parallel, jika tidak ada worker diluncurkan, nodesExploredGlobal bisa 0.
		// Kita asumsikan field NodesVisited di Result pertama (jika ada) adalah representasi global.
		// Jika tidak ada Results, kita tidak bisa menampilkannya dari sini.
		fmt.Println(strings.Repeat("-", 40))
		return
	}

	fmt.Printf("Ditemukan %d jalur berbeda (dari maks %d yang diminta):\n", len(result.Results), maxPaths)
	// NodesVisited pada setiap Result di MultipleResult harusnya sudah diisi oleh fungsi BFS
	// dengan total eksplorasi global. Kita bisa cetak salah satunya sebagai representasi.
	if len(result.Results) > 0 {
		fmt.Printf("Nodes Explored (Estimasi Global): %d\n", result.Results[0].NodesVisited)
	}

	for i, resPath := range result.Results {
		fmt.Printf("\n  Jalur %d/%d:\n", i+1, len(result.Results))
		if len(resPath.Path) == 0 {
			if graphInstance.BaseElements[target] { // Double check, meski kasus base harusnya sudah ditangani
				fmt.Println("  - Elemen dasar.")
			} else {
				fmt.Println("  - Path kosong (kemungkinan tidak seharusnya terjadi jika result.Results tidak kosong dan bukan base).")
			}
		} else {
			fmt.Printf("  Jumlah Langkah Resep: %d\n", len(resPath.Path))
			fmt.Println("  Resep:")
			for j, step := range resPath.Path {
				fmt.Printf("    %d. %s = %s + %s\n", j+1, step.ChildName, step.Parent1Name, step.Parent2Name)
			}
		}
	}
	fmt.Println(strings.Repeat("-", 40))
}

func main() {
	// --- 0. (Opsional) Jalankan Scraper untuk Memperbarui Data ---
	// Uncomment jika ingin menjalankan scraper setiap kali.
	// fmt.Println("===== MEMULAI PROSES SCRAPING DATA ELEMEN =====")
	// scrape.Scrapping()
	// fmt.Println("===== PROSES SCRAPING DATA ELEMEN SELESAI =====")
	// fmt.Println(strings.Repeat("=", 50))

	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Memuat data resep dari elements_filtered.json...")
	var errLoad error
	// Pastikan graphInstance diinisialisasi.
	// Jika elements_filtered.json tidak ada, scrape.Scrapping() harus dijalankan dulu atau pastikan file ada.
	// Untuk development, Anda bisa menganggap file sudah ada.
	graphPath := "elements_filtered.json" 
	// Jika Anda memindahkan elements_filtered.json ke folder scrape, pathnya jadi "scrape/elements_filtered.json"
	// Untuk proyek Go, biasanya file data diletakkan di root atau subdirektori khusus data.
	// Jika berada di dalam direktori `scrape` dan `main.go` ada di root `Tubes2_BE_SayMyName`, pathnya bisa jadi `scrape/elements_filtered.json`
	// Namun, berdasarkan struktur file yang Anda berikan, elements_filtered.json tampak sejajar dengan main.go setelah proses scrape.
	// Jika `scrape.Scrapping()` menghasilkan output di `Tubes2_BE_SayMyName/scrape/elements_filtered.json`, maka pathnya harus itu.
	// Asumsi saat ini: scrape.Scrapping() menghasilkan `elements_filtered.json` di direktori yang sama dengan `main.go` dijalankan
	// atau `main.go` ada di root dan `elements_filtered.json` juga di root.
	// Jika scrape menghasilkan `scrape/elements_filtered.json`, gunakan path `scrape/elements_filtered.json`.
	// Berdasarkan struktur folder Anda, sepertinya `elements_filtered.json` ada di `Tubes2_BE_SayMyName/scrape/elements_filtered.json`.
	// Namun, file `main.go` Anda juga di `Tubes2_BE_SayMyName`.
	// Mari asumsikan `elements_filtered.json` berada di direktori yang sama dengan executable `main` setelah build,
	// atau jika menjalankan dengan `go run main.go` dari root project, pathnya adalah `scrape/elements_filtered.json` jika scraper menyimpannya di sana.
	// Jika scraper menyimpan di root, maka cukup `elements_filtered.json`.
	// Saya akan menggunakan "elements_filtered.json" dengan asumsi file tersebut dapat diakses dari direktori kerja saat program dijalankan.
	// Sesuaikan jika scraper Anda menyimpan file di subdirektori `scrape`.
	graphInstance, errLoad = loadrecipes.LoadBiGraph(graphPath) 
	if errLoad != nil {
		log.Printf("WARNING: Gagal memuat data resep dari '%s': %v. Mencoba dari 'scrape/%s'", graphPath, errLoad, graphPath)
		graphPath = "scrape/" + graphPath // Coba path alternatif jika scraper menyimpan di subfolder scrape
		graphInstance, errLoad = loadrecipes.LoadBiGraph(graphPath)
		if errLoad != nil {
			log.Fatalf("FATAL: Gagal memuat data resep dari kedua path ('%s' dan alternatifnya): %v", "elements_filtered.json", errLoad)
		}
	}
	fmt.Printf("Data resep berhasil dimuat dari '%s'.\n", graphPath)
	fmt.Println(strings.Repeat("=", 50))


	for {
		fmt.Print("\nMasukkan nama elemen target (atau 'exit' untuk keluar): ")
		targetInput, _ := reader.ReadString('\n')
		target := strings.TrimSpace(targetInput)

		if strings.ToLower(target) == "exit" {
			break
		}
		if target == "" {
			fmt.Println("Nama target tidak boleh kosong.")
			continue
		}

		fmt.Print("Masukkan jumlah maksimal jalur yang ingin dicari (default: 1): ")
		numPathsStr, _ := reader.ReadString('\n')
		numPathsStr = strings.TrimSpace(numPathsStr)
		if numPathsStr == "" {
			numPathsStr = "1" // Default jika input kosong
		}
		numPathsToFind, errConv := strconv.Atoi(numPathsStr)
		if errConv != nil || numPathsToFind <= 0 {
			fmt.Printf("Input jumlah jalur tidak valid, menggunakan default 1.\n")
			numPathsToFind = 1
		}
		fmt.Println(strings.Repeat("=", 50))

		// --- 1. Menjalankan BFS Sekuensial ---
		fmt.Printf("\n[INFO] Memulai BFS Sekuensial untuk: '%s', Max Jalur: %d\n", target, numPathsToFind)
		startSequential := time.Now()
		resultSequential, errSequential := bfs.BFSFindXDifferentPathsBackward(graphInstance, target, numPathsToFind)
		durationSequential := time.Since(startSequential)
		printMultipleResult("BFS Sekuensial (Original)", target, resultSequential, numPathsToFind, durationSequential, errSequential)
		fmt.Println(strings.Repeat("=", 50))

		// --- 2. Menjalankan BFS Multi-threaded ---
		// Pastikan fungsi BFSFindXDifferentPathsBackward_Parallel sudah ada di bfs/BFS.go
		fmt.Printf("\n[INFO] Memulai BFS Multi-threaded untuk: '%s', Max Jalur: %d\n", target, numPathsToFind)
		startParallel := time.Now()
		resultParallel, errParallel := bfs.BFSFindXDifferentPathsBackward_Parallel(graphInstance, target, numPathsToFind)
		durationParallel := time.Since(startParallel)
		printMultipleResult("BFS Multi-threaded (Paralel per Resep Awal)", target, resultParallel, numPathsToFind, durationParallel, errParallel)
		fmt.Println(strings.Repeat("=", 50))

		// Perbandingan Waktu
		fmt.Printf("\nPerbandingan Waktu Eksekusi untuk Target '%s' (Max Jalur: %d):\n", target, numPathsToFind)
		fmt.Printf("  - BFS Sekuensial: %s\n", durationSequential)
		fmt.Printf("  - BFS Multi-threaded: %s\n", durationParallel)
		if durationSequential > 0 && durationParallel > 0 {
			if durationParallel < durationSequential {
				speedup := float64(durationSequential) / float64(durationParallel)
				fmt.Printf("  - Multi-threaded %.2fx lebih cepat.\n", speedup)
			} else if durationSequential < durationParallel {
				slowdown := float64(durationParallel) / float64(durationSequential)
				fmt.Printf("  - Sekuensial %.2fx lebih cepat (Multi-threaded lebih lambat).\n", slowdown)
			} else {
				fmt.Println("  - Waktu eksekusi kurang lebih sama.")
			}
		}
		fmt.Println(strings.Repeat("#", 60))
	}
	fmt.Println("\n===== Program Selesai =====")
}