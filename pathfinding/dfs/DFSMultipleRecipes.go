package dfs

import (
	"fmt"
	"log"
	"sort" // Digunakan untuk membuat signature path yang konsisten
	"strings"
	"sync"         // Digunakan untuk sinkronisasi goroutine
	"sync/atomic"  // Digunakan untuk operasi atomik pada counter

	"github.com/Starath/Tubes2_BE_SayMyName/loadrecipes" // Sesuaikan dengan path package Anda
	"github.com/Starath/Tubes2_BE_SayMyName/pathfinding" // Sesuaikan dengan path package Anda
)

// --- Implementasi Baru untuk Multiple Recipes DFS ---

// workerResult adalah struktur untuk membawa hasil dari setiap goroutine worker.
type workerResult struct {
	path         []pathfinding.PathStep
	nodesVisited int
	err          error // Untuk melaporkan error spesifik dari worker jika ada
}

// DFSFindMultiplePathsString memulai pencarian untuk banyak jalur resep yang berbeda
// menuju elemen target menggunakan DFS dengan pendekatan multithreading.
// Setiap resep awal untuk elemen target akan dieksplorasi dalam goroutine terpisah.
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
	// Channel buffered untuk hasil dari worker, ukurannya sejumlah resep awal
	resultsProcessingChan := make(chan workerResult, len(initialRecipesForTarget))
	
	// Memoization bersama: menyimpan status apakah suatu elemen *secara umum* dapat dibuat.
	// Ini membantu memangkas cabang yang sudah diketahui tidak mungkin di semua worker.
	// Akses ke map ini harus disinkronkan menggunakan mutex.
	sharedOverallCanBeMadeMemo := make(map[string]bool)
	var sharedMemoMutex sync.Mutex
	
	var pathsFoundCounter int32 // Counter atomik untuk jumlah jalur unik yang sudah ditemukan.
	var totalNodesVisitedByWorkers int64 // Counter atomik untuk total node yang dikunjungi oleh worker yang berhasil.

	// doneChan digunakan untuk memberi sinyal kepada semua worker untuk berhenti jika maxRecipes sudah tercapai.
	doneChan := make(chan struct{})

	// 4. Luncurkan goroutine worker untuk setiap resep awal dari elemen target
	for _, initialRecipe := range initialRecipesForTarget {
		// Pemeriksaan awal: jika sudah cukup banyak path yang dijadwalkan/ditemukan,
		// kita bisa berhenti meluncurkan worker baru.
		if atomic.LoadInt32(&pathsFoundCounter) >= int32(maxRecipes) {
			break
		}

		wg.Add(1)
		go dfsWorkerFindOnePathAttempt(
			graph,
			targetElementName,
			initialRecipe, // Resep spesifik yang *harus* digunakan worker ini untuk langkah pertama (membuat target)
			maxRecipes,    // Batas global maxRecipes
			&pathsFoundCounter,
			&totalNodesVisitedByWorkers,
			resultsProcessingChan,
			&wg,
			sharedOverallCanBeMadeMemo,
			&sharedMemoMutex,
			doneChan, // Teruskan doneChan ke worker
		)
	}

	// Goroutine untuk menutup resultsProcessingChan setelah semua worker selesai.
	// Ini penting agar loop pengumpul hasil di bawah bisa berhenti.
	go func() {
		wg.Wait()
		close(resultsProcessingChan)
	}()

	// 5. Kumpulkan hasil dari semua worker dan lakukan deduplikasi path
	var collectedUniquePathResults []pathfinding.Result
	// map untuk menyimpan signature dari path yang sudah ditemukan, untuk memastikan keunikan.
	pathSignatures := make(map[string]bool) 
	var accumulatedNodesForUniquePaths int // Akumulasi node dari path unik yang ditambahkan

	for workerRes := range resultsProcessingChan {
		if workerRes.err != nil {
			// Log error dari worker, tapi jangan hentikan pengumpulan path lain kecuali errornya kritis.
			log.Printf("Error dari worker untuk target %s: %v", targetElementName, workerRes.err)
			continue // Lanjutkan ke hasil worker berikutnya
		}

		// Hanya proses jika path ditemukan (panjang > 0)
		if len(workerRes.path) > 0 {
			pathSignature := generatePathSignature(workerRes.path) // Buat signature untuk path ini
			
			// Cek keunikan path menggunakan signature
			if !pathSignatures[pathSignature] {
				pathSignatures[pathSignature] = true // Tandai signature ini sudah ditemukan
				collectedUniquePathResults = append(collectedUniquePathResults, pathfinding.Result{
					Path:         workerRes.path,
					NodesVisited: workerRes.nodesVisited, // Nodes yang dikunjungi worker ini untuk path ini
				})
				accumulatedNodesForUniquePaths += workerRes.nodesVisited

				// Jika jumlah path unik sudah mencapai maxRecipes, beri sinyal worker lain untuk berhenti.
				if len(collectedUniquePathResults) >= maxRecipes {
					// Tutup doneChan hanya sekali. Gunakan sync.Once jika ada potensi race condition (meski di sini kecil kemungkinannya).
					// Atau, cukup tutup dan biarkan goroutine yang mencoba mengirim ke doneChan yang sudah tertutup akan no-op.
					// Namun, lebih aman jika doneChan ditutup dari sini.
					// Pastikan penutupan doneChan aman dan tidak menyebabkan panic jika sudah ditutup.
					// Cara sederhana: cek dulu.
					select {
					case <-doneChan: // Jika sudah ditutup, jangan tutup lagi
					default:
						close(doneChan)
					}
					
					// Setelah doneChan ditutup, kita perlu mengosongkan resultsProcessingChan
					// agar goroutine worker yang mungkin masih mengirim tidak terblokir selamanya.
					// Ini mencegah deadlock.
					go func() {
						for range resultsProcessingChan {
							// Buang sisa hasil
						}
					}()
					break // Keluar dari loop pengumpul hasil
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

// dfsWorkerFindOnePathAttempt dijalankan oleh setiap goroutine.
// Fungsi ini mencoba mencari satu jalur lengkap ke `targetElementName`
// dengan syarat bahwa langkah *pertama* (resep untuk `targetElementName`) adalah `initialRecipeForTargetElement`.
func dfsWorkerFindOnePathAttempt(
	graph *loadrecipes.BiGraphAlchemy,
	targetElementName string,
	initialRecipeForTargetElement loadrecipes.PairMats, // Resep spesifik yang *harus* digunakan worker ini untuk target
	maxRecipesGlobalLimit int,
	pathsFoundGlobalCounter *int32,      // Pointer ke counter atomik global untuk path unik yang ditemukan
	overallNodesVisitedCounter *int64, // Pointer ke counter atomik global untuk total node yang dikunjungi
	resultsChan chan<- workerResult,   // Channel untuk mengirim hasil (path, nodes, error)
	wg *sync.WaitGroup,                // WaitGroup untuk sinkronisasi
	sharedOverallCanBeMadeMemo map[string]bool, // Memo bersama: apakah elemen X *bisa* dibuat (secara umum)?
	sharedMemoMutex *sync.Mutex,       // Mutex untuk melindungi sharedOverallCanBeMadeMemo
	doneChan <-chan struct{},          // Channel untuk mendengarkan sinyal berhenti
) {
	defer wg.Done() // Pastikan Done dipanggil saat goroutine selesai

	// Inisialisasi struktur data lokal untuk pencarian DFS worker ini
	pathStepsForThisWorker := make(map[string]pathfinding.PathStep) // Langkah-langkah resep untuk jalur yang sedang dibangun worker ini
	currentlySolvingForThisWorker := make(map[string]bool)         // Deteksi siklus untuk jalur yang sedang dibangun worker ini
	memoForThisWorkerBranch := make(map[string]bool)               // Memo untuk cabang rekursif worker ini (apakah elemen X bisa dibuat di *cabang ini*?)
	
	var nodesVisitedByThisWorker int // Counter node yang dikunjungi oleh worker ini

	// Langkah 0: Cek apakah worker harus berhenti lebih awal
	select {
	case <-doneChan: // Jika doneChan sudah ditutup (misalnya maxRecipes tercapai oleh worker lain)
		// Tidak mengirim hasil karena worker dihentikan lebih awal
		return
	default:
		// Lanjutkan jika belum ada sinyal berhenti.
		// Cek juga apakah jumlah path yang ditemukan secara global sudah mencapai batas.
		if atomic.LoadInt32(pathsFoundGlobalCounter) >= int32(maxRecipesGlobalLimit) {
			return // Batas global sudah tercapai
		}
	}

	// Langkah 1: Rekursif mencari cara membuat Parent 1 dari `initialRecipeForTargetElement`
	canMakeP1 := dfsRecursiveHelperForWorkerPath(
		initialRecipeForTargetElement.Mat1, graph, pathStepsForThisWorker, currentlySolvingForThisWorker, memoForThisWorkerBranch,
		sharedOverallCanBeMadeMemo, sharedMemoMutex, &nodesVisitedByThisWorker, doneChan, pathsFoundGlobalCounter, maxRecipesGlobalLimit)

	if !canMakeP1 {
		resultsChan <- workerResult{path: nil, nodesVisited: nodesVisitedByThisWorker, err: nil} // Jalur tidak ditemukan melalui resep awal ini
		return
	}

	// Langkah 2: Rekursif mencari cara membuat Parent 2 dari `initialRecipeForTargetElement`
	canMakeP2 := dfsRecursiveHelperForWorkerPath(
		initialRecipeForTargetElement.Mat2, graph, pathStepsForThisWorker, currentlySolvingForThisWorker, memoForThisWorkerBranch,
		sharedOverallCanBeMadeMemo, sharedMemoMutex, &nodesVisitedByThisWorker, doneChan, pathsFoundGlobalCounter, maxRecipesGlobalLimit)

	if !canMakeP2 {
		resultsChan <- workerResult{path: nil, nodesVisited: nodesVisitedByThisWorker, err: nil} // Jalur tidak ditemukan melalui resep awal ini
		return
	}

	// Langkah 3: Jika kedua parent dari resep awal berhasil dibuat, worker ini telah menemukan satu jalur valid.
	// Catat langkah terakhir (pembuatan targetElementName itu sendiri).
	pathStepsForThisWorker[targetElementName] = pathfinding.PathStep{
		ChildName:   targetElementName,
		Parent1Name: initialRecipeForTargetElement.Mat1,
		Parent2Name: initialRecipeForTargetElement.Mat2,
	}
	
	// Rekonstruksi jalur lengkap dari langkah-langkah yang tercatat.
	reconstructedPath := reconstructFullPathFromSteps(pathStepsForThisWorker, targetElementName, graph.BaseElements)
	
	// Kirim hasil (jalur dan jumlah node yang dikunjungi worker ini) ke channel utama.
	// Penambahan ke overallNodesVisitedCounter akan dilakukan di kolektor jika path ini unik.
	resultsChan <- workerResult{path: reconstructedPath, nodesVisited: nodesVisitedByThisWorker, err: nil}
	
	// Increment counter global untuk path yang ditemukan (meski belum tentu unik, ini membantu doneChan)
	// Keunikan akan ditangani di kolektor.
	atomic.AddInt32(pathsFoundGlobalCounter, 1) 
}


// dfsRecursiveHelperForWorkerPath adalah fungsi rekursif inti yang digunakan oleh setiap worker.
// Fungsi ini mencoba mencari apakah `elementName` dapat dibuat dan mencatat langkah-langkahnya
// dalam `pathStepsThisBranch` (spesifik untuk cabang rekursif worker ini).
func dfsRecursiveHelperForWorkerPath(
	elementName string,
	graph *loadrecipes.BiGraphAlchemy,
	pathStepsThisBranch map[string]pathfinding.PathStep, // Langkah resep untuk cabang worker ini
	currentlySolvingThisBranch map[string]bool,         // Deteksi siklus untuk cabang worker ini
	memoForThisWorkerBranch map[string]bool,           // Memo untuk cabang worker ini
	sharedOverallCanBeMadeMemo map[string]bool,         // Memo bersama: apakah elemen X bisa dibuat (umum)?
	sharedMemoMutex *sync.Mutex,                       // Mutex untuk `sharedOverallCanBeMadeMemo`
	nodesVisitedCounter *int,                          // Pointer ke counter node yang dikunjungi worker ini
	doneChan <-chan struct{},                          // Channel untuk sinyal berhenti
	pathsFoundGlobalCounter *int32,                    // Counter global path yang ditemukan
	maxRecipesGlobalLimit int,                         // Batas global maxRecipes
) bool {
	// 0. Cek sinyal berhenti lebih awal
	select {
	case <-doneChan:
		return false // Berhenti jika doneChan ditutup
	default:
		// Lanjutkan jika belum ada sinyal berhenti.
		// Cek juga apakah jumlah path yang ditemukan secara global sudah mencapai batas.
		// Ini adalah optimasi agar worker tidak melanjutkan pekerjaan yang tidak perlu.
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
		(*nodesVisitedCounter)++ // Hitung kunjungan node untuk worker ini
		memoForThisWorkerBranch[elementName] = true
		return true
	}
	
	// 3. Deteksi Siklus untuk cabang rekursif ini
	if currentlySolvingThisBranch[elementName] {
		return false // Siklus terdeteksi di cabang ini
	}
	currentlySolvingThisBranch[elementName] = true
	defer delete(currentlySolvingThisBranch, elementName) // Hapus tanda saat backtrack

	// 4. Cek memo bersama (sharedOverallCanBeMadeMemo)
	// Ini membantu memangkas pencarian jika elemen ini sudah diketahui tidak bisa dibuat secara global.
	sharedMemoMutex.Lock()
	if knownGlobalStatus, exists := sharedOverallCanBeMadeMemo[elementName]; exists && !knownGlobalStatus {
		sharedMemoMutex.Unlock()
		memoForThisWorkerBranch[elementName] = false // Tandai juga di memo cabang ini
		return false
	}
	sharedMemoMutex.Unlock()
	
	(*nodesVisitedCounter)++ // Hitung kunjungan node untuk worker ini

	// 5. Dapatkan semua resep yang menghasilkan `elementName`
	recipesForCurrentElement, hasRecipes := graph.ChildToParents[elementName]
	if !hasRecipes { // Jika tidak ada resep dan bukan elemen dasar
		sharedMemoMutex.Lock()
		sharedOverallCanBeMadeMemo[elementName] = false // Tandai elemen ini tidak bisa dibuat secara global
		sharedMemoMutex.Unlock()
		memoForThisWorkerBranch[elementName] = false
		return false
	}

	// 6. Coba setiap resep secara rekursif untuk cabang ini.
	// Untuk menemukan *satu* cara membuat `elementName` dalam konteks cabang worker ini,
	// kita akan mengambil resep pertama yang berhasil.
	for _, recipePair := range recipesForCurrentElement {
		// Rekursif panggil untuk Parent 1
		canMakeP1 := dfsRecursiveHelperForWorkerPath(
			recipePair.Mat1, graph, pathStepsThisBranch, currentlySolvingThisBranch, memoForThisWorkerBranch,
			sharedOverallCanBeMadeMemo, sharedMemoMutex, nodesVisitedCounter, doneChan, pathsFoundGlobalCounter, maxRecipesGlobalLimit)
		if !canMakeP1 {
			continue // Jika Parent 1 tidak bisa dibuat, coba resep lain
		}

		// Rekursif panggil untuk Parent 2
		canMakeP2 := dfsRecursiveHelperForWorkerPath(
			recipePair.Mat2, graph, pathStepsThisBranch, currentlySolvingThisBranch, memoForThisWorkerBranch,
			sharedOverallCanBeMadeMemo, sharedMemoMutex, nodesVisitedCounter, doneChan, pathsFoundGlobalCounter, maxRecipesGlobalLimit)
		if !canMakeP2 {
			continue // Jika Parent 2 tidak bisa dibuat, coba resep lain
		}

		// Jika kedua parent berhasil dibuat untuk resep ini dalam cabang ini
		// Catat langkah resep ini ke `pathStepsThisBranch`
		pathStepsThisBranch[elementName] = pathfinding.PathStep{ChildName: elementName, Parent1Name: recipePair.Mat1, Parent2Name: recipePair.Mat2}
		memoForThisWorkerBranch[elementName] = true // Tandai elemen ini bisa dibuat di cabang ini
		return true // Berhasil menemukan satu cara untuk membuat `elementName` di cabang ini
	}

	// 7. Jika tidak ada resep yang berhasil untuk elemen ini di cabang ini
	// Jangan tandai sharedOverallCanBeMadeMemo sebagai false di sini, karena cabang lain mungkin berhasil.
	// Penandaan global false dilakukan jika elemen benar-benar tidak punya resep atau semua cabang gagal secara definitif.
	memoForThisWorkerBranch[elementName] = false
	return false
}

// generatePathSignature membuat representasi string yang unik dari sebuah jalur (path).
// Ini digunakan untuk deduplikasi jalur yang ditemukan oleh worker yang berbeda.
// Path diurutkan berdasarkan ChildName, kemudian Parent1Name (setelah normalisasi urutan parent), lalu Parent2Name.
func generatePathSignature(path []pathfinding.PathStep) string {
	if len(path) == 0 {
		return "base_element_or_empty_path" // Signature khusus untuk path kosong
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
