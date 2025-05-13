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


type workerResult struct {
	path         []pathfinding.PathStep
	nodesVisited int
	err          error
}

type workerConfig struct {
	targetElementName string
	graph             *loadrecipes.BiGraphAlchemy
	workerID          int
	maxRecipesGlobal  int
	randomSeed        int64
	explorationDepth  int  
}

func DFSFindMultiplePathsString(graph *loadrecipes.BiGraphAlchemy, targetElementName string, maxRecipes int) (*pathfinding.MultipleResult, int, error) {
	if _, targetExists := graph.AllElements[targetElementName]; !targetExists {
		return nil, 0, fmt.Errorf("elemen target '%s' tidak ditemukan dalam data", targetElementName)
	}
	if maxRecipes <= 0 {
		return nil, 0, fmt.Errorf("parameter maxRecipes harus positif")
	}
	if graph.BaseElements[targetElementName] { // Jika target adalah elemen dasar
		return &pathfinding.MultipleResult{Results: []pathfinding.Result{{Path: []pathfinding.PathStep{}, NodesVisited: 1}}}, 1, nil
	}

	initialRecipesForTarget, hasRecipes := graph.ChildToParents[targetElementName]
	if !hasRecipes {
		return nil, 0, fmt.Errorf("elemen target '%s' tidak memiliki resep", targetElementName)
	}

	var wg sync.WaitGroup
	
	numInitialRecipes := len(initialRecipesForTarget)
	numWorkers := numInitialRecipes
	
	if maxRecipes > numInitialRecipes {
		numAdditionalWorkers := maxRecipes - numInitialRecipes
		maxAdditionalWorkers := numInitialRecipes * 3
		if numAdditionalWorkers > maxAdditionalWorkers {
			numAdditionalWorkers = maxAdditionalWorkers
		}
		numWorkers += numAdditionalWorkers
	}
	
	resultsProcessingChan := make(chan workerResult, numWorkers)
	
	// Memoization bersama: menyimpan status apakah suatu elemen *secara umum* dapat dibuat.
	sharedOverallCanBeMadeMemo := make(map[string]bool)
	var sharedMemoMutex sync.Mutex
	
	var pathsFoundCounter int32 // Counter atomik untuk jumlah jalur unik yang sudah ditemukan
	var totalNodesVisitedByWorkers int64 // Counter atomik untuk total node yang dikunjungi
	var pathsFoundCounter int32
	var totalNodesVisitedByWorkers int64

	doneChan := make(chan struct{})

	workerCount := 0
	
	for _, initialRecipe := range initialRecipesForTarget {
		if atomic.LoadInt32(&pathsFoundCounter) >= int32(maxRecipes) {
			break
		}

		wg.Add(1)
		workerCount++
		
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
			0,
			time.Now().UnixNano() + int64(workerCount),
		)
	}
	
	for i := workerCount; i < numWorkers; i++ {
		if atomic.LoadInt32(&pathsFoundCounter) >= int32(maxRecipes) {
			break
		}

		wg.Add(1)
		workerCount++
		
		explorationDepth := 1 + (i % 5)
		
		var initialRecipe loadrecipes.PairMats
		if i < len(initialRecipesForTarget)*2 { 
			recipeIndex := i % len(initialRecipesForTarget)
			initialRecipe = initialRecipesForTarget[recipeIndex]
		} else {
			recipeIndex := i % len(initialRecipesForTarget)
			initialRecipe = initialRecipesForTarget[recipeIndex]
		}
		
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

	go func() {
		wg.Wait()
		close(resultsProcessingChan)
	}()

	var collectedUniquePathResults []pathfinding.Result
	pathSignatures := make(map[string]bool) 
	var accumulatedNodesForUniquePaths int

	for workerRes := range resultsProcessingChan {
		if workerRes.err != nil {
			log.Printf("Error dari worker untuk target %s: %v", targetElementName, workerRes.err)
			continue
		}

		if len(workerRes.path) > 0 {
			pathSignature := generatePathSignature(workerRes.path)
			
			if !pathSignatures[pathSignature] {
				pathSignatures[pathSignature] = true
				collectedUniquePathResults = append(collectedUniquePathResults, pathfinding.Result{
					Path:         workerRes.path,
					NodesVisited: workerRes.nodesVisited,
				})
				accumulatedNodesForUniquePaths += workerRes.nodesVisited

				if len(collectedUniquePathResults) >= maxRecipes {
					select {
					case <-doneChan:
					default:
						close(doneChan)
					}
					
					go func() {
						for range resultsProcessingChan {
						}
					}()
					break
				}
			}
		}
	}
	
	if len(collectedUniquePathResults) == 0 && !graph.BaseElements[targetElementName] {
		return &pathfinding.MultipleResult{Results: []pathfinding.Result{}}, accumulatedNodesForUniquePaths, fmt.Errorf("tidak ada jalur resep unik yang ditemukan untuk elemen '%s' setelah semua worker selesai", targetElementName)
	}

	return &pathfinding.MultipleResult{Results: collectedUniquePathResults}, accumulatedNodesForUniquePaths, nil
}

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
	explorationDepth int,
	randomSeed int64,    
) {
	defer wg.Done()

	localRNG := rand.New(rand.NewSource(randomSeed))
	
	pathStepsForThisWorker := make(map[string]pathfinding.PathStep)
	currentlySolvingForThisWorker := make(map[string]bool)
	memoForThisWorkerBranch := make(map[string]bool)
	
	var nodesVisitedByThisWorker int

	select {
	case <-doneChan:
		return
	default:
		if atomic.LoadInt32(pathsFoundGlobalCounter) >= int32(maxRecipesGlobalLimit) {
			return
		}
	}

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
		explorationDepth, 
		localRNG,         
		0,                
	)

	if !canMakeP1 {
		resultsChan <- workerResult{path: nil, nodesVisited: nodesVisitedByThisWorker, err: nil}
		return
	}

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

	pathStepsForThisWorker[targetElementName] = pathfinding.PathStep{
		ChildName:   targetElementName,
		Parent1Name: initialRecipeForTargetElement.Mat1,
		Parent2Name: initialRecipeForTargetElement.Mat2,
	}
	
	reconstructedPath := reconstructFullPathFromSteps(pathStepsForThisWorker, targetElementName, graph.BaseElements)
	
	resultsChan <- workerResult{path: reconstructedPath, nodesVisited: nodesVisitedByThisWorker, err: nil}
	atomic.AddInt32(pathsFoundGlobalCounter, 1)
}

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
	explorationDepth int,
	localRNG *rand.Rand, 
	currentDepth int,    
) bool {
	select {
	case <-doneChan:
		return false
	default:
		if atomic.LoadInt32(pathsFoundGlobalCounter) >= int32(maxRecipesGlobalLimit) {
			return false
		}
	}

	if canBeMade, exists := memoForThisWorkerBranch[elementName]; exists {
		return canBeMade
	}

	if graph.BaseElements[elementName] {
		(*nodesVisitedCounter)++
		memoForThisWorkerBranch[elementName] = true
		return true
	}
	
	if currentlySolvingThisBranch[elementName] {
		return false
	}
	currentlySolvingThisBranch[elementName] = true
	defer delete(currentlySolvingThisBranch, elementName)

	sharedMemoMutex.Lock()
	if knownGlobalStatus, exists := sharedOverallCanBeMadeMemo[elementName]; exists && !knownGlobalStatus {
		sharedMemoMutex.Unlock()
		memoForThisWorkerBranch[elementName] = false
		return false
	}
	sharedMemoMutex.Unlock()
	
	(*nodesVisitedCounter)++

	recipesForCurrentElement, hasRecipes := graph.ChildToParents[elementName]
	if !hasRecipes {
		sharedMemoMutex.Lock()
		sharedOverallCanBeMadeMemo[elementName] = false
		sharedMemoMutex.Unlock()
		memoForThisWorkerBranch[elementName] = false
		return false
	}

	shuffledRecipeIndices := make([]int, len(recipesForCurrentElement))
	for i := range shuffledRecipeIndices {
		shuffledRecipeIndices[i] = i
	}
	
	shuffleProbability := float64(explorationDepth) * 0.2
	if currentDepth <= explorationDepth && localRNG.Float64() < shuffleProbability {
		for i := len(shuffledRecipeIndices) - 1; i > 0; i-- {
			j := localRNG.Intn(i + 1)
			shuffledRecipeIndices[i], shuffledRecipeIndices[j] = shuffledRecipeIndices[j], shuffledRecipeIndices[i]
		}
	}

	for _, index := range shuffledRecipeIndices {
		recipePair := recipesForCurrentElement[index]
		
		parent1, parent2 := recipePair.Mat1, recipePair.Mat2
		if explorationDepth > 0 && currentDepth <= explorationDepth && localRNG.Float64() < 0.3 {
			parent1, parent2 = parent2, parent1
		}
		
		canMakeP1 := dfsRecursiveHelperForWorkerPathEnhanced(
			parent1, graph, pathStepsThisBranch, currentlySolvingThisBranch, memoForThisWorkerBranch,
			sharedOverallCanBeMadeMemo, sharedMemoMutex, nodesVisitedCounter, doneChan, 
			pathsFoundGlobalCounter, maxRecipesGlobalLimit, explorationDepth, localRNG, currentDepth+1)
		if !canMakeP1 {
			continue
		}

		canMakeP2 := dfsRecursiveHelperForWorkerPathEnhanced(
			parent2, graph, pathStepsThisBranch, currentlySolvingThisBranch, memoForThisWorkerBranch,
			sharedOverallCanBeMadeMemo, sharedMemoMutex, nodesVisitedCounter, doneChan, 
			pathsFoundGlobalCounter, maxRecipesGlobalLimit, explorationDepth, localRNG, currentDepth+1)
		if !canMakeP2 {
			continue
		}

		pathStepsThisBranch[elementName] = pathfinding.PathStep{ChildName: elementName, Parent1Name: parent1, Parent2Name: parent2}
		memoForThisWorkerBranch[elementName] = true
		return true
	}

	memoForThisWorkerBranch[elementName] = false
	return false
}

func generatePathSignature(path []pathfinding.PathStep) string {
	if len(path) == 0 {
		return "base_element_or_empty_path"
	}

	pathCopy := make([]pathfinding.PathStep, len(path))
	copy(pathCopy, path)

	sort.Slice(pathCopy, func(i, j int) bool {
		if pathCopy[i].ChildName != pathCopy[j].ChildName {
			return pathCopy[i].ChildName < pathCopy[j].ChildName
		}
		p1_i, p2_i := pathCopy[i].Parent1Name, pathCopy[i].Parent2Name
		if p1_i > p2_i {p1_i, p2_i = p2_i, p1_i}

		p1_j, p2_j := pathCopy[j].Parent1Name, pathCopy[j].Parent2Name
		if p1_j > p2_j {p1_j, p2_j = p2_j, p1_j}
		
		if p1_i != p1_j {
			return p1_i < p1_j
		}
		return p2_i < p2_j
	})

	var signatureParts []string
	for _, step := range pathCopy {
		parent1, parent2 := step.Parent1Name, step.Parent2Name
		if parent1 > parent2 {parent1, parent2 = parent2, parent1} 
		signatureParts = append(signatureParts, fmt.Sprintf("%s=(%s+%s)", step.ChildName, parent1, parent2))
	}
	return strings.Join(signatureParts, ";")
}