// File: main.go
package main

import (
	"fmt"
	"log"
	"strings"
	"time" // Untuk mengukur waktu eksekusi (opsional)

	// Sesuaikan path import ini jika struktur direktori/nama modul Anda berbeda
	"github.com/Starath/Tubes2_BE_SayMyName/loadrecipes"
	"github.com/Starath/Tubes2_BE_SayMyName/pathfinding"
	"github.com/Starath/Tubes2_BE_SayMyName/pathfinding/bfs"
	"github.com/Starath/Tubes2_BE_SayMyName/pathfinding/dfs"
)

// Helper function untuk mencetak hasil path (agar tidak duplikat kode)
func printPathResult(algorithmName string, target string, path []pathfinding.PathStep, nodesVisited int, duration time.Duration) {
	fmt.Printf("--- Hasil %s untuk: %s ---\n", algorithmName, target)
	fmt.Printf("Waktu Eksekusi: %s\n", duration) // Tampilkan durasi
	fmt.Printf("Nodes Explored: %d\n", nodesVisited)
	if len(path) == 0 {
		// Cek apakah memang elemen dasar atau tidak ada path
		// (BFS/DFS sudah menangani error jika tidak ditemukan, jadi ini biasanya elemen dasar)
		fmt.Println("- Elemen dasar atau tidak ada langkah resep.")
	} else {
		fmt.Printf("Jumlah Langkah Resep: %d\n", len(path))
		fmt.Println("Resep:")
		for i, step := range path {
			fmt.Printf("  %d. %s = %s + %s\n", i+1, step.ChildName, step.Parent1Name, step.Parent2Name)
		}
	}
	fmt.Println(strings.Repeat("-", 30)) // Separator
}

func main() {
	// --- 1. Muat Data Resep ---
	fmt.Println("Memuat data resep dari elements.json...")
	// Pastikan file 'elements.json' ada di direktori yang sama dengan main.go saat dijalankan,
	// atau gunakan path absolut/relatif yang benar.
	recipeData, err := loadrecipes.LoadBiGraph("elements.json")
	if err != nil {
		log.Fatalf("FATAL: Gagal memuat data resep: %v", err)
	}
	fmt.Println("Data resep berhasil dimuat.")

	// --- 2. Tentukan Target Elemen ---
	targets := []string{
		"Fireworks",          // Contoh elemen kompleks
		"Warmth",           // Contoh lain
		"Wall",        // Contoh dengan jalur berbeda mungkin
		"Water",           // Contoh elemen dasar
		"Wind", // Contoh elemen tidak ada
		"Picnic",
	}

	// --- 3. Jalankan & Bandingkan DFS dan BFS untuk setiap target ---
	for _, target := range targets {
		fmt.Printf("\n===== MEMPROSES TARGET: %s =====\n", target)

		// --- Jalankan DFS ---
		startDFS := time.Now()
		resultDFS, errDFS := dfs.DFSFindPathString(recipeData, target)
		durationDFS := time.Since(startDFS)

		if errDFS != nil {
			fmt.Printf("--- Hasil DFS untuk: %s ---\n", target)
			fmt.Printf("Waktu Eksekusi: %s\n", durationDFS)
			fmt.Printf("Error: %v\n", errDFS)
			fmt.Println(strings.Repeat("-", 30))
		} else {
			printPathResult("DFS (Salah Satu Jalur)", target, resultDFS.Path, resultDFS.NodesVisited, durationDFS)
		}

		// Beri sedikit jeda jika perlu (opsional)
		// time.Sleep(100 * time.Millisecond)

		// --- Jalankan BFS ---
		startBFS := time.Now()
		resultBFS, errBFS := bfs.BFSFindShortestPathString(recipeData, target)
		durationBFS := time.Since(startBFS)

		if errBFS != nil {
			fmt.Printf("--- Hasil BFS untuk: %s ---\n", target)
			fmt.Printf("Waktu Eksekusi: %s\n", durationBFS)
			fmt.Printf("Error: %v\n", errBFS)
			fmt.Println(strings.Repeat("-", 30))
		} else {
			printPathResult("BFS (Jalur Terpendek)", target, resultBFS.Path, resultBFS.NodesVisited, durationBFS)
		}
	}

	fmt.Println("\n===== Selesai =====")
}