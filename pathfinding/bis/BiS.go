package bis

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
	// TimeoutDuration   time.Duration // Dihapus

	VisitedForward    map[string][]pathfinding.PathStep
	VisitedBackward   map[string][]pathfinding.PathStep
	
	FoundRecipes      []pathfinding.Result
	RecipeSignatures  map[string]bool      
	
	Mutex             sync.RWMutex

	NodesExplored     int64 
	FoundRecipesCount int32 
	
	StopSearch        chan struct{} 
	Wg                sync.WaitGroup
}

// createBiSPathSignature membuat representasi string kanonis dari sebuah jalur resep.
func createBiSPathSignature(steps []pathfinding.PathStep) string {
	if len(steps) == 0 {
		return "base_element_or_empty_path"
	}

	pathCopy := make([]pathfinding.PathStep, len(steps))
	copy(pathCopy, steps)

	for i := range pathCopy {
		if pathCopy[i].Parent1Name > pathCopy[i].Parent2Name {
			pathCopy[i].Parent1Name, pathCopy[i].Parent2Name = pathCopy[i].Parent2Name, pathCopy[i].Parent1Name
		}
	}

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
	fullRecipe := make([]pathfinding.PathStep, 0, len(pathForward)+len(pathBackwardDeconstruction))
	fullRecipe = append(fullRecipe, pathForward...)

	for i := len(pathBackwardDeconstruction) - 1; i >= 0; i-- {
		fullRecipe = append(fullRecipe, pathBackwardDeconstruction[i])
	}
	return fullRecipe
}

// processMeetingPoint memproses titik pertemuan, merekonstruksi resep, dan menambahkannya jika unik.
func processMeetingPoint(shared *BiSSharedData, meetingElement string, currentPathForward []pathfinding.PathStep, pathFromTargetToMeetingBackward []pathfinding.PathStep) { // Hapus nodesVisitedAtThisPoint
	if atomic.LoadInt32(&shared.FoundRecipesCount) >= int32(shared.MaxRecipes) {
		return 
	}

	recipeSteps := reconstructRecipe(meetingElement, currentPathForward, pathFromTargetToMeetingBackward)
	signature := createBiSPathSignature(recipeSteps)

	shared.Mutex.Lock()
	defer shared.Mutex.Unlock()

	if !shared.RecipeSignatures[signature] {
		if atomic.LoadInt32(&shared.FoundRecipesCount) < int32(shared.MaxRecipes) {
			shared.RecipeSignatures[signature] = true
			shared.FoundRecipes = append(shared.FoundRecipes, pathfinding.Result{
				Path:         recipeSteps,
				NodesVisited: int(atomic.LoadInt64(&shared.NodesExplored)), 
			})
			atomic.AddInt32(&shared.FoundRecipesCount, 1)
			log.Printf("[BiS-INFO] Recipe %d found for %s via %s. Signature: %s. Steps: %d", atomic.LoadInt32(&shared.FoundRecipesCount), shared.TargetElement, meetingElement, signature, len(recipeSteps))

			if atomic.LoadInt32(&shared.FoundRecipesCount) >= int32(shared.MaxRecipes) {
				select {
				case <-shared.StopSearch: 
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
	meetingCheckChan chan string, 
) {
	defer shared.Wg.Done()

	for _, item := range itemsToExpand {
		select {
		case <-shared.StopSearch:
			return 
		default:
		}

		currentElement := item.ElementName
		currentPath := item.PathSoFar
		
		elementsToCombineWith := make([]string, 0)
		for baseElem := range shared.Graph.BaseElements {
			elementsToCombineWith = append(elementsToCombineWith, baseElem)
		}
		
		shared.Mutex.RLock()
		for visitedElem := range shared.VisitedForward {
			elementsToCombineWith = append(elementsToCombineWith, visitedElem)
		}
		shared.Mutex.RUnlock()

		processedCombinations := make(map[loadrecipes.PairMats]bool)

		for _, partnerElement := range elementsToCombineWith {
			select {
			case <-shared.StopSearch:
				return
			default:
			}

			pair := loadrecipes.ConstructPair(currentElement, partnerElement)
			if processedCombinations[pair] {
				continue
			}
			processedCombinations[pair] = true
			atomic.AddInt64(&shared.NodesExplored, 1) 

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

					shared.Mutex.Lock() 
					if _, visited := shared.VisitedForward[childName]; !visited || len(newPath) < len(shared.VisitedForward[childName]) {
						shared.VisitedForward[childName] = newPath
						nextForwardQueueChan <- BiSQueueItem{ElementName: childName, PathSoFar: newPath}
						
						if _, met := shared.VisitedBackward[childName]; met {
							meetingCheckChan <- childName 
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
	meetingCheckChan chan string, 
) {
	defer shared.Wg.Done()

	for _, item := range itemsToExpand {
		select {
		case <-shared.StopSearch:
			return 
		default:
		}

		currentElement := item.ElementName 
		currentPathDeconstruction := item.PathSoFar 

		atomic.AddInt64(&shared.NodesExplored, 1)

		parentPairs, hasRecipes := shared.Graph.ChildToParents[currentElement]
		if !hasRecipes {
			continue
		}

		for _, pair := range parentPairs { 
			select {
			case <-shared.StopSearch:
				return
			default:
			}
			
			deconstructionStep := pathfinding.PathStep{ChildName: currentElement, Parent1Name: pair.Mat1, Parent2Name: pair.Mat2}

			newPathToParent1 := make([]pathfinding.PathStep, len(currentPathDeconstruction))
			copy(newPathToParent1, currentPathDeconstruction)
			newPathToParent1 = append(newPathToParent1, deconstructionStep) 

			shared.Mutex.Lock()
			if _, visited := shared.VisitedBackward[pair.Mat1]; !visited || len(newPathToParent1) < len(shared.VisitedBackward[pair.Mat1]) {
				shared.VisitedBackward[pair.Mat1] = newPathToParent1
				nextBackwardQueueChan <- BiSQueueItem{ElementName: pair.Mat1, PathSoFar: newPathToParent1}
				if _, met := shared.VisitedForward[pair.Mat1]; met {
					meetingCheckChan <- pair.Mat1
				}
			}
			shared.Mutex.Unlock()
			
			newPathToParent2 := make([]pathfinding.PathStep, len(currentPathDeconstruction))
			copy(newPathToParent2, currentPathDeconstruction)
			newPathToParent2 = append(newPathToParent2, deconstructionStep) 

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
// Mengembalikan MultipleResult, jumlah total node yang dieksplorasi, dan error.
func BiSFindMultiplePaths(graph *loadrecipes.BiGraphAlchemy, targetElement string, maxRecipes int) (*pathfinding.MultipleResult, int, error) {
	if _, exists := graph.AllElements[targetElement]; !exists {
		return nil, 0, fmt.Errorf("elemen target '%s' tidak ditemukan dalam data", targetElement)
	}
	if maxRecipes <= 0 {
		return nil, 0, fmt.Errorf("maxRecipes harus integer positif")
	}

	if graph.BaseElements[targetElement] {
		return &pathfinding.MultipleResult{
			Results: []pathfinding.Result{
				{Path: []pathfinding.PathStep{}, NodesVisited: 1},
			},
		}, 1, nil // 1 node (elemen dasar itu sendiri) dieksplorasi
	}

	shared := &BiSSharedData{
		Graph:             graph,
		TargetElement:     targetElement,
		MaxRecipes:        maxRecipes,
		// TimeoutDuration: Dihapus
		VisitedForward:    make(map[string][]pathfinding.PathStep),
		VisitedBackward:   make(map[string][]pathfinding.PathStep),
		FoundRecipes:      make([]pathfinding.Result, 0, maxRecipes),
		RecipeSignatures:  make(map[string]bool),
		StopSearch:        make(chan struct{}),
		NodesExplored:     0,
		FoundRecipesCount: 0,
	}

	qForward := list.New()
	qBackward := list.New()

	for baseElem := range graph.BaseElements {
		shared.VisitedForward[baseElem] = []pathfinding.PathStep{}
		qForward.PushBack(BiSQueueItem{ElementName: baseElem, PathSoFar: []pathfinding.PathStep{}})
		atomic.AddInt64(&shared.NodesExplored, 1)
	}

	shared.VisitedBackward[targetElement] = []pathfinding.PathStep{}
	qBackward.PushBack(BiSQueueItem{ElementName: targetElement, PathSoFar: []pathfinding.PathStep{}})
	atomic.AddInt64(&shared.NodesExplored, 1)

	iteration := 0
	// maxIterations bisa disesuaikan atau dibuat sebagai parameter jika diperlukan
	// Untuk saat ini, kita akan mengandalkan kondisi berhenti lain (antrian kosong atau maxRecipes tercapai)
	// Namun, menambahkan batas iterasi tetap merupakan praktik yang baik untuk mencegah loop tak terbatas dalam kasus yang sangat kompleks.
	maxIterations := 200 // Batas iterasi yang lebih masuk akal untuk pencarian resep

	for qForward.Len() > 0 && qBackward.Len() > 0 && atomic.LoadInt32(&shared.FoundRecipesCount) < int32(maxRecipes) && iteration < maxIterations {
		select {
		case <-shared.StopSearch:
			log.Println("[BiS-INFO] Pencarian dihentikan karena sinyal StopSearch.")
			goto endSearch
		// case <-timeout: // Logika timeout dihapus
		// 	log.Println("[BiS-WARN] Pencarian dihentikan karena timeout.")
		// 	goto endSearch
		default:
		}
		iteration++
		log.Printf("[BiS-DEBUG] Iterasi %d, Resep Ditemukan: %d, QF: %d, QB: %d, VF: %d, VB: %d, Nodes: %d\n",
			iteration, atomic.LoadInt32(&shared.FoundRecipesCount), qForward.Len(), qBackward.Len(), len(shared.VisitedForward), len(shared.VisitedBackward), atomic.LoadInt64(&shared.NodesExplored))

		currentForwardItems := make([]BiSQueueItem, 0, qForward.Len())
		for qForward.Len() > 0 {
			currentForwardItems = append(currentForwardItems, qForward.Remove(qForward.Front()).(BiSQueueItem))
		}
		
		nextForwardQueueChan := make(chan BiSQueueItem, len(shared.Graph.AllElements)) 
		meetingCheckChanForward := make(chan string, len(shared.Graph.AllElements)) 

		numForwardWorkers := len(currentForwardItems) 
		if numForwardWorkers > 0 {
			log.Printf("[BiS-DEBUG] Iterasi %d: Meluncurkan %d forward worker.", iteration, numForwardWorkers)
			shared.Wg.Add(numForwardWorkers)
			for _, item := range currentForwardItems {
				itemCopy := item 
				go expandForwardWorker(shared, []BiSQueueItem{itemCopy}, nextForwardQueueChan, meetingCheckChanForward)
			}
		}
		
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
		
		shared.Wg.Wait()
		close(nextForwardQueueChan)
		close(nextBackwardQueueChan)
		close(meetingCheckChanForward)
		close(meetingCheckChanBackward)
		
		log.Printf("[BiS-DEBUG] Iterasi %d: Semua worker selesai.", iteration)

		for item := range nextForwardQueueChan {
			qForward.PushBack(item)
		}
		for item := range nextBackwardQueueChan {
			qBackward.PushBack(item)
		}

		for meetingElem := range meetingCheckChanForward {
			shared.Mutex.RLock()
			pathFwd, okFwd := shared.VisitedForward[meetingElem]
			pathBwd, okBwd := shared.VisitedBackward[meetingElem]
			shared.Mutex.RUnlock()
			if okFwd && okBwd {
				processMeetingPoint(shared, meetingElem, pathFwd, pathBwd)
			}
		}
		for meetingElem := range meetingCheckChanBackward {
			shared.Mutex.RLock()
			pathFwd, okFwd := shared.VisitedForward[meetingElem]
			pathBwd, okBwd := shared.VisitedBackward[meetingElem]
			shared.Mutex.RUnlock()
			if okFwd && okBwd {
				processMeetingPoint(shared, meetingElem, pathFwd, pathBwd)
			}
		}
		
		if atomic.LoadInt32(&shared.FoundRecipesCount) >= int32(maxRecipes) {
			log.Printf("[BiS-INFO] Batas resep tercapai setelah iterasi %d.", iteration)
			break
		}
	}

endSearch:
	select {
	case <-shared.StopSearch:
	default:
		close(shared.StopSearch)
	}
	
	shared.Mutex.RLock()
	finalResults := make([]pathfinding.Result, len(shared.FoundRecipes))
	copy(finalResults, shared.FoundRecipes)
	shared.Mutex.RUnlock()

	totalNodesExplored := int(atomic.LoadInt64(&shared.NodesExplored))

	if len(finalResults) == 0 && !graph.BaseElements[targetElement] {
		log.Printf("[BiS-WARN] Tidak ada resep ditemukan untuk %s setelah %d iterasi.", targetElement, iteration)
		return &pathfinding.MultipleResult{Results: []pathfinding.Result{}}, totalNodesExplored, fmt.Errorf("tidak ada jalur resep yang ditemukan untuk elemen '%s'", targetElement)
	}
	if iteration >= maxIterations && atomic.LoadInt32(&shared.FoundRecipesCount) < int32(maxRecipes) {
		log.Printf("[BiS-WARN] Pencarian mencapai batas iterasi maksimum (%d) untuk %s sebelum menemukan %d resep.", maxIterations, targetElement, maxRecipes)
	}
	
	log.Printf("[BiS-INFO] Pencarian selesai untuk %s. Ditemukan %d resep. Total node dieksplorasi: %d.", targetElement, len(finalResults), totalNodesExplored)
	return &pathfinding.MultipleResult{Results: finalResults}, totalNodesExplored, nil
}
