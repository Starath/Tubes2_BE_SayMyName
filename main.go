// File: main.go
package main

import (
	"bufio"
	"strconv"
	"os"
	"fmt"
	"log"
	"strings"
	"time" // Untuk mengukur waktu eksekusi (opsional)

	// Sesuaikan path import ini jika struktur direktori/nama modul Anda berbeda
	"github.com/Starath/Tubes2_BE_SayMyName/loadrecipes"
	"github.com/Starath/Tubes2_BE_SayMyName/pathfinding"
	"github.com/Starath/Tubes2_BE_SayMyName/pathfinding/bfs"
	"github.com/Starath/Tubes2_BE_SayMyName/scrape"
)

// Helper function untuk mencetak hasil path (agar tidak duplikat kode)
func printPathResult(algorithmName string, target string, path []pathfinding.PathStep, nodesVisited int, duration time.Duration) {
	fmt.Printf("--- Hasil %s untuk: %s ---\n", algorithmName, target)
	fmt.Printf("Waktu Eksekusi: %s\n", duration)
	fmt.Printf("Nodes Explored (States for BFS-Multi): %d\n", nodesVisited)
	if len(path) == 0 {
		if graphInstance.BaseElements[target] { // Perlu akses ke graphInstance
			fmt.Println("- Elemen dasar.")
		} else {
			fmt.Println("- Tidak ada langkah resep yang ditemukan atau path kosong.")
		}
	} else {
		fmt.Printf("Jumlah Langkah Resep: %d\n", len(path))
		fmt.Println("Resep:")
		for i, step := range path {
			fmt.Printf("  %d. %s = %s + %s\n", i+1, step.ChildName, step.Parent1Name, step.Parent2Name)
		}
	}
	fmt.Println(strings.Repeat("-", 30))
}

var graphInstance *loadrecipes.BiGraphAlchemy

func main() {
	// --- 0. Jalankan Scraper untuk Memperbarui Data ---
	fmt.Println("===== MEMULAI PROSES SCRAPING DATA ELEMEN =====")
	// Panggil fungsi Scrapping dari package scrape
	// Pastikan ini tidak menyebabkan circular dependency jika scrape juga mengimpor sesuatu dari main (tidak umum)
	scrape.Scrapping() // <-- PANGGIL FUNGSI SCRAPPING DI SINI
	fmt.Println("===== PROSES SCRAPING DATA ELEMEN SELESAI =====")

	reader := bufio.NewReader(os.Stdin)
	
	fmt.Println("Memuat data resep dari elements.json...")
	var err error
	graphInstance, err = loadrecipes.LoadBiGraph("elements.json") // Menggunakan file JSON default
	if err != nil {
		log.Fatalf("FATAL: Gagal memuat data resep: %v", err)
	}
	fmt.Println("Data resep berhasil dimuat.")

	// targets := []string{
	// 	"Brick",    
	// 	"Mud",      
	// 	"Fire",     
	// 	"Steam",    
	// 	"Picnic", // Bisa sangat lambat untuk banyak path dengan BFS ini
	// }
	// numPathsToFind := 1 

	for {
		fmt.Print("\nMasukkan target resep: ")
		target, _ := reader.ReadString('\n')
		target = strings.TrimSpace(target)
		if target == "" {
			break
		}

		fmt.Print("\nMasukkan jumlah jalur yang ingin dicari (default: 1): ")
		numPathsToFindStr, _ := reader.ReadString('\n')
		numPathsToFindStr = strings.TrimSpace(numPathsToFindStr)
		if numPathsToFindStr == "" {
			numPathsToFindStr = "1"
		}
		numPathsToFind, err := strconv.Atoi(numPathsToFindStr)
		if err != nil {
			numPathsToFind = 1
		}

		fmt.Printf("\n===== MEMPROSES TARGET: %s (Mencari %d Jalur BFS Mundur) =====\n", target, numPathsToFind)

		startBFSPaths := time.Now()
		resultBFSPaths, errBFSPaths := bfs.BFSFindXDifferentPathsBackward(graphInstance, target, numPathsToFind)
		durationBFSPaths := time.Since(startBFSPaths)



		if errBFSPaths != nil {
			fmt.Printf("--- Hasil BFS Mundur (X Jalur) untuk: %s ---\n", target)
			fmt.Printf("Waktu Eksekusi: %s\n", durationBFSPaths)
			fmt.Printf("Error: %v\n", errBFSPaths)
			fmt.Println(strings.Repeat("-", 30))
		} else {
			if resultBFSPaths == nil || len(resultBFSPaths.Results) == 0 {
				fmt.Printf("--- Hasil BFS Mundur (X Jalur) untuk: %s ---\n", target)
				fmt.Printf("Waktu Eksekusi: %s\n", durationBFSPaths)
				if graphInstance.BaseElements[target] {
					fmt.Println("- Elemen dasar (tidak ada path resep).")
					fmt.Printf("Nodes Explored (States for BFS-Multi): %d\n", 1) 
				} else {
					fmt.Println("- Tidak ditemukan path resep.")
                    nodesExplored := 0
                    if resultBFSPaths != nil && len(resultBFSPaths.Results) > 0 {
                        nodesExplored = resultBFSPaths.Results[0].NodesVisited
                    }
                     fmt.Printf("Nodes Explored (States for BFS-Multi): %d\n", nodesExplored)

				}
				fmt.Println(strings.Repeat("-", 30))
			} else {
				fmt.Printf("Ditemukan %d jalur berbeda untuk %s (maks %d):\n", len(resultBFSPaths.Results), target, numPathsToFind)
				for i, res := range resultBFSPaths.Results {
					printPathResult(fmt.Sprintf("BFS Mundur Jalur %d/%d", i+1, len(resultBFSPaths.Results)), target, res.Path, res.NodesVisited, durationBFSPaths) // Durasi adalah total
				}
			}
		}
        fmt.Println("\n====================================================\n")
	}
	fmt.Println("\n===== Selesai =====")
}