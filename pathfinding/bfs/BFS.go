package bfs

import (
	"container/list"
	"fmt"
	"log"
	"sort"

	"github.com/Starath/Tubes2_BE_SayMyName/loadrecipes"
	"github.com/Starath/Tubes2_BE_SayMyName/pathfinding"
)

// reversePathStepsBFS membalik urutan slice PathStep
func reversePathStepsBFS(steps []pathfinding.PathStep) {
	for i, j := 0, len(steps)-1; i < j; i, j = i+1, j-1 {
		steps[i], steps[j] = steps[j], steps[i]
	}
}

// normalizeAndRemoveDuplicateStepsBFS menormalisasi (urutan parent) dan menghapus duplikat step
func normalizePathSteps(steps []pathfinding.PathStep) []pathfinding.PathStep {
	if len(steps) == 0 {
		return steps
	}
	
	normalized := make([]pathfinding.PathStep, len(steps))
	copy(normalized, steps)
	for i := range normalized {
		if normalized[i].Parent1Name > normalized[i].Parent2Name {
			normalized[i].Parent1Name, normalized[i].Parent2Name = normalized[i].Parent2Name, normalized[i].Parent1Name
		}
	}
	return normalized
}


// createPathSignature membuat string unik untuk sebuah path agar bisa dideteksi duplikasinya.
// Path yang diberikan HARUS sudah dalam urutan base-ke-target.
func createPathSignature(steps []pathfinding.PathStep) string {
	pathCopy := make([]pathfinding.PathStep, len(steps))
	copy(pathCopy, steps)

	for i := range pathCopy {
		if pathCopy[i].Parent1Name > pathCopy[i].Parent2Name {
			pathCopy[i].Parent1Name, pathCopy[i].Parent2Name = pathCopy[i].Parent2Name, pathCopy[i].Parent1Name
		}
	}

	// Ini penting agar path {A=B+C, C=D+E} dan {C=D+E, A=B+C} dianggap sama jika merupakan set step yang sama.
	sort.Slice(pathCopy, func(i, j int) bool {
		if pathCopy[i].ChildName != pathCopy[j].ChildName {
			return pathCopy[i].ChildName < pathCopy[j].ChildName
		}
		if pathCopy[i].Parent1Name != pathCopy[j].Parent1Name { 
			return pathCopy[i].Parent1Name < pathCopy[j].Parent1Name
		}
		return pathCopy[i].Parent2Name < pathCopy[j].Parent2Name
	})
	// Perbaikan typo di atas:
	sort.Slice(pathCopy, func(i, j int) bool {
		if pathCopy[i].ChildName != pathCopy[j].ChildName {
			return pathCopy[i].ChildName < pathCopy[j].ChildName
		}
		if pathCopy[i].Parent1Name != pathCopy[j].Parent1Name { 
			return pathCopy[i].Parent1Name < pathCopy[j].Parent1Name
		}
		return pathCopy[i].Parent2Name < pathCopy[j].Parent2Name
	})


	var signature string
	for _, step := range pathCopy {
		signature += fmt.Sprintf("%s=(%s+%s);", step.ChildName, step.Parent1Name, step.Parent2Name)
	}
	return signature
}


// State untuk item dalam antrian BFS Multi-Path (Backward)
type BFSMPStateBackward struct {
	// ElementsToDeconstruct adalah daftar elemen yang masih perlu diurai untuk path saat ini.
	// Setiap elemen di sini akan dicari resepnya.
	ElementsToDeconstruct []string
	PathTakenSoFar        []pathfinding.PathStep // Langkah-langkah dari target "ke bawah" (Child -> P1, P2)
	// elementsInCurrentPathTree digunakan untuk deteksi siklus dalam satu jalur pembentukan path.
	// Kunci: ChildName, Nilai: true jika sudah ada dalam proses pembuatan path saat ini.
	elementsInCurrentPathTree map[string]bool
}


// BFSFindXDifferentPathsBackward mencari X path berbeda dari target ke elemen dasar.
func BFSFindXDifferentPathsBackward(graph *loadrecipes.BiGraphAlchemy, targetElementName string, maxPaths int) (*pathfinding.MultipleResult, error) {
	if _, targetExists := graph.AllElements[targetElementName]; !targetExists {
		return nil, fmt.Errorf("elemen target '%s' tidak ditemukan dalam data", targetElementName)
	}
	if maxPaths <= 0 {
		return nil, fmt.Errorf("maxPaths harus integer positif")
	}

	var collectedPaths [][]pathfinding.PathStep
	uniquePathSignatures := make(map[string]bool)
	totalNodesExplored := 0 // Menghitung jumlah state yang diproses dari queue

	if graph.BaseElements[targetElementName] {
		// Path kosong adalah satu-satunya cara membuat elemen dasar
		// Tidak ada langkah resep, hanya elemen itu sendiri.
		// Sesuai spesifikasi, visualisasi tree dengan leaf elemen dasar.
		// Jika target adalah base, tree-nya hanya node target itu sendiri.
		// Untuk konsistensi, kita bisa representasikan ini sebagai path kosong.
		collectedPaths = append(collectedPaths, []pathfinding.PathStep{})
		return &pathfinding.MultipleResult{
			Results: []pathfinding.Result{
				{Path: []pathfinding.PathStep{}, NodesVisited: 1},
			},
		}, nil
	}

	initialState := BFSMPStateBackward{
		ElementsToDeconstruct:     []string{targetElementName},
		PathTakenSoFar:            []pathfinding.PathStep{},
		elementsInCurrentPathTree: make(map[string]bool),
	}

	queue := list.New()
	queue.PushBack(initialState)
	totalNodesExplored++

	maxIterations := 2000000 
	currentIterations := 0

	for queue.Len() > 0 && len(collectedPaths) < maxPaths && currentIterations < maxIterations {
		currentIterations++
		stateInterface := queue.Remove(queue.Front())
		currentState := stateInterface.(BFSMPStateBackward)

		// Cek apakah semua elemen yang perlu didekonstruksi sudah base
		allCurrentDecomposedToBase := true
		if len(currentState.ElementsToDeconstruct) == 0 { 
			allCurrentDecomposedToBase = true
		} else {
			for _, elemName := range currentState.ElementsToDeconstruct {
				if !graph.BaseElements[elemName] {
					allCurrentDecomposedToBase = false
					break
				}
			}
		}

		if allCurrentDecomposedToBase {
			// Path lengkap ditemukan. currentState.PathTakenSoFar adalah langkah-langkahnya (mundur).
			pathCandidate := make([]pathfinding.PathStep, len(currentState.PathTakenSoFar))
			copy(pathCandidate, currentState.PathTakenSoFar)
			reversePathStepsBFS(pathCandidate) 

			sig := createPathSignature(pathCandidate)
			if !uniquePathSignatures[sig] {
				uniquePathSignatures[sig] = true
				collectedPaths = append(collectedPaths, pathCandidate)
			}
			continue 
		}

		// Ambil elemen non-base pertama dari ElementsToDeconstruct untuk diproses
		var elementToProcess string
		var nextElementToProcessIdx = -1
		remainingToDeconstructForNextState := []string{}

		for i, elem := range currentState.ElementsToDeconstruct {
			if !graph.BaseElements[elem] {
				elementToProcess = elem
				nextElementToProcessIdx = i
				break
			}
		}

		if nextElementToProcessIdx == -1 { // Semua sisa adalah base, seharusnya sudah ditangani 'allCurrentDecomposedToBase'
			// Ini bisa terjadi jika ElementsToDeconstruct berisi base-base saja.
			// Kondisi ini sama dengan allCurrentDecomposedToBase, path sudah lengkap.
			pathCandidate := make([]pathfinding.PathStep, len(currentState.PathTakenSoFar))
			copy(pathCandidate, currentState.PathTakenSoFar)
			reversePathStepsBFS(pathCandidate)
			sig := createPathSignature(pathCandidate)
			if !uniquePathSignatures[sig] {
				uniquePathSignatures[sig] = true
				collectedPaths = append(collectedPaths, pathCandidate)
			}
			continue
		}
		
		// Buat sisa elemen yang akan didekonstruksi di state berikutnya
		for i, elem := range currentState.ElementsToDeconstruct {
			if i != nextElementToProcessIdx {
				remainingToDeconstructForNextState = append(remainingToDeconstructForNextState, elem)
			}
		}


		// Deteksi siklus dalam path tree saat ini
		if currentState.elementsInCurrentPathTree[elementToProcess] {
			// log.Printf("[BFS-Multi-WARN] Siklus terdeteksi untuk %s. Abaikan cabang ini.", elementToProcess)
			continue
		}
		
		// Buat salinan map untuk state berikutnya agar tidak mengganggu state lain
		newElementsInPathTreeForNext := make(map[string]bool)
		for k, v := range currentState.elementsInCurrentPathTree {
			newElementsInPathTreeForNext[k] = v
		}
		newElementsInPathTreeForNext[elementToProcess] = true // Tandai elemen ini sedang diproses di cabang ini


		parentPairs, hasRecipes := graph.ChildToParents[elementToProcess]
		if !hasRecipes {
			// Elemen non-base tanpa resep, cabang ini buntu
			continue
		}

		// Untuk SETIAP resep yang mungkin untuk elementToProcess
		for _, pair := range parentPairs {
			if len(collectedPaths) >= maxPaths {
				break // Sudah cukup path
			}

			currentStep := pathfinding.PathStep{
				ChildName:   elementToProcess,
				Parent1Name: pair.Mat1,
				Parent2Name: pair.Mat2,
			}

			newPathTaken := make([]pathfinding.PathStep, len(currentState.PathTakenSoFar))
			copy(newPathTaken, currentState.PathTakenSoFar)
			newPathTaken = append(newPathTaken, currentStep)

			// Gabungkan parent dari resep saat ini dengan sisa elemen yang belum didekonstruksi
			nextElementsToDeconstruct := make([]string, len(remainingToDeconstructForNextState))
			copy(nextElementsToDeconstruct, remainingToDeconstructForNextState)
			
			// Tambahkan P1 dan P2 ke daftar yang perlu didekonstruksi, jika belum ada dan bukan base
			tempDeconstructSet := make(map[string]bool)
			for _, el := range nextElementsToDeconstruct { tempDeconstructSet[el] = true }
			
			if !tempDeconstructSet[pair.Mat1] { // Cek agar tidak duplikat dalam list dekonstruksi
				nextElementsToDeconstruct = append(nextElementsToDeconstruct, pair.Mat1)
			}
			if !tempDeconstructSet[pair.Mat2] { // Cek agar tidak duplikat dalam list dekonstruksi
				nextElementsToDeconstruct = append(nextElementsToDeconstruct, pair.Mat2)
			}


			newState := BFSMPStateBackward{
				ElementsToDeconstruct:     nextElementsToDeconstruct,
				PathTakenSoFar:            newPathTaken,
				elementsInCurrentPathTree: newElementsInPathTreeForNext, // Gunakan map yang sudah di-copy dan diupdate
			}
			queue.PushBack(newState)
			totalNodesExplored++
		}
		if len(collectedPaths) >= maxPaths { break }
	}

	if currentIterations >= maxIterations {
		log.Printf("[BFS-Multi-WARN] Mencapai batas iterasi maksimal (%d) untuk target '%s'. Hasil mungkin tidak lengkap (%d path ditemukan).", maxIterations, targetElementName, len(collectedPaths))
	}

	var finalResults []pathfinding.Result
	for _, path := range collectedPaths {
		// Path sudah dalam urutan base ke target (setelah reversePathStepsBFS saat ditemukan)
		// dan sudah unik.
		finalResults = append(finalResults, pathfinding.Result{
			Path:         path,
			NodesVisited: totalNodesExplored, // Ini adalah total state yang diproses, bukan per path
		})
	}
	
	if len(finalResults) == 0 && !graph.BaseElements[targetElementName] {
         log.Printf("[BFS-Multi-INFO] Tidak ada path yang ditemukan untuk '%s' setelah %d iterasi. Ditemukan %d path mentah.", targetElementName, currentIterations, len(collectedPaths))
		 // return nil, fmt.Errorf("tidak ada path yang ditemukan untuk %s", targetElementName)
	}


	return &pathfinding.MultipleResult{Results: finalResults}, nil
}