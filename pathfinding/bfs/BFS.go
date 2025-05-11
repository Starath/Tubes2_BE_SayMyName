package bfs

import (
	"container/list"
	"fmt"
	"log"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/Starath/Tubes2_BE_SayMyName/loadrecipes"
	"github.com/Starath/Tubes2_BE_SayMyName/pathfinding"
)

const (
	DefaultWorkerMaxIterations = 2500000
)

// reversePathStepsBFS membalik urutan slice PathStep
func reversePathStepsBFS(steps []pathfinding.PathStep) {
	for i, j := 0, len(steps)-1; i < j; i, j = i+1, j-1 {
		steps[i], steps[j] = steps[j], steps[i]
	}
}

// createPathSignature membuat string unik untuk sebuah path agar bisa dideteksi duplikasinya.
// Path yang diberikan HARUS sudah dalam urutan base-ke-target.
func createPathSignature(steps []pathfinding.PathStep) string {
	pathCopy := make([]pathfinding.PathStep, len(steps))
	copy(pathCopy, steps)

	// Normalisasi urutan parent dalam setiap langkah
	for i := range pathCopy {
		if pathCopy[i].Parent1Name > pathCopy[i].Parent2Name {
			pathCopy[i].Parent1Name, pathCopy[i].Parent2Name = pathCopy[i].Parent2Name, pathCopy[i].Parent1Name
		}
	}

	// Urutkan langkah-langkah dalam path untuk memastikan path dengan set step yang sama
	// namun urutan berbeda dianggap identik.
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
	ElementsToDeconstruct     []string
	PathTakenSoFar            []pathfinding.PathStep
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
	totalNodesExplored := 0

	if graph.BaseElements[targetElementName] {
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

	maxIterations := 5000000
	currentIterations := 0

	for queue.Len() > 0 && len(collectedPaths) < maxPaths && currentIterations < maxIterations {
		stateInterface := queue.Remove(queue.Front())
		currentState := stateInterface.(BFSMPStateBackward)
		currentIterations++
		totalNodesExplored++

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
			pathCandidate := make([]pathfinding.PathStep, len(currentState.PathTakenSoFar))
			copy(pathCandidate, currentState.PathTakenSoFar)
			reversePathStepsBFS(pathCandidate)

			sig := createPathSignature(pathCandidate)
			if !uniquePathSignatures[sig] {
				uniquePathSignatures[sig] = true
				collectedPaths = append(collectedPaths, pathCandidate)
				if len(collectedPaths) >= maxPaths {
					break
				}
			}
			continue
		}

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

		if nextElementToProcessIdx == -1 {
			pathCandidate := make([]pathfinding.PathStep, len(currentState.PathTakenSoFar))
			copy(pathCandidate, currentState.PathTakenSoFar)
			reversePathStepsBFS(pathCandidate)
			sig := createPathSignature(pathCandidate)
			if !uniquePathSignatures[sig] {
				uniquePathSignatures[sig] = true
				collectedPaths = append(collectedPaths, pathCandidate)
				if len(collectedPaths) >= maxPaths {
					break
				}
			}
			continue
		}

		for i, elem := range currentState.ElementsToDeconstruct {
			if i != nextElementToProcessIdx {
				remainingToDeconstructForNextState = append(remainingToDeconstructForNextState, elem)
			}
		}
        
		// Salinan map untuk state anak. Map ini berguna untuk mendeteksi siklus A->B->...->A yang sebenarnya.
		newElementsInPathTreeForChildren := make(map[string]bool)
		for k, v := range currentState.elementsInCurrentPathTree {
			newElementsInPathTreeForChildren[k] = v
		}
        
		newElementsInPathTreeForChildren[elementToProcess] = true // Tambahkan elementToProcess ke tree untuk anak-anaknya


		parentPairs, hasRecipes := graph.ChildToParents[elementToProcess]
		if !hasRecipes {
			continue
		}

		for _, pair := range parentPairs {
			if len(collectedPaths) >= maxPaths {
				break
			}

			currentStep := pathfinding.PathStep{
				ChildName:   elementToProcess,
				Parent1Name: pair.Mat1,
				Parent2Name: pair.Mat2,
			}

			newPathTaken := make([]pathfinding.PathStep, len(currentState.PathTakenSoFar))
			copy(newPathTaken, currentState.PathTakenSoFar)
			newPathTaken = append(newPathTaken, currentStep)

			nextElementsToDeconstruct := make([]string, len(remainingToDeconstructForNextState))
			copy(nextElementsToDeconstruct, remainingToDeconstructForNextState)
			
			tempDeconstructSet := make(map[string]bool)
			for _, el := range nextElementsToDeconstruct { tempDeconstructSet[el] = true }
			
			if !tempDeconstructSet[pair.Mat1] {
				nextElementsToDeconstruct = append(nextElementsToDeconstruct, pair.Mat1)
			}
			if !tempDeconstructSet[pair.Mat2] {
				nextElementsToDeconstruct = append(nextElementsToDeconstruct, pair.Mat2)
			}

			newState := BFSMPStateBackward{
				ElementsToDeconstruct:     nextElementsToDeconstruct,
				PathTakenSoFar:            newPathTaken,
				elementsInCurrentPathTree: newElementsInPathTreeForChildren, 
			}
			queue.PushBack(newState)
		}
		if len(collectedPaths) >= maxPaths { break }
	}

	if currentIterations >= maxIterations {
		log.Printf("[BFS-Multi-WARN] Mencapai batas iterasi maksimal (%d) untuk target '%s'. Hasil mungkin tidak lengkap (%d path ditemukan). Total state diproses: %d", maxIterations, targetElementName, len(collectedPaths), totalNodesExplored)
	}

	var finalResults []pathfinding.Result
	for _, path := range collectedPaths {
		finalResults = append(finalResults, pathfinding.Result{
			Path:         path,
			NodesVisited: totalNodesExplored, 
		})
	}
	
	if len(finalResults) == 0 && !graph.BaseElements[targetElementName] {
         log.Printf("[BFS-Multi-INFO] Tidak ada path yang ditemukan untuk '%s' setelah %d iterasi (total state diproses: %d). Ditemukan %d path mentah.", targetElementName, currentIterations, totalNodesExplored, len(collectedPaths))
	}

	return &pathfinding.MultipleResult{Results: finalResults}, nil
}

// bfsBranchWorker adalah fungsi worker untuk satu cabang resep awal dalam pencarian BFS paralel.
// - initialState: State BFS yang sudah satu langkah maju (target sudah didekomposisi sekali).
// - rawPathChannel: Channel untuk mengirim path lengkap yang ditemukan (dalam urutan base-ke-target).
// - wg: WaitGroup untuk memberi tahu orkestrator ketika worker ini selesai.
// - doneSignal: Channel untuk menerima sinyal berhenti jika maxPaths sudah tercapai secara global.
// - nodesExploredCounter: Pointer ke counter atomik untuk total node yang dieksplorasi semua worker.
// - workerMaxIterations: Batas iterasi maksimal untuk worker ini.
func bfsBranchWorker(
	graph *loadrecipes.BiGraphAlchemy,
	initialState BFSMPStateBackward,
	rawPathChannel chan<- []pathfinding.PathStep,
	wg *sync.WaitGroup,
	doneSignal <-chan struct{},
	nodesExploredCounter *int64,
) {
	defer wg.Done() // Pastikan wg.Done() dipanggil saat worker selesai

	workerQueue := list.New()
	workerQueue.PushBack(initialState)
	
	currentWorkerIterations := 0
	workerNodesExploredThisInstance := 0 // Node yang dieksplorasi oleh instance worker ini saja

	// Logging untuk debugging (opsional, bisa di- uncomment jika perlu)
	// initialTargetForLogging := "UNKNOWN_TARGET_IN_WORKER"
	// if len(initialState.PathTakenSoFar) > 0 && initialState.PathTakenSoFar[0].ChildName != "" {
	// 	 initialTargetForLogging = initialState.PathTakenSoFar[0].ChildName
	// }
	// log.Printf("[BFS-BRANCH-WORKER] Dimulai untuk cabang dari target: %s, langkah awal: %s=(%s+%s)", 
	// 	initialTargetForLogging, 
	// 	initialState.PathTakenSoFar[0].ChildName, 
	// 	initialState.PathTakenSoFar[0].Parent1Name, 
	// 	initialState.PathTakenSoFar[0].Parent2Name)

	for workerQueue.Len() > 0 {
		// Cek sinyal berhenti dari orkestrator
		select {
		case <-doneSignal:
			atomic.AddInt64(nodesExploredCounter, int64(workerNodesExploredThisInstance))
			// log.Printf("[BFS-BRANCH-WORKER] Berhenti lebih awal karena doneSignal untuk cabang dari target %s.", initialTargetForLogging)
			return
		default:
			// Lanjutkan eksekusi
		}

		stateInterface := workerQueue.Remove(workerQueue.Front())
		currentState := stateInterface.(BFSMPStateBackward)
		currentWorkerIterations++
		workerNodesExploredThisInstance++

		// Cek apakah semua elemen yang perlu didekonstruksi sudah menjadi elemen dasar
		allCurrentDecomposedToBase := true
		if len(currentState.ElementsToDeconstruct) == 0 { // Jika tidak ada yg perlu didekonstruksi, berarti sudah base
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
			// Jalur lengkap ditemukan untuk cabang worker ini
			pathCandidate := make([]pathfinding.PathStep, len(currentState.PathTakenSoFar))
			copy(pathCandidate, currentState.PathTakenSoFar)
			reversePathStepsBFS(pathCandidate) // Balik urutan menjadi base-ke-target (menggunakan fungsi yang sudah ada)

			// Kirim path ke channel (non-blocking send untuk menghindari deadlock jika doneSignal diterima saat mengirim)
			select {
			case rawPathChannel <- pathCandidate:
				// Path berhasil dikirim
			case <-doneSignal: // Jika doneSignal ditutup saat worker mencoba mengirim
				atomic.AddInt64(nodesExploredCounter, int64(workerNodesExploredThisInstance))
				// log.Printf("[BFS-BRANCH-WORKER] Berhenti (doneSignal) sebelum mengirim path untuk cabang dari target %s.", initialTargetForLogging)
				return
			}
			continue // Lanjutkan mencari path lain dari antrian worker ini
		}

		// Pilih elemen berikutnya untuk didekonstruksi
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
		
		// Jika tidak ada elemen non-base untuk diproses (seharusnya sudah ditangani oleh allCurrentDecomposedToBase)
		if nextElementToProcessIdx == -1 { 
			pathCandidate := make([]pathfinding.PathStep, len(currentState.PathTakenSoFar))
			copy(pathCandidate, currentState.PathTakenSoFar)
			reversePathStepsBFS(pathCandidate)
			select {
			case rawPathChannel <- pathCandidate:
			case <-doneSignal:
				atomic.AddInt64(nodesExploredCounter, int64(workerNodesExploredThisInstance))
				// log.Printf("[BFS-BRANCH-WORKER] Berhenti (doneSignal) sebelum mengirim path (safeguard) untuk cabang dari target %s.", initialTargetForLogging)
				return
			}
			continue
		}

		// Buat daftar elemen sisa untuk state berikutnya
		for i, elem := range currentState.ElementsToDeconstruct {
			if i != nextElementToProcessIdx {
				remainingToDeconstructForNextState = append(remainingToDeconstructForNextState, elem)
			}
		}
        
		// Salinan map untuk state anak (deteksi siklus)
		newElementsInPathTreeForChildren := make(map[string]bool)
		for k, v := range currentState.elementsInCurrentPathTree {
			newElementsInPathTreeForChildren[k] = v
		}
		newElementsInPathTreeForChildren[elementToProcess] = true // Tandai elemen yang sedang diproses

		parentPairs, hasRecipes := graph.ChildToParents[elementToProcess]
		if !hasRecipes {
			continue // Tidak ada cara membuat elementToProcess, cabang ini buntu
		}

		for _, pair := range parentPairs {
			// Deteksi siklus: jika salah satu parent adalah elemen yang sedang diproses ATAU sudah ada di path tree.
			// elementsInCurrentPathTree sudah berisi target awal dan elemen-elemen di atasnya dalam path ini.
			if currentState.elementsInCurrentPathTree[pair.Mat1] || currentState.elementsInCurrentPathTree[pair.Mat2] {
				// log.Printf("[BFS-BRANCH-WORKER-CYCLE] Siklus terdeteksi: mencoba membuat %s atau %s yang sudah ada di path tree untuk %s dalam target %s.", pair.Mat1, pair.Mat2, elementToProcess, initialTargetForLogging)
				continue
			}

			currentStep := pathfinding.PathStep{
				ChildName:   elementToProcess,
				Parent1Name: pair.Mat1,
				Parent2Name: pair.Mat2,
			}

			newPathTaken := make([]pathfinding.PathStep, len(currentState.PathTakenSoFar))
			copy(newPathTaken, currentState.PathTakenSoFar)
			newPathTaken = append(newPathTaken, currentStep)

			nextElementsToDeconstruct := make([]string, len(remainingToDeconstructForNextState))
			copy(nextElementsToDeconstruct, remainingToDeconstructForNextState)
			
			tempDeconstructSet := make(map[string]bool)
			for _, el := range nextElementsToDeconstruct { tempDeconstructSet[el] = true }
			
			if !graph.BaseElements[pair.Mat1] && !tempDeconstructSet[pair.Mat1] {
				nextElementsToDeconstruct = append(nextElementsToDeconstruct, pair.Mat1)
			}
			if !graph.BaseElements[pair.Mat2] && !tempDeconstructSet[pair.Mat2] {
				nextElementsToDeconstruct = append(nextElementsToDeconstruct, pair.Mat2)
			}
			sort.Strings(nextElementsToDeconstruct) // Untuk konsistensi (opsional)

			newState := BFSMPStateBackward{ // Menggunakan struct BFSMPStateBackward yang sudah ada
				ElementsToDeconstruct:     nextElementsToDeconstruct,
				PathTakenSoFar:            newPathTaken,
				elementsInCurrentPathTree: newElementsInPathTreeForChildren, 
			}
			workerQueue.PushBack(newState)
		}
	}

	
	atomic.AddInt64(nodesExploredCounter, int64(workerNodesExploredThisInstance))
	// log.Printf("[BFS-BRANCH-WORKER] Selesai untuk cabang dari target %s. Total node dieksplorasi oleh worker ini: %d", initialTargetForLogging, workerNodesExploredThisInstance)
}

// BFSFindXDifferentPathsBackward_Parallel adalah fungsi orkestrator baru untuk BFS multi-threaded.
// Fungsi ini meluncurkan worker untuk setiap resep awal dari elemen target.
func BFSFindXDifferentPathsBackward_Parallel(graph *loadrecipes.BiGraphAlchemy, targetElementName string, maxPaths int) (*pathfinding.MultipleResult, error) {
	if _, targetExists := graph.AllElements[targetElementName]; !targetExists {
		return nil, fmt.Errorf("elemen target '%s' tidak ditemukan dalam data (versi paralel)", targetElementName)
	}
	if maxPaths <= 0 {
		return nil, fmt.Errorf("maxPaths harus integer positif (versi paralel)")
	}

	var collectedPathResults []pathfinding.Result // Hasil akhir (jalur unik)
	uniquePathSignaturesGlobal := make(map[string]bool) // Untuk melacak keunikan path secara global
	var totalNodesExploredGlobal int64 // Counter atomik untuk semua node yang dieksplorasi

	// Kasus dasar: jika target adalah elemen dasar
	if graph.BaseElements[targetElementName] {
		log.Printf("[BFS-PARALLEL-ORCHESTRATOR] Target '%s' adalah elemen dasar.", targetElementName)
		atomic.AddInt64(&totalNodesExploredGlobal, 1) // Hitung sebagai 1 node
		return &pathfinding.MultipleResult{
			Results: []pathfinding.Result{{Path: []pathfinding.PathStep{}, NodesVisited: 1}},
		}, nil
	}

	// Dapatkan semua resep awal (langkah pertama) untuk targetElementName
	initialParentPairs, hasInitialRecipes := graph.ChildToParents[targetElementName]
	if !hasInitialRecipes {
		log.Printf("[BFS-PARALLEL-ORCHESTRATOR] Target '%s' tidak memiliki resep awal.", targetElementName)
		// Meskipun tidak ada resep, kita mungkin telah melakukan pengecekan 'targetExists' yang dihitung sebagai eksplorasi node.
		// Namun, untuk konsistensi dengan BFS asli (yang mungkin tidak menghitung node jika tidak ada resep), kita set ke 0 atau 1.
		// BFS asli mengembalikan totalNodesExplored=0 jika tidak ada path.
		return &pathfinding.MultipleResult{Results: []pathfinding.Result{}}, nil 
	}

	var wg sync.WaitGroup
	// Channel untuk menerima path yang sudah jadi (base-to-target) dari worker.
	// Ukuran buffer bisa disesuaikan.
	rawPathChannel := make(chan []pathfinding.PathStep, len(initialParentPairs)*20) // Meningkatkan buffer
	doneSignal := make(chan struct{}) // Channel untuk memberi sinyal worker agar berhenti

	log.Printf("[BFS-PARALLEL-ORCHESTRATOR] Target: '%s'. Meluncurkan %d worker untuk %d resep awal.", targetElementName, len(initialParentPairs), len(initialParentPairs))

	// Luncurkan satu worker untuk setiap resep awal
	numWorkersLaunched := 0
	for _, initialPair := range initialParentPairs {
		// Optimasi: jika sudah cukup path, jangan luncurkan worker baru
		if len(collectedPathResults) >= maxPaths {
			break 
		}
		numWorkersLaunched++
		wg.Add(1)

		// Buat state awal untuk worker ini
		firstStep := pathfinding.PathStep{
			ChildName:   targetElementName,
			Parent1Name: initialPair.Mat1,
			Parent2Name: initialPair.Mat2,
		}
		
		elementsToDeconstructForWorker := []string{}
		// Hanya tambahkan parent ke dekonstruksi jika BUKAN elemen dasar
		if !graph.BaseElements[initialPair.Mat1] {
			elementsToDeconstructForWorker = append(elementsToDeconstructForWorker, initialPair.Mat1)
		}
		if !graph.BaseElements[initialPair.Mat2] {
			elementsToDeconstructForWorker = append(elementsToDeconstructForWorker, initialPair.Mat2)
		}
		sort.Strings(elementsToDeconstructForWorker) // Untuk konsistensi state (opsional)

		initialWorkerState := BFSMPStateBackward{
			ElementsToDeconstruct:     elementsToDeconstructForWorker,
			PathTakenSoFar:            []pathfinding.PathStep{firstStep}, // Dimulai dengan langkah dekomposisi pertama
			elementsInCurrentPathTree: map[string]bool{targetElementName: true}, // Target awal sudah bagian dari tree
		}

		go bfsBranchWorker(
			graph, 
			initialWorkerState, 
			rawPathChannel, 
			&wg, 
			doneSignal, 
			&totalNodesExploredGlobal, 
		)
	}
	if numWorkersLaunched == 0 && len(initialParentPairs) > 0 { // Jika maxPaths = 0 atau negatif (seharusnya sudah ditangani) atau sangat kecil
		log.Printf("[BFS-PARALLEL-ORCHESTRATOR] Tidak ada worker yang diluncurkan untuk target '%s' (kemungkinan maxPaths sudah terpenuhi atau <=0).", targetElementName)
	}


	// Goroutine untuk menunggu semua worker selesai dan kemudian menutup rawPathChannel
	// Ini penting agar loop pengumpul hasil di bawah bisa berhenti.
	collectorWg := sync.WaitGroup{} // WaitGroup terpisah untuk memastikan kolektor menunggu penutupan channel
	collectorWg.Add(1)
	go func() {
		defer collectorWg.Done()
		wg.Wait() // Tunggu semua bfsBranchWorker selesai
		close(rawPathChannel)
		// log.Printf("[BFS-PARALLEL-ORCHESTRATOR] Semua worker untuk target '%s' telah selesai. Menutup rawPathChannel.", targetElementName)
	}()

	// Kumpulkan dan proses hasil dari worker sampai rawPathChannel ditutup
	// dan semua item yang di-buffer sudah diproses.
	for pathFromWorker := range rawPathChannel {
		if len(collectedPathResults) >= maxPaths {
			// Jika sudah cukup path, beri sinyal ke worker yang mungkin masih berjalan
			select {
			case <-doneSignal: // doneSignal sudah ditutup
			default:
				close(doneSignal)
				// log.Printf("[BFS-PARALLEL-ORCHESTRATOR] Max paths (%d) tercapai untuk '%s'. Memberi sinyal worker lain untuk berhenti.", maxPaths, targetElementName)
			}
			// Lanjutkan mengosongkan channel agar goroutine worker tidak terblokir, tapi jangan tambahkan lagi ke hasil.
			continue
		}

		// Path dari worker sudah dalam urutan base-to-target.
		// Gunakan createPathSignature yang sudah ada untuk memeriksa keunikan.
		sig := createPathSignature(pathFromWorker) 
		if !uniquePathSignaturesGlobal[sig] {
			uniquePathSignaturesGlobal[sig] = true
			// NodesVisited akan diisi dengan nilai global total di akhir.
			collectedPathResults = append(collectedPathResults, pathfinding.Result{Path: pathFromWorker, NodesVisited: 0}) 
			// log.Printf("[BFS-PARALLEL-ORCHESTRATOR-DEBUG] Mengumpulkan path unik ke-%d untuk '%s'. Signature: %s", len(collectedPathResults), targetElementName, sig)

			// Cek lagi setelah menambahkan, kalau-kalau ini adalah path terakhir yang dibutuhkan
			if len(collectedPathResults) >= maxPaths {
				select {
				case <-doneSignal: // Sudah ditutup
				default:
					close(doneSignal)
					// log.Printf("[BFS-PARALLEL-ORCHESTRATOR] Max paths (%d) tercapai saat pengumpulan untuk '%s'. Memberi sinyal worker.", maxPaths, targetElementName)
				}
			}
		}
	}

	collectorWg.Wait() // Pastikan goroutine penutup channel sudah selesai (artinya semua worker sudah wg.Done())

	// Pastikan doneSignal ditutup jika loop selesai karena channel ditutup 
	// (misalnya, semua worker selesai dan menemukan < maxPaths path).
	select {
	case <-doneSignal:
	default:
		close(doneSignal)
	}
	
	// Atur NodesVisited untuk semua path yang terkumpul.
	// Ini meniru perilaku BFS sekuensial di mana NodesVisited adalah hitungan global.
	finalNodesExploredCount := int(atomic.LoadInt64(&totalNodesExploredGlobal))
	
	// Jika tidak ada worker yang diluncurkan tapi ada resep awal (misal maxPaths=0), nodesExplored bisa 0.
	// Jika target bukan base dan tidak ada worker diluncurkan atau tidak ada path ditemukan,
	// setidaknya ada 1 node (target itu sendiri) yang "dieksplorasi" dalam arti diperiksa.
	// BFSFindXDifferentPathsBackward yang asli akan memiliki totalNodesExplored > 0 jika ada iterasi.
	// Untuk konsistensi, jika ada usaha (worker diluncurkan atau resep awal ada), nodes explored > 0.
	if finalNodesExploredCount == 0 && numWorkersLaunched > 0 && len(collectedPathResults) == 0 && !graph.BaseElements[targetElementName] {
		// Jika worker diluncurkan tapi tidak ada yang dieksplorasi (misal semua cabang awal langsung buntu)
		// Anggap setiap upaya peluncuran worker sebagai eksplorasi minimal.
		// Namun, counter atomik seharusnya sudah menangkap ini jika worker sempat berjalan.
		// Paling aman adalah membiarkan counter atomik apa adanya, kecuali jika 0 dan ada path.
	}
	if finalNodesExploredCount == 0 && len(collectedPathResults) > 0 {
		finalNodesExploredCount = 1 // Minimal 1 jika ada hasil tapi counter belum update (jarang terjadi)
	}
	// Jika tidak ada hasil dan bukan elemen base, dan tidak ada worker diluncurkan, NodesVisited dari MultipleResult akan 0.
	// Jika target adalah elemen base, nodes visited adalah 1 (sudah ditangani di atas).

	for i := range collectedPathResults {
		collectedPathResults[i].NodesVisited = finalNodesExploredCount
	}

	if len(collectedPathResults) == 0 && !graph.BaseElements[targetElementName] {
		 log.Printf("[BFS-PARALLEL-ORCHESTRATOR-INFO] Tidak ada jalur unik yang ditemukan untuk '%s'. Total node dieksplorasi: %d", targetElementName, finalNodesExploredCount)
	} else if len(collectedPathResults) > 0 {
		 log.Printf("[BFS-PARALLEL-ORCHESTRATOR-INFO] Selesai untuk target '%s'. Ditemukan %d jalur unik. Total node dieksplorasi: %d", targetElementName, len(collectedPathResults), finalNodesExploredCount)
	} else if graph.BaseElements[targetElementName] {
		// Sudah ditangani, tapi untuk logging bisa ditambahkan di sini jika perlu
	}


	return &pathfinding.MultipleResult{Results: collectedPathResults}, nil
}