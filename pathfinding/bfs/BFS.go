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