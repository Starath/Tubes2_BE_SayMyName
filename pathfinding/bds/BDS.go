package bis

import (
	"container/list"
	"fmt"
	"log"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Starath/Tubes2_BE_SayMyName/loadrecipes"
	"github.com/Starath/Tubes2_BE_SayMyName/pathfinding"
)

// BiSQueueItem merepresentasikan item dalam antrian pencarian BiS.
// Ini menyimpan nama elemen dan jalur (langkah-langkah resep) untuk mencapainya.
type BiSQueueItem struct {
	ElementName string
	PathSoFar   []pathfinding.PathStep // PathSoFar untuk forward: dari base ke ElementName.
	                                 // PathSoFar untuk backward: dari Target ke ElementName (langkah dekonstruksi).
}

// BiSSharedData menyimpan data yang dibagikan antar goroutine selama pencarian BiS.
// Sinkronisasi diperlukan untuk mengakses data ini secara aman.
type BiSSharedData struct {
	Graph             *loadrecipes.BiGraphAlchemy
	TargetElement     string
	MaxRecipes        int
	TimeoutDuration   time.Duration // Durasi timeout untuk pencarian

	// VisitedForward: map dari nama elemen ke jalur terpendek (sebagai slice PathStep) dari elemen dasar.
	VisitedForward    map[string][]pathfinding.PathStep
	// VisitedBackward: map dari nama elemen ke jalur dekonstruksi terpendek (sebagai slice PathStep) dari elemen target.
	VisitedBackward   map[string][]pathfinding.PathStep
	
	FoundRecipes      []pathfinding.Result // Daftar hasil resep unik yang ditemukan.
	RecipeSignatures  map[string]bool      // Digunakan untuk memastikan keunikan resep.
	
	Mutex             sync.RWMutex // Melindungi akses ke VisitedForward, VisitedBackward, FoundRecipes, RecipeSignatures.

	NodesExplored     int64 // Counter atomik untuk jumlah node yang dieksplorasi.
	FoundRecipesCount int32 // Counter atomik untuk jumlah resep yang ditemukan.
	
	StopSearch        chan struct{} // Channel untuk memberi sinyal berhenti ke semua goroutine.
	Wg                sync.WaitGroup
}

// createBiSPathSignature membuat representasi string kanonis dari sebuah jalur resep.
// Ini digunakan untuk memeriksa keunikan resep.
// Orang tua dalam setiap langkah diurutkan, dan langkah-langkah dalam jalur juga diurutkan.
func createBiSPathSignature(steps []pathfinding.PathStep) string {
	if len(steps) == 0 {
		// Ini bisa terjadi jika elemen target adalah elemen dasar.
		return "base_element_or_empty_path"
	}

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

// reconstructRecipe menggabungkan jalur dari pencarian maju dan mundur saat bertemu.
func reconstructRecipe(meetingElement string, pathForward []pathfinding.PathStep, pathBackwardDeconstruction []pathfinding.PathStep) []pathfinding.PathStep {
	// pathForward adalah resep dari elemen dasar ke meetingElement.
	// pathBackwardDeconstruction adalah langkah-langkah dekonstruksi dari targetElement ke meetingElement.
	// Kita perlu membalik pathBackwardDeconstruction untuk mendapatkan langkah konstruksi dari meetingElement ke targetElement.
	
	fullRecipe := make([]pathfinding.PathStep, 0, len(pathForward)+len(pathBackwardDeconstruction))
	fullRecipe = append(fullRecipe, pathForward...)

	// Balik urutan langkah-langkah dekonstruksi untuk mendapatkan konstruksi dari meeting ke target
	for i := len(pathBackwardDeconstruction) - 1; i >= 0; i-- {
		fullRecipe = append(fullRecipe, pathBackwardDeconstruction[i])
	}
	return fullRecipe
}

// processMeetingPoint memproses titik pertemuan, merekonstruksi resep, dan menambahkannya jika unik.
func processMeetingPoint(shared *BiSSharedData, meetingElement string, currentPathForward []pathfinding.PathStep, pathFromTargetToMeetingBackward []pathfinding.PathStep, nodesVisitedAtThisPoint int64) {
	if atomic.LoadInt32(&shared.FoundRecipesCount) >= int32(shared.MaxRecipes) {
		return // Sudah cukup resep ditemukan
	}

	// Rekonstruksi resep lengkap
	// currentPathForward adalah jalur dari base ke meetingElement
	// pathFromTargetToMeetingBackward adalah jalur dekonstruksi dari Target ke meetingElement
	recipeSteps := reconstructRecipe(meetingElement, currentPathForward, pathFromTargetToMeetingBackward)
	
	// Buat signature untuk memeriksa keunikan
	signature := createBiSPathSignature(recipeSteps)

	shared.Mutex.Lock()
	defer shared.Mutex.Unlock()

	if !shared.RecipeSignatures[signature] {
		if atomic.LoadInt32(&shared.FoundRecipesCount) < int32(shared.MaxRecipes) {
			shared.RecipeSignatures[signature] = true
			// Catatan: NodesVisited di sini bisa diinterpretasikan sebagai total node yang dieksplorasi hingga resep ini ditemukan.
			// Untuk BiS, ini bisa menjadi jumlah node dari kedua arah atau total global saat ini.
			// Untuk konsistensi dengan DFS, kita akan menggunakan shared.NodesExplored saat ini.
			// Namun, untuk hasil per resep, mungkin lebih baik menyimpan snapshot NodesExplored saat resep ditemukan.
			// Untuk saat ini, kita akan gunakan nilai global saat resep ditambahkan.
			shared.FoundRecipes = append(shared.FoundRecipes, pathfinding.Result{
				Path:         recipeSteps,
				NodesVisited: int(atomic.LoadInt64(&shared.NodesExplored)), // Atau nodesVisitedAtThisPoint
			})
			atomic.AddInt32(&shared.FoundRecipesCount, 1)
			log.Printf("[BiS-INFO] Recipe %d found for %s via %s. Signature: %s. Steps: %d", atomic.LoadInt32(&shared.FoundRecipesCount), shared.TargetElement, meetingElement, signature, len(recipeSteps))

			if atomic.LoadInt32(&shared.FoundRecipesCount) >= int32(shared.MaxRecipes) {
				// Beri sinyal untuk menghentikan pencarian lain jika sudah mencapai batas
				select {
				case <-shared.StopSearch: // Sudah ditutup
				default:
					close(shared.StopSearch)
					log.Printf("[BiS-INFO] Max recipes (%d) reached for %s. Signaling stop.", shared.MaxRecipes, shared.TargetElement)
				}
			}
		}
	}
}

// expandForwardWorker adalah goroutine untuk satu langkah ekspansi dari frontier maju.
func expandForwardWorker(
	shared *BiSSharedData,
	itemsToExpand []BiSQueueItem,
	nextForwardQueueChan chan BiSQueueItem,
	meetingCheckChan chan string, // Elemen yang baru ditemukan oleh forward untuk dicek di backward visited
) {
	defer shared.Wg.Done()

	for _, item := range itemsToExpand {
		select {
		case <-shared.StopSearch:
			return // Hentikan jika sudah diperintahkan
		default:
		}

		currentElement := item.ElementName
		currentPath := item.PathSoFar

		// Strategi ekspansi maju:
		// 1. Kombinasikan currentElement dengan semua elemen dasar.
		// 2. Kombinasikan currentElement dengan semua elemen yang sudah ada di VisitedForward.
		
		elementsToCombineWith := make([]string, 0)
		for baseElem := range shared.Graph.BaseElements {
			elementsToCombineWith = append(elementsToCombineWith, baseElem)
		}
		
		shared.Mutex.RLock()
		for visitedElem := range shared.VisitedForward {
			// Hindari kombinasi elemen dengan dirinya sendiri jika tidak menghasilkan elemen baru yang berguna
			// atau jika kombinasi tersebut sudah dieksplorasi melalui urutan lain.
			// Untuk kesederhanaan, kita izinkan, ConstructPair akan menanganinya.
			elementsToCombineWith = append(elementsToCombineWith, visitedElem)
		}
		shared.Mutex.RUnlock()


		// Deduplikasi kombinasi untuk efisiensi
		processedCombinations := make(map[loadrecipes.PairMats]bool)

		for _, partnerElement := range elementsToCombineWith {
			select {
			case <-shared.StopSearch:
				return
			default:
			}

			// Bentuk pasangan kanonis untuk lookup di ParentPairToChild
			pair := loadrecipes.ConstructPair(currentElement, partnerElement)
			if processedCombinations[pair] {
				continue
			}
			processedCombinations[pair] = true
			atomic.AddInt64(&shared.NodesExplored, 1) // Anggap setiap percobaan kombinasi sebagai eksplorasi

			children, canCombine := shared.Graph.ParentPairToChild[pair]
			if canCombine {
				for _, childName := range children {
					select {
					case <-shared.StopSearch:
						return
					default:
					}

					newStep := pathfinding.PathStep{ChildName: childName, Parent1Name: pair.Mat1, Parent2Name: pair.Mat2}
					newPath := make([]pathfinding.PathStep, len(currentPath))
					copy(newPath, currentPath)
					newPath = append(newPath, newStep)

					shared.Mutex.Lock() // Lock untuk VisitedForward dan pengecekan pertemuan
					if _, visited := shared.VisitedForward[childName]; !visited || len(newPath) < len(shared.VisitedForward[childName]) {
						// Update jika path baru lebih pendek atau belum dikunjungi
						shared.VisitedForward[childName] = newPath
						
						// Kirim ke channel untuk diproses di antrian level berikutnya
						nextForwardQueueChan <- BiSQueueItem{ElementName: childName, PathSoFar: newPath}
						
						// Periksa pertemuan dengan VisitedBackward
						if _, met := shared.VisitedBackward[childName]; met {
							// Penting: proses pertemuan harus dilakukan setelah unlock jika memanggil fungsi yang juga lock
							// Atau, pastikan processMeetingPoint tidak melakukan lock yang sama secara rekursif.
							// Untuk sekarang, kita kirim info pertemuan ke channel lain atau proses di sini dengan hati-hati.
							// Kita akan memproses pertemuan di main loop setelah ekspansi level selesai.
							meetingCheckChan <- childName // Kirim elemen pertemuan untuk dicek nanti
						}
					}
					shared.Mutex.Unlock()
				}
			}
		}
	}
}

// expandBackwardWorker adalah goroutine untuk satu langkah ekspansi dari frontier mundur.
func expandBackwardWorker(
	shared *BiSSharedData,
	itemsToExpand []BiSQueueItem,
	nextBackwardQueueChan chan BiSQueueItem,
	meetingCheckChan chan string, // Elemen (parent) yang baru ditemukan oleh backward untuk dicek di forward visited
) {
	defer shared.Wg.Done()

	for _, item := range itemsToExpand {
		select {
		case <-shared.StopSearch:
			return // Hentikan jika sudah diperintahkan
		default:
		}

		currentElement := item.ElementName // Ini adalah 'child' yang sedang didekonstruksi
		currentPathDeconstruction := item.PathSoFar // Path dekonstruksi dari Target ke currentElement

		atomic.AddInt64(&shared.NodesExplored, 1)

		parentPairs, hasRecipes := shared.Graph.ChildToParents[currentElement]
		if !hasRecipes {
			continue
		}

		for _, pair := range parentPairs { // pair.Mat1 dan pair.Mat2 adalah parent dari currentElement
			select {
			case <-shared.StopSearch:
				return
			default:
			}
			
			// Langkah dekonstruksi baru dari currentElement ke parent-parentnya
			deconstructionStep := pathfinding.PathStep{ChildName: currentElement, Parent1Name: pair.Mat1, Parent2Name: pair.Mat2}

			// Proses untuk Parent1 (pair.Mat1)
			newPathToParent1 := make([]pathfinding.PathStep, len(currentPathDeconstruction))
			copy(newPathToParent1, currentPathDeconstruction)
			newPathToParent1 = append(newPathToParent1, deconstructionStep) // Path dari Target ke pair.Mat1

			shared.Mutex.Lock()
			if _, visited := shared.VisitedBackward[pair.Mat1]; !visited || len(newPathToParent1) < len(shared.VisitedBackward[pair.Mat1]) {
				shared.VisitedBackward[pair.Mat1] = newPathToParent1
				nextBackwardQueueChan <- BiSQueueItem{ElementName: pair.Mat1, PathSoFar: newPathToParent1}
				if _, met := shared.VisitedForward[pair.Mat1]; met {
					meetingCheckChan <- pair.Mat1
				}
			}
			shared.Mutex.Unlock()
			
			// Proses untuk Parent2 (pair.Mat2)
			// Path dekonstruksi ke Parent2 juga menggunakan deconstructionStep yang sama
			newPathToParent2 := make([]pathfinding.PathStep, len(currentPathDeconstruction))
			copy(newPathToParent2, currentPathDeconstruction)
			newPathToParent2 = append(newPathToParent2, deconstructionStep) // Path dari Target ke pair.Mat2

			shared.Mutex.Lock()
			if _, visited := shared.VisitedBackward[pair.Mat2]; !visited || len(newPathToParent2) < len(shared.VisitedBackward[pair.Mat2]) {
				shared.VisitedBackward[pair.Mat2] = newPathToParent2
				nextBackwardQueueChan <- BiSQueueItem{ElementName: pair.Mat2, PathSoFar: newPathToParent2}
				if _, met := shared.VisitedForward[pair.Mat2]; met {
					meetingCheckChan <- pair.Mat2
				}
			}
			shared.Mutex.Unlock()
		}
	}
}


// BiSFindMultiplePaths adalah fungsi utama untuk Pencarian Bidireksional Multithreaded.
func BiSFindMultiplePaths(graph *loadrecipes.BiGraphAlchemy, targetElement string, maxRecipes int, timeoutSeconds int) (*pathfinding.MultipleResult, error) {
	if _, exists := graph.AllElements[targetElement]; !exists {
		return nil, fmt.Errorf("elemen target '%s' tidak ditemukan dalam data", targetElement)
	}
	if maxRecipes <= 0 {
		return nil, fmt.Errorf("maxRecipes harus integer positif")
	}

	// Kasus dasar: jika target adalah elemen dasar
	if graph.BaseElements[targetElement] {
		return &pathfinding.MultipleResult{
			Results: []pathfinding.Result{
				{Path: []pathfinding.PathStep{}, NodesVisited: 1},
			},
		}, nil
	}

	shared := &BiSSharedData{
		Graph:             graph,
		TargetElement:     targetElement,
		MaxRecipes:        maxRecipes,
		TimeoutDuration:   time.Duration(timeoutSeconds) * time.Second,
		VisitedForward:    make(map[string][]pathfinding.PathStep),
		VisitedBackward:   make(map[string][]pathfinding.PathStep),
		FoundRecipes:      make([]pathfinding.Result, 0, maxRecipes),
		RecipeSignatures:  make(map[string]bool),
		StopSearch:        make(chan struct{}),
		NodesExplored:     0,
		FoundRecipesCount: 0,
	}

	// Inisialisasi antrian dan visited sets
	qForward := list.New()
	qBackward := list.New()

	// Inisialisasi pencarian maju dari elemen dasar
	for baseElem := range graph.BaseElements {
		shared.VisitedForward[baseElem] = []pathfinding.PathStep{}
		qForward.PushBack(BiSQueueItem{ElementName: baseElem, PathSoFar: []pathfinding.PathStep{}})
		atomic.AddInt64(&shared.NodesExplored, 1)
		// Cek pertemuan awal jika target adalah elemen dasar (sudah ditangani di atas)
	}

	// Inisialisasi pencarian mundur dari elemen target
	shared.VisitedBackward[targetElement] = []pathfinding.PathStep{}
	qBackward.PushBack(BiSQueueItem{ElementName: targetElement, PathSoFar: []pathfinding.PathStep{}})
	atomic.AddInt64(&shared.NodesExplored, 1)
	// Cek pertemuan awal jika target adalah elemen dasar (sudah ditangani di atas)
	// Jika elemen dasar ada di VisitedForward, dan target adalah elemen dasar, ini adalah pertemuan.
	if graph.BaseElements[targetElement] { // Sebenarnya ini sudah ditangani di awal fungsi
		// processMeetingPoint(shared, targetElement, []pathfinding.PathStep{}, []pathfinding.PathStep{})
		// Tidak perlu, karena kasus dasar sudah mengembalikan hasil.
	}


	// Timer untuk timeout
	timeout := time.After(shared.TimeoutDuration)
	iteration := 0
	maxIterations := 100 // Batas iterasi untuk mencegah loop tak terbatas pada graf yang sangat besar/kompleks

	// Loop utama pencarian lapis demi lapis
	for qForward.Len() > 0 && qBackward.Len() > 0 && atomic.LoadInt32(&shared.FoundRecipesCount) < int32(maxRecipes) && iteration < maxIterations {
		select {
		case <-shared.StopSearch:
			log.Println("[BiS-INFO] Pencarian dihentikan karena sinyal StopSearch.")
			goto endSearch
		case <-timeout:
			log.Println("[BiS-WARN] Pencarian dihentikan karena timeout.")
			goto endSearch
		default:
		}
		iteration++
		log.Printf("[BiS-DEBUG] Iterasi %d, Resep Ditemukan: %d, QF: %d, QB: %d, VF: %d, VB: %d, Nodes: %d\n",
			iteration, atomic.LoadInt32(&shared.FoundRecipesCount), qForward.Len(), qBackward.Len(), len(shared.VisitedForward), len(shared.VisitedBackward), atomic.LoadInt64(&shared.NodesExplored))

		// --- Fase Ekspansi Maju ---
		currentForwardItems := make([]BiSQueueItem, 0, qForward.Len())
		for qForward.Len() > 0 {
			currentForwardItems = append(currentForwardItems, qForward.Remove(qForward.Front()).(BiSQueueItem))
		}
		
		nextForwardQueueChan := make(chan BiSQueueItem, len(shared.Graph.AllElements)) // Buffer besar
		meetingCheckChanForward := make(chan string, len(shared.Graph.AllElements)) // Buffer untuk elemen pertemuan

		numForwardWorkers := len(currentForwardItems) // Bisa diatur, misal Max(1, Min(numCPU, len/chunkSize))
		if numForwardWorkers > 0 {
			// Idealnya, bagi currentForwardItems menjadi chunk untuk worker, tapi untuk kesederhanaan, satu item per worker jika sedikit
			// atau batasi jumlah worker. Untuk sekarang, kita buat worker sebanyak item.
			// Ini bisa tidak efisien jika item sangat banyak.
			// Lebih baik: chunkSize := 10; numWorkers = (len + chunkSize -1) / chunkSize
			// Dan setiap worker memproses satu chunk.
			// Untuk contoh ini, kita akan spawn satu worker per item di frontier saat ini.

			log.Printf("[BiS-DEBUG] Iterasi %d: Meluncurkan %d forward worker.", iteration, numForwardWorkers)
			shared.Wg.Add(numForwardWorkers)
			for _, item := range currentForwardItems {
				// Perlu membuat salinan item untuk goroutine
				itemCopy := item 
				go expandForwardWorker(shared, []BiSQueueItem{itemCopy}, nextForwardQueueChan, meetingCheckChanForward)
			}
		}
		
		// --- Fase Ekspansi Mundur ---
		currentBackwardItems := make([]BiSQueueItem, 0, qBackward.Len())
		for qBackward.Len() > 0 {
			currentBackwardItems = append(currentBackwardItems, qBackward.Remove(qBackward.Front()).(BiSQueueItem))
		}

		nextBackwardQueueChan := make(chan BiSQueueItem, len(shared.Graph.AllElements))
		meetingCheckChanBackward := make(chan string, len(shared.Graph.AllElements))

		numBackwardWorkers := len(currentBackwardItems)
		if numBackwardWorkers > 0 {
			log.Printf("[BiS-DEBUG] Iterasi %d: Meluncurkan %d backward worker.", iteration, numBackwardWorkers)
			shared.Wg.Add(numBackwardWorkers)
			for _, item := range currentBackwardItems {
				itemCopy := item
				go expandBackwardWorker(shared, []BiSQueueItem{itemCopy}, nextBackwardQueueChan, meetingCheckChanBackward)
			}
		}
		
		// Tunggu semua worker di level ini selesai
		shared.Wg.Wait()
		close(nextForwardQueueChan)
		close(nextBackwardQueueChan)
		close(meetingCheckChanForward)
		close(meetingCheckChanBackward)
		
		log.Printf("[BiS-DEBUG] Iterasi %d: Semua worker selesai.", iteration)

		// Kumpulkan hasil dari channel untuk frontier berikutnya
		for item := range nextForwardQueueChan {
			qForward.PushBack(item)
		}
		for item := range nextBackwardQueueChan {
			qBackward.PushBack(item)
		}

		// Proses pertemuan yang terdeteksi oleh forward workers
		for meetingElem := range meetingCheckChanForward {
			shared.Mutex.RLock()
			pathFwd, okFwd := shared.VisitedForward[meetingElem]
			pathBwd, okBwd := shared.VisitedBackward[meetingElem]
			shared.Mutex.RUnlock()
			if okFwd && okBwd {
				processMeetingPoint(shared, meetingElem, pathFwd, pathBwd, atomic.LoadInt64(&shared.NodesExplored))
			}
		}
		// Proses pertemuan yang terdeteksi oleh backward workers
		for meetingElem := range meetingCheckChanBackward {
			shared.Mutex.RLock()
			pathFwd, okFwd := shared.VisitedForward[meetingElem]
			pathBwd, okBwd := shared.VisitedBackward[meetingElem]
			shared.Mutex.RUnlock()
			if okFwd && okBwd {
				processMeetingPoint(shared, meetingElem, pathFwd, pathBwd, atomic.LoadInt64(&shared.NodesExplored))
			}
		}
		
		if atomic.LoadInt32(&shared.FoundRecipesCount) >= int32(maxRecipes) {
			log.Printf("[BiS-INFO] Batas resep tercapai setelah iterasi %d.", iteration)
			break
		}
	}

endSearch:
	// Pastikan channel StopSearch ditutup jika belum, untuk membersihkan goroutine yang mungkin masih berjalan
	select {
	case <-shared.StopSearch:
	default:
		close(shared.StopSearch)
	}
	
	// Ambil hasil akhir
	shared.Mutex.RLock()
	finalResults := make([]pathfinding.Result, len(shared.FoundRecipes))
	copy(finalResults, shared.FoundRecipes)
	shared.Mutex.RUnlock()

	if len(finalResults) == 0 && !graph.BaseElements[targetElement] {
		log.Printf("[BiS-WARN] Tidak ada resep ditemukan untuk %s setelah %d iterasi.", targetElement, iteration)
		return &pathfinding.MultipleResult{Results: []pathfinding.Result{}}, fmt.Errorf("tidak ada jalur resep yang ditemukan untuk elemen '%s'", targetElement)
	}
	if iteration >= maxIterations {
		log.Printf("[BiS-WARN] Pencarian mencapai batas iterasi maksimum (%d) untuk %s.", maxIterations, targetElement)
	}
	
	log.Printf("[BiS-INFO] Pencarian selesai untuk %s. Ditemukan %d resep. Total node dieksplorasi: %d.", targetElement, len(finalResults), atomic.LoadInt64(&shared.NodesExplored))
	return &pathfinding.MultipleResult{Results: finalResults}, nil
}
