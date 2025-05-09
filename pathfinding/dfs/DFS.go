package dfs

import (
	"container/list"
	"fmt"
	"log"

	"github.com/Starath/Tubes2_BE_SayMyName/loadrecipes"
	"github.com/Starath/Tubes2_BE_SayMyName/pathfinding"
)

// dfsRecursiveHelperString adalah fungsi rekursif inti untuk DFS mundur (versi string).
// Mengembalikan true jika path ditemukan, false jika tidak.
// pathSteps: map untuk menyimpan langkah-langkah resep yang ditemukan (hanya langkah terakhir per elemen).
// currentlySolving: set untuk deteksi siklus dalam path rekursif saat ini.
// memo: map untuk menyimpan hasil elemen yang sudah dihitung (true=dapat dibuat, false=tidak dapat dibuat).
// visitedCounter: pointer ke integer untuk menghitung node unik yang dieksplorasi.
func dfsRecursiveHelperString(
	elementName string,
	graph *loadrecipes.BiGraphAlchemy,
	pathSteps map[string]pathfinding.PathStep, // map[childName]pathfinding.PathStep
	currentlySolving map[string]bool,
	memo map[string]bool,
	visitedCounter *int,
) bool {
	// 1. Cek Memoization dan Hitung Kunjungan Unik
	if canBeMade, exists := memo[elementName]; exists {
		return canBeMade // Kembalikan hasil yg sudah disimpan
	}
	// Jika belum ada di memo, berarti ini eksplorasi pertama untuk node ini
	*visitedCounter++

	// 2. Cek Base Case (Elemen Dasar)
	if graph.BaseElements[elementName] {
		memo[elementName] = true // Elemen dasar bisa "dibuat"
		return true
	}

	// 3. Cek Siklus
	if currentlySolving[elementName] {
		// Tidak simpan di memo karena ini hanya siklus di path *saat ini*
		return false // Siklus terdeteksi
	}

	// 4. Tandai sedang diselesaikan
	currentlySolving[elementName] = true
	// 'defer' akan dijalankan tepat sebelum fungsi return
	defer delete(currentlySolving, elementName) // Hapus tanda saat backtrack

	// 5. Cek apakah elemen punya resep (menggunakan map mundur)
	parentPairs, hasRecipes := graph.ChildToParents[elementName]
	if !hasRecipes {
		memo[elementName] = false // Tidak bisa dibuat jika tidak ada resep
		return false
	}

	// 6. Coba setiap resep secara rekursif
	foundPath := false // Flag apakah salah satu resep berhasil
	for _, pair := range parentPairs {
		// Coba selesaikan parent 1
		canMakeP1 := dfsRecursiveHelperString(pair.Mat1, graph, pathSteps, currentlySolving, memo, visitedCounter)
		if !canMakeP1 {
			continue // Jika parent 1 tidak bisa dibuat, coba resep lain
		}

		// Coba selesaikan parent 2
		canMakeP2 := dfsRecursiveHelperString(pair.Mat2, graph, pathSteps, currentlySolving, memo, visitedCounter)
		if !canMakeP2 {
			continue // Jika parent 2 tidak bisa dibuat, coba resep lain
		}

		// Jika KEDUA parent bisa dibuat
		if canMakeP1 && canMakeP2 {
			// Simpan langkah resep ini (langkah terakhir yg berhasil utk elemen ini)
			pathSteps[elementName] = pathfinding.PathStep{ChildName: elementName, Parent1Name: pair.Mat1, Parent2Name: pair.Mat2}
			foundPath = true // Setidaknya satu resep berhasil
			// PENTING: Untuk DFS dasar (cari 1 jalur), kita bisa langsung return true di sini
			// setelah menemukan satu cara. Jika ingin mencari semua jalur atau jalur 'terbaik'
			// menurut kriteria lain, kita tidak return di sini tapi lanjutkan loop.
			break // Hentikan loop resep setelah menemukan 1 cara valid
		}
	}

	// 7. Simpan hasil ke memo berdasarkan flag foundPath
	memo[elementName] = foundPath
	return foundPath
}

// DFSFindPathString memulai pencarian DFS mundur (versi string untuk satu jalur).
func DFSFindPathString(graph *loadrecipes.BiGraphAlchemy, targetElementName string) (*pathfinding.Result, error) {
	if _, targetExists := graph.AllElements[targetElementName]; !targetExists {
		return nil, fmt.Errorf("elemen target '%s' tidak ditemukan", targetElementName)
	}

	if graph.BaseElements[targetElementName] {
		return &pathfinding.Result{Path: []pathfinding.PathStep{}, NodesVisited: 1}, nil // Elemen dasar
	}

	pathSteps := make(map[string]pathfinding.PathStep)   // Hanya menyimpan langkah terakhir per elemen
	currentlySolving := make(map[string]bool) // Deteksi siklus per path
	memo := make(map[string]bool)           // Memoization global untuk run ini
	visitedCount := 0                       // Counter node unik yg dieksplorasi

	success := dfsRecursiveHelperString(targetElementName, graph, pathSteps, currentlySolving, memo, &visitedCount)

	if success {
		// Rekonstruksi path lengkap dari pathSteps menggunakan fungsi utilitas bersama
		finalPath := reconstructFullPathFromSteps(pathSteps, targetElementName, graph.BaseElements)
		return &pathfinding.Result{Path: finalPath, NodesVisited: visitedCount}, nil
	}

	// Jika success == false
	nodesExploredFinal := visitedCount
	if !memo[targetElementName] {
		 // Pastikan hitungan visited sudah final jika gagal dan target bukan base
		 // dan belum pernah di-visit (masuk ke memo)
		if _, inMemo := memo[targetElementName]; !inMemo && !graph.BaseElements[targetElementName] {
			nodesExploredFinal++
		}
		log.Printf("INFO: Elemen '%s' ditandai tidak dapat dibuat (memo=false) oleh DFSFindPathString.\n", targetElementName)
	}

	return nil, fmt.Errorf("tidak ditemukan jalur resep untuk elemen '%s' menggunakan DFSFindPathString (Nodes Explored: %d)", targetElementName, nodesExploredFinal)
}

func reconstructFullPathFromSteps(
	steps map[string]pathfinding.PathStep, // Map langkah resep yang spesifik untuk satu jalur yang ditemukan
	targetElementName string,
	baseElements map[string]bool,
) []pathfinding.PathStep {
	var path []pathfinding.PathStep
	queue := list.New() 
	
	if _, exists := steps[targetElementName]; !exists && !baseElements[targetElementName] {
		if targetElementName != "" { 
			// log.Printf("[RECONSTRUCT_WARN] Target '%s' tidak ditemukan dalam steps dan bukan elemen dasar saat rekonstruksi.", targetElementName)
		}
		return path 
	}

	queue.PushBack(targetElementName)
	processedForThisPathReconstruction := make(map[string]bool) 

	for queue.Len() > 0 {
		currentElementName := queue.Remove(queue.Front()).(string)

		if baseElements[currentElementName] || processedForThisPathReconstruction[currentElementName] {
			continue
		}

		step, exists := steps[currentElementName]
		if !exists {
			// log.Printf("[RECONSTRUCT_ERROR] Tidak ada langkah resep yang tercatat untuk elemen non-dasar '%s' saat rekonstruksi jalur.", currentElementName)
			continue 
		}
		
		path = append(path, step) 
		processedForThisPathReconstruction[currentElementName] = true 

		if !baseElements[step.Parent1Name] && !processedForThisPathReconstruction[step.Parent1Name] {
			queue.PushBack(step.Parent1Name)
		}
		if !baseElements[step.Parent2Name] && !processedForThisPathReconstruction[step.Parent2Name] {
			queue.PushBack(step.Parent2Name)
		}
	}

	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}

// --- Contoh Penggunaan ---
// func main() {
//   // --- (Pastikan fungsi LoadFlexibleRecipes dipanggil sebelum ini) ---
//    recipeData, err := loadrecipes.LoadBiGraph("elements.json")
//    if err != nil {
//      log.Fatalf("FATAL: Gagal memuat data resep: %v", err)
//    }

//    fmt.Println("\n--- DFS SEARCH (Single-Threaded) ---")
//    targetDFS := "Picnic" // Ganti dengan target yg diinginkan
//    resultDFS, errDFS := DFSFindPathString(recipeData, targetDFS)
//    if errDFS != nil {
//      fmt.Printf("Error mencari resep DFS untuk %s: %v\n", targetDFS, errDFS)
//    } else {
//      fmt.Printf("Resep DFS (salah satu jalur) untuk %s (Nodes Explored: %d):\n", targetDFS, resultDFS.NodesVisited)
//      if len(resultDFS.Path) == 0 {
//        fmt.Println("- Elemen dasar.")
//      } else {
//        // Tampilkan path (urutan sudah dibalik)
//        for _, step := range resultDFS.Path {
//          fmt.Printf("  %s = %s + %s\n", step.ChildName, step.Parent1Name, step.Parent2Name)
//        }
//      }
//    }

//    fmt.Println("\n-------------------\n")

//    targetDFS = "Hedgehog" // Contoh lain
//    resultDFS, errDFS = DFSFindPathString(recipeData, targetDFS)
//    if errDFS != nil {
//      fmt.Printf("Error mencari resep DFS untuk %s: %v\n", targetDFS, errDFS)
//    } else {
//      fmt.Printf("Resep DFS (salah satu jalur) untuk %s (Nodes Explored: %d):\n", targetDFS, resultDFS.NodesVisited)
//      if len(resultDFS.Path) == 0 {
//        fmt.Println("- Elemen dasar.")
//      } else {
//        for _, step := range resultDFS.Path {
//          fmt.Printf("  %s = %s + %s\n", step.ChildName, step.Parent1Name, step.Parent2Name)
//        }
//      }
//    }
// }