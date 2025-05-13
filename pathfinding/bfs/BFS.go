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

// createPathSignature membuat string unik untuk sebuah path agar bisa deteksi duplikasinya. Path yang diberikan sudah dalam urutan base-ke-target.
func createPathSignature(steps []pathfinding.PathStep) string {
	pathCopy := make([]pathfinding.PathStep, len(steps))
	copy(pathCopy, steps)

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

func proxyBFSWorker(
	originalGraph *loadrecipes.BiGraphAlchemy,
	targetElementName string, 
	assignedInitialRecipe loadrecipes.PairMats, 
	maxPathsForWorkerBranch int, // bisa dibuat maxPathsForWorkerBranch = maxPaths
	rawPathChannel chan<- []pathfinding.PathStep,
	wg *sync.WaitGroup,
	doneSignal <-chan struct{},
	nodesExploredCounter *int64,
) {
	defer wg.Done()

	// Buat graf yang dimodifikasi HANYA untuk worker ini
	modifiedChildToParents := make(map[string][]loadrecipes.PairMats)
	for key, value := range originalGraph.ChildToParents {
		modifiedChildToParents[key] = value 
	}
	// Override resep untuk targetElementName di graf yang dimodifikasi
	modifiedChildToParents[targetElementName] = []loadrecipes.PairMats{assignedInitialRecipe}

	workerGraph := &loadrecipes.BiGraphAlchemy{
		ChildToParents:    modifiedChildToParents,
		ParentPairToChild: originalGraph.ParentPairToChild,
		BaseElements:      originalGraph.BaseElements,
		AllElements:       originalGraph.AllElements,
	}

	// log.Printf("[WORKER-PROXY %s via %s+%s] Dimulai. Max paths worker: %d", targetElementName, assignedInitialRecipe.Mat1, assignedInitialRecipe.Mat2, maxPathsForWorkerBranch)

	select {
	case <-doneSignal:
		// log.Printf("[WORKER-PROXY %s via %s+%s] Berhenti (doneSignal) sebelum BFS sekuensial.", targetElementName, assignedInitialRecipe.Mat1, assignedInitialRecipe.Mat2)
		return
	default:
	}

	result, err := BFSFindXDifferentPathsBackward(workerGraph, targetElementName, maxPathsForWorkerBranch)

	// Akumulasi NodesVisited
	var nodesFromThisCall int
	if result != nil && len(result.Results) > 0 {
		nodesFromThisCall = result.Results[0].NodesVisited
	} else if result != nil && len(result.Results) == 0 && !originalGraph.BaseElements[targetElementName] {
		// Untuk sekarang, jika `result.Results` kosong, `nodesFromThisCall` akan 0.
		// Ini mungkin menjadi sumber perbedaan `totalNodesExploredGlobal` dengan sekuensial.
	}
	atomic.AddInt64(nodesExploredCounter, int64(nodesFromThisCall))


	if err != nil {
		return
	}

	if result != nil {
		for _, resPath := range result.Results {
			select {
			case rawPathChannel <- resPath.Path: 
			case <-doneSignal:
				return
			}
		}
	}
}

func BFSFindXDifferentPathsBackward_ProxyParallel(graph *loadrecipes.BiGraphAlchemy, targetElementName string, maxPaths int) (*pathfinding.MultipleResult, error) {
	if _, targetExists := graph.AllElements[targetElementName]; !targetExists {
		return nil, fmt.Errorf("elemen target '%s' tidak ditemukan (ProxyParallel)", targetElementName)
	}
	if maxPaths <= 0 {
		return nil, fmt.Errorf("maxPaths harus integer positif (ProxyParallel)")
	}

	var collectedPathResults []pathfinding.Result
	uniquePathSignaturesGlobal := make(map[string]bool)
	var totalNodesExploredGlobal int64

	if graph.BaseElements[targetElementName] {
		log.Printf("[BFS-PROXY-ORCH] Target '%s' adalah elemen dasar.", targetElementName)
		return &pathfinding.MultipleResult{
			Results: []pathfinding.Result{{Path: []pathfinding.PathStep{}, NodesVisited: 1}},
		}, nil
	}

	initialParentPairs, hasInitialRecipes := graph.ChildToParents[targetElementName]
	if !hasInitialRecipes {
		log.Printf("[BFS-PROXY-ORCH] Target '%s' tidak memiliki resep awal.", targetElementName)
		return &pathfinding.MultipleResult{Results: []pathfinding.Result{}}, nil 
	}

	var wg sync.WaitGroup
	// Setiap worker memanggil BFSFindXDifferentPathsBackward yang bisa menghasilkan hingga `maxPaths` jalur.
	// Jadi, buffer channel bisa len(initialParentPairs) * maxPaths.
	rawPathChannel := make(chan []pathfinding.PathStep, len(initialParentPairs)*maxPaths) 
	doneSignal := make(chan struct{})

	log.Printf("[BFS-PROXY-ORCH] Target: '%s'. Meluncurkan %d worker (Proxy ke BFS Sekuensial). MaxPaths Global: %d", targetElementName, len(initialParentPairs), maxPaths)

	numWorkersLaunched := 0
	
	maxPathsForWorkerExecution := maxPaths 

	for _, initialRecipe := range initialParentPairs {
		if len(collectedPathResults) >= maxPaths {
			break
		}
		numWorkersLaunched++
		wg.Add(1)
		go proxyBFSWorker(
			graph, 
			targetElementName,
			initialRecipe,
			maxPathsForWorkerExecution, 
			rawPathChannel,
			&wg,
			doneSignal,
			&totalNodesExploredGlobal,
		)
	}
	
	if numWorkersLaunched == 0 && len(initialParentPairs) > 0 {
        log.Printf("[BFS-PROXY-ORCH] Tidak ada worker yang diluncurkan untuk target '%s'.", targetElementName)
    }

	collectorWg := sync.WaitGroup{}
	collectorWg.Add(1)
	go func() {
		defer collectorWg.Done()
		wg.Wait() 
		close(rawPathChannel)
	}()

	for pathFromWorker := range rawPathChannel {
		if len(collectedPathResults) >= maxPaths {
			select {
			case <-doneSignal:
			default:
				close(doneSignal)
			}
			continue 
		}

		sig := createPathSignature(pathFromWorker)
		if !uniquePathSignaturesGlobal[sig] {
			uniquePathSignaturesGlobal[sig] = true
			collectedPathResults = append(collectedPathResults, pathfinding.Result{Path: pathFromWorker, NodesVisited: 0}) 
			if len(collectedPathResults) >= maxPaths {
				select {
				case <-doneSignal:
				default:
					close(doneSignal)
				}
			}
		}
	}

	collectorWg.Wait()
	select {
	case <-doneSignal:
	default:
		close(doneSignal)
	}

	finalNodesExploredCount := int(atomic.LoadInt64(&totalNodesExploredGlobal))
	
	if len(collectedPathResults) == 0 && !graph.BaseElements[targetElementName] {
		if finalNodesExploredCount == 0 && numWorkersLaunched > 0 {
			// Ini berarti semua worker mengembalikan 0 NodesExplored, yang mungkin terjadi jika semua cabang buntu sangat awal.
			// finalNodesExploredCount = 1 
		}
		log.Printf("[BFS-PROXY-ORCH-INFO] Tidak ada jalur unik yang ditemukan untuk '%s'. Total node dieksplorasi (gabungan worker): %d", targetElementName, finalNodesExploredCount)
	} else if len(collectedPathResults) > 0 {
		log.Printf("[BFS-PROXY-ORCH-INFO] Selesai untuk target '%s'. Ditemukan %d jalur unik. Total node dieksplorasi (gabungan worker): %d", targetElementName, len(collectedPathResults), finalNodesExploredCount)
	}


	for i := range collectedPathResults {
		collectedPathResults[i].NodesVisited = finalNodesExploredCount
	}

	sort.SliceStable(collectedPathResults, func(i, j int) bool {
		return len(collectedPathResults[i].Path) < len(collectedPathResults[j].Path)
	})
	
	return &pathfinding.MultipleResult{
		Results: collectedPathResults,
	}, nil
}