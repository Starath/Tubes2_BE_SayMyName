package bfs

import (
	"container/list"
	"fmt"
	"log"

	"github.com/Starath/Tubes2_BE_SayMyName/loadrecipes"
	"github.com/Starath/Tubes2_BE_SayMyName/pathfinding"
)

// Definisikan pathfinding.PathStep di sini jika belum ada
// type pathfinding.PathStep struct {
// 	ChildName   string
// 	Parent1Name string
// 	Parent2Name string
// }

// Helper struct untuk menyimpan step yang membawa ke node (tidak jadi dipakai di versi ini)
// type predecessorInfo struct {
// 	step pathfinding.PathStep
// }

// Fungsi compare tidak jadi dipakai
// type byChildName []pathfinding.PathStep
// func (a byChildName) Len() int           { return len(a) }
// func (a byChildName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
// func (a byChildName) Less(i, j int) bool { return a[i].ChildName < a[j].ChildName }

// Helper function untuk construct pair, agar konsisten dengan loadrecipes
func constructPairBFS(mat1, mat2 string) loadrecipes.PairMats {
	if mat1 < mat2 {
		return loadrecipes.PairMats{Mat1: mat1, Mat2: mat2}
	}
	return loadrecipes.PairMats{Mat1: mat2, Mat2: mat1}
}


func BFSFindShortestPathString(graph *loadrecipes.BiGraphAlchemy, targetElementName string) (*pathfinding.Result, error) {
	if _, targetExists := graph.AllElements[targetElementName]; !targetExists {
		return nil, fmt.Errorf("elemen target '%s' tidak ditemukan dalam data", targetElementName)
	}

	if graph.BaseElements[targetElementName] {
		return &pathfinding.Result{Path: []pathfinding.PathStep{}, NodesVisited: 1}, nil
	}

	// --- Tahap 1: BFS Maju untuk Menemukan Jalur Terpendek & Mencatat Predecessor ---
	queue := list.New()
	visited := make(map[string]bool)        // Melacak elemen yang sudah bisa dibuat
	predecessorSteps := make(map[string]pathfinding.PathStep) // Menyimpan resep terpendek ke elemen
	nodesVisitedCount := 0
	foundTarget := false

	// Inisialisasi antrian dengan elemen dasar
	baseElementsList := make([]string, 0, len(graph.BaseElements)) // Simpan base elements untuk iterasi nanti
	for baseElem := range graph.BaseElements { // <-- FIX: Iterasi pada keys
		queue.PushBack(baseElem)
		visited[baseElem] = true
		nodesVisitedCount++
		baseElementsList = append(baseElementsList, baseElem)
	}

	// elemenYangSudahDikunjungi diperlukan untuk iterasi pasangan
	elemenYangSudahDikunjungi := make([]string, len(baseElementsList))
	copy(elemenYangSudahDikunjungi, baseElementsList)

	processedCount := 0 // Untuk debug, memastikan loop tidak infinite

	// Lakukan BFS maju
	for queue.Len() > 0 {
		processedCount++
		if processedCount > len(graph.AllElements)*len(graph.AllElements) { // Safety break
			log.Println("WARN: BFS loop exceeded expected iterations, possibly stuck.")
			break
		}

		elem := queue.Front()
		p1 := elem.Value.(string) // Parent 1
		queue.Remove(elem)

		// Coba kombinasikan p1 dengan semua elemen lain p2 yang sudah dikunjungi
		for _, p2 := range elemenYangSudahDikunjungi {
			// Bentuk pair (pastikan urutan konsisten)
			pair := constructPairBFS(p1, p2)

			// Cek apakah pair ini menghasilkan child
			child, producesChild := graph.ParentPairToChild[pair]

			if producesChild && !visited[child] { // Jika menghasilkan child BARU
				visited[child] = true
				nodesVisitedCount++
				predecessorSteps[child] = pathfinding.PathStep{
					ChildName:   child,
					Parent1Name: pair.Mat1, // Gunakan field dari pair yang sudah diurut
					Parent2Name: pair.Mat2,
				}
				queue.PushBack(child)
				elemenYangSudahDikunjungi = append(elemenYangSudahDikunjungi, child) // Tambahkan ke daftar elemen yg bisa jadi parent

				if child == targetElementName {
					foundTarget = true
					// Kita bisa break dari loop p2 karena target sudah ditemukan via p1
					// Tapi kita tidak break dari loop queue utama, agar level lain selesai
				}
			}
		} // End loop p2

		if foundTarget && queue.Len() > 0 {
			// Optimasi: Jika target sudah ditemukan, cek apakah elemen di queue
			// bisa jadi parent untuk *elemen lain* yang mungkin dibutuhkan
			// untuk rekonstruksi path target. Ini agak kompleks.
			// Untuk simplifikasi, kita bisa lanjutkan saja sampai queue habis,
			// atau stop jika target ditemukan (tapi mungkin node count kurang akurat).
			// Mari kita lanjutkan sampai queue habis untuk memastikan semua predecessor tercatat.
		}

	} // Akhir BFS Maju loop (queue)

	if !foundTarget {
		// Jika target tidak pernah di-visit, berarti tidak bisa dibuat
		// Cek apakah nodesVisitedCount sudah menghitung target atau belum
		finalNodesVisited := nodesVisitedCount
		if !visited[targetElementName] {
			// Jika target tidak di 'visited', tambahkan 1 ke hitungan eksplorasi
			// karena usaha mencarinya tetap dihitung.
			// Tapi ini debatable, tergantung definisi 'explored'. Kita biarkan saja.
			log.Printf("INFO: Target '%s' tidak tercapai oleh BFS maju.", targetElementName)
		}
		return nil, fmt.Errorf("tidak ditemukan jalur resep untuk elemen '%s' (Nodes Explored: %d)", targetElementName, finalNodesVisited)
	}


	// --- Tahap 2: Rekonstruksi Path dari Target Mundur Menggunakan Predecessor ---
	finalPathSteps := make([]pathfinding.PathStep, 0)
	reconstructionQueue := list.New()
	processedForPath := make(map[string]bool)

	if _, exists := predecessorSteps[targetElementName]; !exists && !graph.BaseElements[targetElementName]{
		 // Safety check jika target ditemukan tapi stepnya tidak tercatat
		 return nil, fmt.Errorf("internal error: target '%s' ditemukan tapi tidak ada step predecessor", targetElementName)
	}

	reconstructionQueue.PushBack(targetElementName)
	processedForPath[targetElementName] = true // Tandai target sudah masuk antrian rekonstruksi

	for reconstructionQueue.Len() > 0 {
		elem := reconstructionQueue.Front()
		childElement := elem.Value.(string)
		reconstructionQueue.Remove(elem)

		// Jika elemen ini adalah base, stop cabang ini
		if graph.BaseElements[childElement] {
			continue
		}

		// Ambil step yang *pasti* merupakan bagian dari shortest path
		step, stepExists := predecessorSteps[childElement]
		if !stepExists {
			log.Printf("WARNING: Predecessor step tidak ditemukan untuk '%s' saat rekonstruksi BFS Shortest Path.", childElement)
			continue // Lewati elemen ini jika step tidak ada
		}

		// Tambahkan step ke hasil akhir
		// Hindari duplikasi step jika struktur graph memungkinkan (meski BFS harusnya tidak)
		isDuplicateStep := false
		for _, existingStep := range finalPathSteps {
			if existingStep == step {
				isDuplicateStep = true
				break
			}
		}
		if !isDuplicateStep {
			finalPathSteps = append(finalPathSteps, step)
		}


		// Tambahkan parent ke antrian jika belum diproses
		if !graph.BaseElements[step.Parent1Name] && !processedForPath[step.Parent1Name] {
			processedForPath[step.Parent1Name] = true
			reconstructionQueue.PushBack(step.Parent1Name)
		}
		if !graph.BaseElements[step.Parent2Name] && !processedForPath[step.Parent2Name] {
			processedForPath[step.Parent2Name] = true
			reconstructionQueue.PushBack(step.Parent2Name)
		}
	}

	// Balikkan urutan slice agar sesuai alur pembuatan (Base -> Target)
	for i, j := 0, len(finalPathSteps)-1; i < j; i, j = i+1, j-1 {
		finalPathSteps[i], finalPathSteps[j] = finalPathSteps[j], finalPathSteps[i]
	}

	// Pengecekan Akhir
	if len(finalPathSteps) == 0 && !graph.BaseElements[targetElementName] {
		return nil, fmt.Errorf("rekonstruksi menghasilkan path kosong untuk target non-dasar '%s'. Nodes Explored: %d", targetElementName, nodesVisitedCount)
	}

	// nodesVisitedCount mungkin sedikit lebih tinggi dari jumlah node di path final
	// karena BFS maju mengeksplorasi semua kemungkinan di setiap level.
	return &pathfinding.Result{Path: finalPathSteps, NodesVisited: nodesVisitedCount}, nil
}