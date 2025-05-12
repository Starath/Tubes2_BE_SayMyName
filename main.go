// // package main

// // import (
// // 	"bufio"
// // 	"fmt"
// // 	"log"
// // 	"os"
// // 	"strconv"
// // 	"strings"
// // 	"time" // Untuk mengukur waktu eksekusi (opsional)

// // 	// Sesuaikan path import ini jika struktur direktori/nama modul Anda berbeda
// // 	"github.com/Starath/Tubes2_BE_SayMyName/loadrecipes"
// // 	"github.com/Starath/Tubes2_BE_SayMyName/pathfinding"
// // 	"github.com/Starath/Tubes2_BE_SayMyName/pathfinding/bfs"
// // 	"github.com/Starath/Tubes2_BE_SayMyName/scrape"
// // )

// // // Helper function untuk mencetak hasil path (agar tidak duplikat kode)
// // func printPathResult(algorithmName string, target string, path []pathfinding.PathStep, nodesVisited int, duration time.Duration) {
// // 	fmt.Printf("--- Hasil %s untuk: %s ---\n", algorithmName, target)
// // 	fmt.Printf("Waktu Eksekusi: %s\n", duration)
// // 	fmt.Printf("Nodes Explored (States for BFS-Multi): %d\n", nodesVisited)
// // 	if len(path) == 0 {
// // 		if graphInstance.BaseElements[target] { // Perlu akses ke graphInstance
// // 			fmt.Println("- Elemen dasar.")
// // 		} else {
// // 			fmt.Println("- Tidak ada langkah resep yang ditemukan atau path kosong.")
// // 		}
// // 	} else {
// // 		fmt.Printf("Jumlah Langkah Resep: %d\n", len(path))
// // 		fmt.Println("Resep:")
// // 		for i, step := range path {
// // 			fmt.Printf("  %d. %s = %s + %s\n", i+1, step.ChildName, step.Parent1Name, step.Parent2Name)
// // 		}
// // 	}
// // 	fmt.Println(strings.Repeat("-", 30))
// // }

// // var graphInstance *loadrecipes.BiGraphAlchemy

// // func main() {
// // 	// --- 0. Jalankan Scraper untuk Memperbarui Data ---
// // 	fmt.Println("===== MEMULAI PROSES SCRAPING DATA ELEMEN =====")
// // 	// Panggil fungsi Scrapping dari package scrape
// // 	// Pastikan ini tidak menyebabkan circular dependency jika scrape juga mengimpor sesuatu dari main (tidak umum)
// // 	scrape.Scrapping() // <-- PANGGIL FUNGSI SCRAPPING DI SINI
// // 	fmt.Println("===== PROSES SCRAPING DATA ELEMEN SELESAI =====")

// // 	reader := bufio.NewReader(os.Stdin)
	
// // 	fmt.Println("Memuat data resep dari elements_filtered.json...")
// // 	var err error
// // 	graphInstance, err = loadrecipes.LoadBiGraph("elements_filtered.json") // Menggunakan file JSON default
// // 	if err != nil {
// // 		log.Fatalf("FATAL: Gagal memuat data resep: %v", err)
// // 	}
// // 	fmt.Println("Data resep berhasil dimuat.")

// // 	// targets := []string{
// // 	// 	"Brick",    
// // 	// 	"Mud",      
// // 	// 	"Fire",     
// // 	// 	"Steam",    
// // 	// 	"Picnic", // Bisa sangat lambat untuk banyak path dengan BFS ini
// // 	// }
// // 	// numPathsToFind := 1 

// // 	for {
// // 		fmt.Print("\nMasukkan target resep: ")
// // 		target, _ := reader.ReadString('\n')
// // 		target = strings.TrimSpace(target)
// // 		if target == "" {
// // 			break
// // 		}

// // 		fmt.Print("\nMasukkan jumlah jalur yang ingin dicari (default: 1): ")
// // 		numPathsToFindStr, _ := reader.ReadString('\n')
// // 		numPathsToFindStr = strings.TrimSpace(numPathsToFindStr)
// // 		if numPathsToFindStr == "" {
// // 			numPathsToFindStr = "1"
// // 		}
// // 		numPathsToFind, err := strconv.Atoi(numPathsToFindStr)
// // 		if err != nil {
// // 			numPathsToFind = 1
// // 		}

// // 		fmt.Printf("\n===== MEMPROSES TARGET: %s (Mencari %d Jalur BFS Mundur) =====\n", target, numPathsToFind)

// // 		startBFSPaths := time.Now()
// // 		resultBFSPaths, errBFSPaths := bfs.BFSFindXDifferentPathsBackward(graphInstance, target, numPathsToFind)
// // 		durationBFSPaths := time.Since(startBFSPaths)

// import (
// 	"bufio"
// 	"bufio"
// 	"fmt"
// 	"log"
// 	"os"
// 	"strconv"
// 	"os"
// 	"strconv"
// 	"strings"
// 	"time"

// 	"github.com/Starath/Tubes2_BE_SayMyName/loadrecipes" // Sesuaikan dengan path package Anda
// 	"github.com/Starath/Tubes2_BE_SayMyName/pathfinding" // Sesuaikan dengan path package Anda
// 	"github.com/Starath/Tubes2_BE_SayMyName/pathfinding/dfs" // Sesuaikan dengan path package Anda
// )

// // Variabel global untuk menyimpan data graf yang dimuat
// var graphInstanceDFS *loadrecipes.BiGraphAlchemy

// // Helper function untuk mencetak hasil MultipleResult dari DFS
// func printMultipleDFSPathResult(
// 	algorithmName string,
// 	target string,
// 	multipleResult *pathfinding.MultipleResult,
// 	totalNodesVisitedByWorkers int, // Parameter baru untuk total node yang dikunjungi
// 	duration time.Duration,
// 	maxRecipes int,
// ) {
// 	if multipleResult == nil || len(multipleResult.Results) == 0 {
// 		fmt.Printf("--- Hasil %s untuk: %s (Maks: %d jalur) ---\n", algorithmName, target, maxRecipes)
// 		fmt.Printf("Waktu Eksekusi Total: %s\n", duration)
// 		if graphInstanceDFS.BaseElements[target] {
// 			fmt.Println("- Elemen dasar (tidak ada path resep).")
// 			fmt.Printf("Total Nodes Explored oleh Worker (jika ada): %d\n", 1) // Elemen dasar hanya 1 node
// 		} else {
// 			fmt.Println("- Tidak ditemukan jalur resep.")
// 			fmt.Printf("Total Nodes Explored oleh Worker (yang berkontribusi pada path unik): %d\n", totalNodesVisitedByWorkers)
// 		}
// 		fmt.Println(strings.Repeat("-", 40))
// 		return
// 	}

// 	fmt.Printf("--- Hasil %s untuk: %s (Ditemukan %d dari maks %d jalur) ---\n",
// 		algorithmName, target, len(multipleResult.Results), maxRecipes)
// 	fmt.Printf("Waktu Eksekusi Total: %s\n", duration)
// 	fmt.Printf("Total Nodes Explored oleh Worker (yang berkontribusi pada path unik): %d\n", totalNodesVisitedByWorkers)
// 	fmt.Println(strings.Repeat("-", 40))

// 	for i, res := range multipleResult.Results {
// 		fmt.Printf("Jalur %d/%d (Nodes Explored oleh Worker ini: %d):\n", i+1, len(multipleResult.Results), res.NodesVisited)
// 		if len(res.Path) == 0 {
// 			if graphInstanceDFS.BaseElements[target] {
// 				fmt.Println("  - Elemen dasar.")
// 			} else {
// 				fmt.Println("  - Path kosong (kemungkinan elemen dasar atau error internal).")
// 			}
// 		} else {
// 			fmt.Printf("  Jumlah Langkah Resep: %d\n", len(res.Path))
// 			fmt.Println("  Resep:")
// 			for stepIdx, step := range res.Path {
// 				fmt.Printf("    %d. %s = %s + %s\n", stepIdx+1, step.ChildName, step.Parent1Name, step.Parent2Name)
// 			}
// 		}
// 		fmt.Println(strings.Repeat(".", 30))
// 	}
// 	fmt.Println(strings.Repeat("=", 40))
// }

// func main() {
// 	// --- 1. Muat Data Resep ---
// 	fmt.Println("Memuat data resep dari elements_filtered.json...")
// 	var err error
// 	// Pastikan path ke file JSON sudah benar
// 	graphInstanceDFS, err = loadrecipes.LoadBiGraph("elements_filtered.json")
// 	if err != nil {
// 		log.Fatalf("FATAL: Gagal memuat data resep: %v", err)
// 	}
// 	if graphInstanceDFS == nil {
// 		log.Fatalf("FATAL: graphInstanceDFS adalah nil setelah LoadBiGraph.")
// 	}
// 	fmt.Printf("Data resep berhasil dimuat. Total Elemen: %d\n", len(graphInstanceDFS.AllElements))
// 	fmt.Println(strings.Repeat("=", 40))

// 	// --- 2. Loop untuk Input Pengguna ---
// 	reader := bufio.NewReader(os.Stdin)
// 	for {
// 		fmt.Print("\nMasukkan elemen target (atau 'exit' untuk keluar): ")
// 		targetElement, _ := reader.ReadString('\n')
// 		targetElement = strings.TrimSpace(targetElement)

// 		if strings.ToLower(targetElement) == "exit" {
// 			break
// 		}
// 		if targetElement == "" {
// 			fmt.Println("Nama elemen tidak boleh kosong.")
// 			continue
// 		}

// 		if _, exists := graphInstanceDFS.AllElements[targetElement]; !exists {
// 			fmt.Printf("Elemen '%s' tidak ditemukan dalam data. Silakan coba lagi.\n", targetElement)
// 			continue
// 		}

// 		fmt.Print("Masukkan jumlah maksimum resep yang ingin dicari (contoh: 1, 3, 5): ")
// 		maxRecipesStr, _ := reader.ReadString('\n')
// 		maxRecipesStr = strings.TrimSpace(maxRecipesStr)
// >>>>>>> 989cdfac46f8c3ffad18043a98929a775fff7186

// 		maxRecipes, err := strconv.Atoi(maxRecipesStr)
// 		if err != nil || maxRecipes <= 0 {
// 			fmt.Println("Input tidak valid untuk jumlah maksimum resep, menggunakan default = 1.")
// 			maxRecipes = 1
// 		}

// 		fmt.Printf("\n===== Mencari %d jalur resep untuk '%s' menggunakan DFS Multiple Recipes =====\n", maxRecipes, targetElement)

// <<<<<<< HEAD
// // 		if errBFSPaths != nil {
// // 			fmt.Printf("--- Hasil BFS Mundur (X Jalur) untuk: %s ---\n", target)
// // 			fmt.Printf("Waktu Eksekusi: %s\n", durationBFSPaths)
// // 			fmt.Printf("Error: %v\n", errBFSPaths)
// // 			fmt.Println(strings.Repeat("-", 30))
// // 		} else {
// // 			if resultBFSPaths == nil || len(resultBFSPaths.Results) == 0 {
// // 				fmt.Printf("--- Hasil BFS Mundur (X Jalur) untuk: %s ---\n", target)
// // 				fmt.Printf("Waktu Eksekusi: %s\n", durationBFSPaths)
// // 				if graphInstance.BaseElements[target] {
// // 					fmt.Println("- Elemen dasar (tidak ada path resep).")
// // 					fmt.Printf("Nodes Explored (States for BFS-Multi): %d\n", 1) 
// // 				} else {
// // 					fmt.Println("- Tidak ditemukan path resep.")
// //                     nodesExplored := 0
// //                     if resultBFSPaths != nil && len(resultBFSPaths.Results) > 0 {
// //                         nodesExplored = resultBFSPaths.Results[0].NodesVisited
// //                     }
// //                      fmt.Printf("Nodes Explored (States for BFS-Multi): %d\n", nodesExplored)

// // 				}
// // 				fmt.Println(strings.Repeat("-", 30))
// // 			} else {
// // 				fmt.Printf("Ditemukan %d jalur berbeda untuk %s (maks %d):\n", len(resultBFSPaths.Results), target, numPathsToFind)
// // 				for i, res := range resultBFSPaths.Results {
// // 					printPathResult(fmt.Sprintf("BFS Mundur Jalur %d/%d", i+1, len(resultBFSPaths.Results)), target, res.Path, res.NodesVisited, durationBFSPaths) // Durasi adalah total
// // 				}
// // 			}
// // 		}
// //         fmt.Println("\n====================================================\n")
// // 	}
// // 	fmt.Println("\n===== Selesai =====")
// // }
// =======
// 		// --- 3. Jalankan DFSFindMultiplePathsString ---
// 		startTime := time.Now()
// 		// DFSFindMultiplePathsString mengembalikan: (result, totalNodesVisitedByWorkers, error)
// 		results, totalNodesVisited, errDFS := dfs.DFSFindMultiplePathsString(graphInstanceDFS, targetElement, maxRecipes)
// 		duration := time.Since(startTime)

// 		// --- 4. Tampilkan Hasil ---
// 		if errDFS != nil {
// 			fmt.Printf("Error saat mencari resep DFS Multiple untuk %s: %v\n", targetElement, errDFS)
// 			fmt.Printf("Waktu Eksekusi: %s\n", duration)
// 			fmt.Printf("Total Nodes Explored oleh Worker (jika ada sebelum error): %d\n", totalNodesVisited)
// 		} else {
// 			printMultipleDFSPathResult("DFS Multiple Recipes", targetElement, results, totalNodesVisited, duration, maxRecipes)
// 		}
// 		fmt.Println(strings.Repeat("=", 40))
// 	}

// 	fmt.Println("\nProgram selesai.")
// }
// >>>>>>> 989cdfac46f8c3ffad18043a98929a775fff7186
