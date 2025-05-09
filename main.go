package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Starath/Tubes2_BE_SayMyName/loadrecipes" // Sesuaikan dengan path package Anda
	"github.com/Starath/Tubes2_BE_SayMyName/pathfinding" // Sesuaikan dengan path package Anda
	"github.com/Starath/Tubes2_BE_SayMyName/pathfinding/dfs" // Sesuaikan dengan path package Anda
)

// Variabel global untuk menyimpan data graf yang dimuat
var graphInstanceDFS *loadrecipes.BiGraphAlchemy

// Helper function untuk mencetak hasil MultipleResult dari DFS
func printMultipleDFSPathResult(
	algorithmName string,
	target string,
	multipleResult *pathfinding.MultipleResult,
	totalNodesVisitedByWorkers int, // Parameter baru untuk total node yang dikunjungi
	duration time.Duration,
	maxRecipes int,
) {
	if multipleResult == nil || len(multipleResult.Results) == 0 {
		fmt.Printf("--- Hasil %s untuk: %s (Maks: %d jalur) ---\n", algorithmName, target, maxRecipes)
		fmt.Printf("Waktu Eksekusi Total: %s\n", duration)
		if graphInstanceDFS.BaseElements[target] {
			fmt.Println("- Elemen dasar (tidak ada path resep).")
			fmt.Printf("Total Nodes Explored oleh Worker (jika ada): %d\n", 1) // Elemen dasar hanya 1 node
		} else {
			fmt.Println("- Tidak ditemukan jalur resep.")
			fmt.Printf("Total Nodes Explored oleh Worker (yang berkontribusi pada path unik): %d\n", totalNodesVisitedByWorkers)
		}
		fmt.Println(strings.Repeat("-", 40))
		return
	}

	fmt.Printf("--- Hasil %s untuk: %s (Ditemukan %d dari maks %d jalur) ---\n",
		algorithmName, target, len(multipleResult.Results), maxRecipes)
	fmt.Printf("Waktu Eksekusi Total: %s\n", duration)
	fmt.Printf("Total Nodes Explored oleh Worker (yang berkontribusi pada path unik): %d\n", totalNodesVisitedByWorkers)
	fmt.Println(strings.Repeat("-", 40))

	for i, res := range multipleResult.Results {
		fmt.Printf("Jalur %d/%d (Nodes Explored oleh Worker ini: %d):\n", i+1, len(multipleResult.Results), res.NodesVisited)
		if len(res.Path) == 0 {
			if graphInstanceDFS.BaseElements[target] {
				fmt.Println("  - Elemen dasar.")
			} else {
				fmt.Println("  - Path kosong (kemungkinan elemen dasar atau error internal).")
			}
		} else {
			fmt.Printf("  Jumlah Langkah Resep: %d\n", len(res.Path))
			fmt.Println("  Resep:")
			for stepIdx, step := range res.Path {
				fmt.Printf("    %d. %s = %s + %s\n", stepIdx+1, step.ChildName, step.Parent1Name, step.Parent2Name)
			}
		}
		fmt.Println(strings.Repeat(".", 30))
	}
	fmt.Println(strings.Repeat("=", 40))
}

func main() {
	// --- 1. Muat Data Resep ---
	fmt.Println("Memuat data resep dari elements_filtered.json...")
	var err error
	// Pastikan path ke file JSON sudah benar
	graphInstanceDFS, err = loadrecipes.LoadBiGraph("elements_filtered.json")
	if err != nil {
		log.Fatalf("FATAL: Gagal memuat data resep: %v", err)
	}
	if graphInstanceDFS == nil {
		log.Fatalf("FATAL: graphInstanceDFS adalah nil setelah LoadBiGraph.")
	}
	fmt.Printf("Data resep berhasil dimuat. Total Elemen: %d\n", len(graphInstanceDFS.AllElements))
	fmt.Println(strings.Repeat("=", 40))

	// --- 2. Loop untuk Input Pengguna ---
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("\nMasukkan elemen target (atau 'exit' untuk keluar): ")
		targetElement, _ := reader.ReadString('\n')
		targetElement = strings.TrimSpace(targetElement)

		if strings.ToLower(targetElement) == "exit" {
			break
		}
		if targetElement == "" {
			fmt.Println("Nama elemen tidak boleh kosong.")
			continue
		}

		if _, exists := graphInstanceDFS.AllElements[targetElement]; !exists {
			fmt.Printf("Elemen '%s' tidak ditemukan dalam data. Silakan coba lagi.\n", targetElement)
			continue
		}

		fmt.Print("Masukkan jumlah maksimum resep yang ingin dicari (contoh: 1, 3, 5): ")
		maxRecipesStr, _ := reader.ReadString('\n')
		maxRecipesStr = strings.TrimSpace(maxRecipesStr)

		maxRecipes, err := strconv.Atoi(maxRecipesStr)
		if err != nil || maxRecipes <= 0 {
			fmt.Println("Input tidak valid untuk jumlah maksimum resep, menggunakan default = 1.")
			maxRecipes = 1
		}

		fmt.Printf("\n===== Mencari %d jalur resep untuk '%s' menggunakan DFS Multiple Recipes =====\n", maxRecipes, targetElement)

		// --- 3. Jalankan DFSFindMultiplePathsString ---
		startTime := time.Now()
		// DFSFindMultiplePathsString mengembalikan: (result, totalNodesVisitedByWorkers, error)
		results, totalNodesVisited, errDFS := dfs.DFSFindMultiplePathsString(graphInstanceDFS, targetElement, maxRecipes)
		duration := time.Since(startTime)

		// --- 4. Tampilkan Hasil ---
		if errDFS != nil {
			fmt.Printf("Error saat mencari resep DFS Multiple untuk %s: %v\n", targetElement, errDFS)
			fmt.Printf("Waktu Eksekusi: %s\n", duration)
			fmt.Printf("Total Nodes Explored oleh Worker (jika ada sebelum error): %d\n", totalNodesVisited)
		} else {
			printMultipleDFSPathResult("DFS Multiple Recipes", targetElement, results, totalNodesVisited, duration, maxRecipes)
		}
		fmt.Println(strings.Repeat("=", 40))
	}

	fmt.Println("\nProgram selesai.")
}