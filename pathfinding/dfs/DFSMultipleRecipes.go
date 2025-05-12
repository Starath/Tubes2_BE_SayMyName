package dfs

import (
	"fmt"
	"log"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Starath/Tubes2_BE_SayMyName/loadrecipes"
	"github.com/Starath/Tubes2_BE_SayMyName/pathfinding"
)

// --- Refactored Implementation for Multiple Recipes DFS ---

// workerResult adalah struktur untuk membawa hasil dari setiap goroutine worker.
type workerResult struct {
	path         []pathfinding.PathStep
	nodesVisited int
	err          error // Untuk melaporkan error spesifik dari worker jika ada
}

// workerConfig mengemas parameter konfigurasi untuk worker DFS
type workerConfig struct {
	targetElementName string
	graph             *loadrecipes.BiGraphAlchemy
	workerID          int
	maxRecipesGlobal  int
	randomSeed        int64 // Seed unik untuk random choices dalam worker
	explorationDepth  int   // Seberapa dalam worker boleh mencoba variasi tambahan (0 = normal)
}

// DFSFindMultiplePathsString memulai pencarian untuk banyak jalur resep yang berbeda
// menuju elemen target menggunakan DFS dengan pendekatan multithreading yang ditingkatkan.
func DFSFindMultiplePathsString(graph *loadrecipes.BiGraphAlchemy, targetElementName string, maxRecipes int) (*pathfinding.MultipleResult, int, error) {
	// 1. Validasi input dan kasus dasar
	if _, targetExists := graph.AllElements[targetElementName]; !targetExists {
		return nil, 0, fmt.Errorf("elemen target '%s' tidak ditemukan dalam data", targetElementName)
	}
	if maxRecipes <= 0 {
		return nil, 0, fmt.Errorf("parameter maxRecipes harus positif")
	}
	if graph.BaseElements[targetElementName] { // Jika target adalah elemen dasar
		return &pathfinding.MultipleResult{Results: []pathfinding.Result{{Path: []pathfinding.PathStep{}, NodesVisited: 1}}}, 1, nil
	}

	// 2. Dapatkan resep-resep awal untuk elemen target
	initialRecipesForTarget, hasRecipes := graph.ChildToParents[targetElementName]
	if !hasRecipes {
		return nil, 0, fmt.Errorf("elemen target '%s' tidak memiliki resep", targetElementName)
	}

	// 3. Inisialisasi untuk multithreading dan koleksi hasil
	var wg sync.WaitGroup
	
	// Hitung jumlah worker yang akan dibuat - minimal sama dengan jumlah resep awal,
	// tapi bisa lebih banyak untuk menghasilkan variasi tambahan hingga maxRecipes
	numInitialRecipes := len(initialRecipesForTarget)
	numWorkers := numInitialRecipes
	
	// Jika maxRecipes lebih besar dari jumlah resep awal, kita buat lebih banyak worker
	// dengan konfigurasi random untuk mencari variasi tambahan
	if maxRecipes > numInitialRecipes {
		// Buat tambahan worker sesuai dengan maxRecipes
		numAdditionalWorkers := maxRecipes - numInitialRecipes
		// Batasi jumlah tambahan worker untuk efisiensi (opsional)
		maxAdditionalWorkers := numInitialRecipes * 3 // Misalnya: 3x lipat dari jumlah resep awal
		if numAdditionalWorkers > maxAdditionalWorkers {
			numAdditionalWorkers = maxAdditionalWorkers
		}
		numWorkers += numAdditionalWorkers
	}
	
	// Channel buffered untuk hasil dari worker
	resultsProcessingChan := make(chan workerResult, numWorkers)
	
	// Memoization bersama: menyimpan status apakah suatu elemen *secara umum* dapat dibuat.
	sharedOverallCanBeMadeMemo := make(map[string]bool)
	var sharedMemoMutex sync.Mutex
	
	var pathsFoundCounter int32 // Counter atomik untuk jumlah jalur unik yang sudah ditemukan
	var totalNodesVisitedByWorkers int64 // Counter atomik untuk total node yang dikunjungi

	// doneChan digunakan untuk memberi sinyal kepada semua worker untuk berhenti jika maxRecipes sudah tercapai.
	doneChan := make(chan struct{})

	// 4. Luncurkan goroutine worker untuk setiap resep awal dari elemen target
	// dan worker tambahan dengan variasi untuk mencapai maxRecipes
	workerCount := 0
	
	// Pertama, luncurkan worker untuk setiap resep awal (pendekatan deterministik)
	for _, initialRecipe := range initialRecipesForTarget {
		if atomic.LoadInt32(&pathsFoundCounter) >= int32(maxRecipes) {
			break
		}

		wg.Add(1)
		workerCount++
		
		go dfsWorkerFindOnePathWithInitialRecipe(
			graph,
			targetElementName,
			initialRecipe, // Resep spesifik yang digunakan oleh worker ini
			maxRecipes,
			&pathsFoundCounter,
			&totalNodesVisitedByWorkers,
			resultsProcessingChan,
			&wg,
			sharedOverallCanBeMadeMemo,
			&sharedMemoMutex,
			doneChan,
			0, // Eksplorasi standar
			time.Now().UnixNano() + int64(workerCount), // Seed unik
		)
	}
	
	// Kedua, luncurkan worker tambahan dengan konfigurasi acak untuk mencari variasi (jika dibutuhkan)
	for i := workerCount; i < numWorkers; i++ {
		if atomic.LoadInt32(&pathsFoundCounter) >= int32(maxRecipes) {
			break
		}

		wg.Add(1)
		workerCount++
		
		// Gunakan variasi kedalaman eksplorasi yang berbeda untuk tiap worker
		explorationDepth := 1 + (i % 5) // Variasi 1-5 kedalaman eksplorasi
		
		// Pilih mau gunakan resep awal spesifik atau random
		var initialRecipe loadrecipes.PairMats
		if i < len(initialRecipesForTarget)*2 { 
			// Beberapa worker pertama, gunakan resep awal yang sudah ada tapi dengan konfigurasi berbeda
			recipeIndex := i % len(initialRecipesForTarget)
			initialRecipe = initialRecipesForTarget[recipeIndex]
		} else {
			// Worker selanjutnya, mulai dengan random
			recipeIndex := i % len(initialRecipesForTarget)
			initialRecipe = initialRecipesForTarget[recipeIndex]
		}
		
		// Buat seed unik untuk random choices dalam worker
		randomSeed := time.Now().UnixNano() + int64(i*1000)
		
		go dfsWorkerFindOnePathWithInitialRecipe(
			graph,
			targetElementName,
			initialRecipe,
			maxRecipes,
			&pathsFoundCounter,
			&totalNodesVisitedByWorkers,
			resultsProcessingChan,
			&wg,
			sharedOverallCanBeMadeMemo,
			&sharedMemoMutex,
			doneChan,
			explorationDepth,
			randomSeed,
		)
	}

	// Goroutine untuk menutup resultsProcessingChan setelah semua worker selesai
	go func() {
		wg.Wait()
		close(resultsProcessingChan)
	}()

	// 5. Kumpulkan hasil dari semua worker dan lakukan deduplikasi path
	var collectedUniquePathResults []pathfinding.Result
	pathSignatures := make(map[string]bool) 
	var accumulatedNodesForUniquePaths int

	for workerRes := range resultsProcessingChan {
		if workerRes.err != nil {
			log.Printf("Error dari worker untuk target %s: %v", targetElementName, workerRes.err)
			continue
		}

		// Hanya proses jika path ditemukan (panjang > 0)
		if len(workerRes.path) > 0 {
			pathSignature := generatePathSignature(workerRes.path)
			
			// Cek keunikan path menggunakan signature
			if !pathSignatures[pathSignature] {
				pathSignatures[pathSignature] = true // Tandai signature ini sudah ditemukan
				collectedUniquePathResults = append(collectedUniquePathResults, pathfinding.Result{
					Path:         workerRes.path,
					NodesVisited: workerRes.nodesVisited,
				})
				accumulatedNodesForUniquePaths += workerRes.nodesVisited

				// Jika jumlah path unik sudah mencapai maxRecipes, beri sinyal worker lain untuk berhenti.
				if len(collectedUniquePathResults) >= maxRecipes {
					select {
					case <-doneChan: // Jika sudah ditutup, jangan tutup lagi
					default:
						close(doneChan)
					}
					
					// Kosongkan resultsProcessingChan untuk mencegah deadlock
					go func() {
						for range resultsProcessingChan {
							// Buang sisa hasil
						}
					}()
					break
				}
			}
		}
	}
	
	// Jika tidak ada path unik yang ditemukan dan target bukan elemen dasar
	if len(collectedUniquePathResults) == 0 && !graph.BaseElements[targetElementName] {
		return &pathfinding.MultipleResult{Results: []pathfinding.Result{}}, accumulatedNodesForUniquePaths, fmt.Errorf("tidak ada jalur resep unik yang ditemukan untuk elemen '%s' setelah semua worker selesai", targetElementName)
	}

	return &pathfinding.MultipleResult{Results: collectedUniquePathResults}, accumulatedNodesForUniquePaths, nil
}

// dfsWorkerFindOnePathWithInitialRecipe dijalankan oleh setiap goroutine worker.
// Versi yang ditingkatkan untuk mencari satu jalur dengan kemampuan mengeksplorasi variasi
func dfsWorkerFindOnePathWithInitialRecipe(
	graph *loadrecipes.BiGraphAlchemy,
	targetElementName string,
	initialRecipeForTargetElement loadrecipes.PairMats,
	maxRecipesGlobalLimit int,
	pathsFoundGlobalCounter *int32,
	overallNodesVisitedCounter *int64,
	resultsChan chan<- workerResult,
	wg *sync.WaitGroup,
	sharedOverallCanBeMadeMemo map[string]bool,
	sharedMemoMutex *sync.Mutex,
	doneChan <-chan struct{},
	explorationDepth int, // Parameter baru: seberapa dalam worker boleh mencoba variasi
	randomSeed int64,     // Parameter baru: seed untuk pilihan acak
) {
	defer wg.Done()

	// Inisialisasi RNG lokal untuk worker ini
	localRNG := rand.New(rand.NewSource(randomSeed))
	
	// Inisialisasi struktur data lokal untuk pencarian DFS worker ini
	pathStepsForThisWorker := make(map[string]pathfinding.PathStep)
	currentlySolvingForThisWorker := make(map[string]bool)
	memoForThisWorkerBranch := make(map[string]bool)
	
	var nodesVisitedByThisWorker int

	// Langkah 0: Cek apakah worker harus berhenti lebih awal
	select {
	case <-doneChan:
		return
	default:
		if atomic.LoadInt32(pathsFoundGlobalCounter) >= int32(maxRecipesGlobalLimit) {
			return
		}
	}

	// Langkah 1: Rekursif mencari cara membuat Parent 1 dari `initialRecipeForTargetElement`
	canMakeP1 := dfsRecursiveHelperForWorkerPathEnhanced(
		initialRecipeForTargetElement.Mat1, 
		graph, 
		pathStepsForThisWorker, 
		currentlySolvingForThisWorker, 
		memoForThisWorkerBranch,
		sharedOverallCanBeMadeMemo, 
		sharedMemoMutex, 
		&nodesVisitedByThisWorker, 
		doneChan, 
		pathsFoundGlobalCounter, 
		maxRecipesGlobalLimit,
		explorationDepth,  // Parameter baru
		localRNG,          // Parameter baru
		0,                 // Depth awal = 0
	)

	if !canMakeP1 {
		resultsChan <- workerResult{path: nil, nodesVisited: nodesVisitedByThisWorker, err: nil}
		return
	}

	// Langkah 2: Rekursif mencari cara membuat Parent 2 dari `initialRecipeForTargetElement`
	canMakeP2 := dfsRecursiveHelperForWorkerPathEnhanced(
		initialRecipeForTargetElement.Mat2, 
		graph, 
		pathStepsForThisWorker, 
		currentlySolvingForThisWorker, 
		memoForThisWorkerBranch,
		sharedOverallCanBeMadeMemo, 
		sharedMemoMutex, 
		&nodesVisitedByThisWorker, 
		doneChan, 
		pathsFoundGlobalCounter, 
		maxRecipesGlobalLimit,
		explorationDepth,
		localRNG,
		0,
	)

	if !canMakeP2 {
		resultsChan <- workerResult{path: nil, nodesVisited: nodesVisitedByThisWorker, err: nil}
		return
	}

	// Langkah 3: Jika kedua parent dari resep awal berhasil dibuat, worker ini telah menemukan satu jalur valid.
	pathStepsForThisWorker[targetElementName] = pathfinding.PathStep{
		ChildName:   targetElementName,
		Parent1Name: initialRecipeForTargetElement.Mat1,
		Parent2Name: initialRecipeForTargetElement.Mat2,
	}
	
	reconstructedPath := reconstructFullPathFromSteps(pathStepsForThisWorker, targetElementName, graph.BaseElements)
	
	resultsChan <- workerResult{path: reconstructedPath, nodesVisited: nodesVisitedByThisWorker, err: nil}
	atomic.AddInt32(pathsFoundGlobalCounter, 1)
}

// dfsRecursiveHelperForWorkerPathEnhanced adalah fungsi rekursif yang ditingkatkan
// untuk mendukung lebih banyak variasi jalur
func dfsRecursiveHelperForWorkerPathEnhanced(
	elementName string,
	graph *loadrecipes.BiGraphAlchemy,
	pathStepsThisBranch map[string]pathfinding.PathStep,
	currentlySolvingThisBranch map[string]bool,
	memoForThisWorkerBranch map[string]bool,
	sharedOverallCanBeMadeMemo map[string]bool,
	sharedMemoMutex *sync.Mutex,
	nodesVisitedCounter *int,
	doneChan <-chan struct{},
	pathsFoundGlobalCounter *int32,
	maxRecipesGlobalLimit int,
	explorationDepth int, // Parameter baru: seberapa dalam worker boleh mencoba variasi
	localRNG *rand.Rand,  // Parameter baru: RNG lokal untuk worker ini
	currentDepth int,     // Parameter baru: depth rekursif saat ini
) bool {
	// 0. Cek sinyal berhenti lebih awal
	select {
	case <-doneChan:
		return false
	default:
		if atomic.LoadInt32(pathsFoundGlobalCounter) >= int32(maxRecipesGlobalLimit) {
			return false
		}
	}

	// 1. Cek memo spesifik cabang worker ini
	if canBeMade, exists := memoForThisWorkerBranch[elementName]; exists {
		return canBeMade
	}

	// 2. Kasus Dasar: Elemen adalah elemen dasar (Air, Api, dll.)
	if graph.BaseElements[elementName] {
		(*nodesVisitedCounter)++
		memoForThisWorkerBranch[elementName] = true
		return true
	}
	
	// 3. Deteksi Siklus untuk cabang rekursif ini
	if currentlySolvingThisBranch[elementName] {
		return false
	}
	currentlySolvingThisBranch[elementName] = true
	defer delete(currentlySolvingThisBranch, elementName)

	// 4. Cek memo bersama (sharedOverallCanBeMadeMemo)
	sharedMemoMutex.Lock()
	if knownGlobalStatus, exists := sharedOverallCanBeMadeMemo[elementName]; exists && !knownGlobalStatus {
		sharedMemoMutex.Unlock()
		memoForThisWorkerBranch[elementName] = false
		return false
	}
	sharedMemoMutex.Unlock()
	
	(*nodesVisitedCounter)++

	// 5. Dapatkan semua resep yang menghasilkan `elementName`
	recipesForCurrentElement, hasRecipes := graph.ChildToParents[elementName]
	if !hasRecipes {
		sharedMemoMutex.Lock()
		sharedOverallCanBeMadeMemo[elementName] = false
		sharedMemoMutex.Unlock()
		memoForThisWorkerBranch[elementName] = false
		return false
	}

	// 6. Menentukan urutan untuk mencoba resep-resep
	// Untuk membuat variasi, kita acak urutan resep berdasarkan explorationDepth
	shuffledRecipeIndices := make([]int, len(recipesForCurrentElement))
	for i := range shuffledRecipeIndices {
		shuffledRecipeIndices[i] = i
	}
	
	// Semakin dalam level eksplorasi, semakin tinggi probabilitas mengacak urutan resep
	shuffleProbability := float64(explorationDepth) * 0.2 // Misalnya: depth 1 = 20% chance, depth 5 = 100% chance
	if currentDepth <= explorationDepth && localRNG.Float64() < shuffleProbability {
		// Fisher-Yates shuffle
		for i := len(shuffledRecipeIndices) - 1; i > 0; i-- {
			j := localRNG.Intn(i + 1)
			shuffledRecipeIndices[i], shuffledRecipeIndices[j] = shuffledRecipeIndices[j], shuffledRecipeIndices[i]
		}
	}

	// 7. Coba resep berdasarkan urutan yang sudah ditentukan
	for _, index := range shuffledRecipeIndices {
		recipePair := recipesForCurrentElement[index]
		
		// Untuk variasi lebih lanjut, kadang-kadang tukar parent1 dan parent2
		parent1, parent2 := recipePair.Mat1, recipePair.Mat2
		if explorationDepth > 0 && currentDepth <= explorationDepth && localRNG.Float64() < 0.3 {
			parent1, parent2 = parent2, parent1
		}
		
		// Rekursif panggil untuk Parent 1
		canMakeP1 := dfsRecursiveHelperForWorkerPathEnhanced(
			parent1, graph, pathStepsThisBranch, currentlySolvingThisBranch, memoForThisWorkerBranch,
			sharedOverallCanBeMadeMemo, sharedMemoMutex, nodesVisitedCounter, doneChan, 
			pathsFoundGlobalCounter, maxRecipesGlobalLimit, explorationDepth, localRNG, currentDepth+1)
		if !canMakeP1 {
			continue
		}

		// Rekursif panggil untuk Parent 2
		canMakeP2 := dfsRecursiveHelperForWorkerPathEnhanced(
			parent2, graph, pathStepsThisBranch, currentlySolvingThisBranch, memoForThisWorkerBranch,
			sharedOverallCanBeMadeMemo, sharedMemoMutex, nodesVisitedCounter, doneChan, 
			pathsFoundGlobalCounter, maxRecipesGlobalLimit, explorationDepth, localRNG, currentDepth+1)
		if !canMakeP2 {
			continue
		}

		// Jika kedua parent berhasil dibuat untuk resep ini dalam cabang ini
		pathStepsThisBranch[elementName] = pathfinding.PathStep{ChildName: elementName, Parent1Name: parent1, Parent2Name: parent2}
		memoForThisWorkerBranch[elementName] = true
		return true
	}

	memoForThisWorkerBranch[elementName] = false
	return false
}

// generatePathSignature membuat representasi string yang unik dari sebuah jalur (path).
func generatePathSignature(path []pathfinding.PathStep) string {
	if len(path) == 0 {
		return "base_element_or_empty_path"
	}

	// Buat salinan path agar tidak mengubah path aslinya saat sorting
	pathCopy := make([]pathfinding.PathStep, len(path))
	copy(pathCopy, path)

	// Urutkan slice pathCopy untuk memastikan signature yang konsisten
	sort.Slice(pathCopy, func(i, j int) bool {
		// Urutkan berdasarkan ChildName
		if pathCopy[i].ChildName != pathCopy[j].ChildName {
			return pathCopy[i].ChildName < pathCopy[j].ChildName
		}
		// Jika ChildName sama, urutkan berdasarkan Parent1Name (setelah normalisasi)
		p1_i, p2_i := pathCopy[i].Parent1Name, pathCopy[i].Parent2Name
		if p1_i > p2_i {p1_i, p2_i = p2_i, p1_i} // Normalisasi urutan parent i

		p1_j, p2_j := pathCopy[j].Parent1Name, pathCopy[j].Parent2Name
		if p1_j > p2_j {p1_j, p2_j = p2_j, p1_j} // Normalisasi urutan parent j
		
		if p1_i != p1_j {
			return p1_i < p1_j
		}
		// Jika Parent1Name juga sama, urutkan berdasarkan Parent2Name (setelah normalisasi)
		return p2_i < p2_j
	})

	// Gabungkan step-step yang sudah diurut menjadi satu string signature
	var signatureParts []string
	for _, step := range pathCopy {
		// Normalisasi urutan parent dalam string signature juga
		parent1, parent2 := step.Parent1Name, step.Parent2Name
		if parent1 > parent2 {parent1, parent2 = parent2, parent1} 
		signatureParts = append(signatureParts, fmt.Sprintf("%s=(%s+%s)", step.ChildName, parent1, parent2))
	}
	return strings.Join(signatureParts, ";")
}