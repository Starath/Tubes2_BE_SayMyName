package dfs

import (
	"container/list"
	"fmt"
	"log"

	"github.com/Starath/Tubes2_BE_SayMyName/loadrecipes"
	"github.com/Starath/Tubes2_BE_SayMyName/pathfinding"
)

func dfsRecursiveHelperString(
	elementName string,
	graph *loadrecipes.BiGraphAlchemy,
	pathSteps map[string]pathfinding.PathStep,
	currentlySolving map[string]bool,
	memo map[string]bool,
	visitedCounter *int,
) bool {
	if canBeMade, exists := memo[elementName]; exists {
		return canBeMade
	}
	*visitedCounter++

	if graph.BaseElements[elementName] {
		memo[elementName] = true
		return true
	}

	if currentlySolving[elementName] {
		return false
	}

	currentlySolving[elementName] = true
	defer delete(currentlySolving, elementName)

	parentPairs, hasRecipes := graph.ChildToParents[elementName]
	if !hasRecipes {
		memo[elementName] = false
		return false
	}

	foundPath := false
	for _, pair := range parentPairs {
		canMakeP1 := dfsRecursiveHelperString(pair.Mat1, graph, pathSteps, currentlySolving, memo, visitedCounter)
		if !canMakeP1 {
			continue
		}

		canMakeP2 := dfsRecursiveHelperString(pair.Mat2, graph, pathSteps, currentlySolving, memo, visitedCounter)
		if !canMakeP2 {
			continue
		}

		if canMakeP1 && canMakeP2 {
			pathSteps[elementName] = pathfinding.PathStep{ChildName: elementName, Parent1Name: pair.Mat1, Parent2Name: pair.Mat2}
			foundPath = true
			break
		}
	}

	memo[elementName] = foundPath
	return foundPath
}

func DFSFindPathString(graph *loadrecipes.BiGraphAlchemy, targetElementName string) (*pathfinding.Result, error) {
	if _, targetExists := graph.AllElements[targetElementName]; !targetExists {
		return nil, fmt.Errorf("elemen target '%s' tidak ditemukan", targetElementName)
	}

	if graph.BaseElements[targetElementName] {
		return &pathfinding.Result{Path: []pathfinding.PathStep{}, NodesVisited: 1}, nil
	}

	pathSteps := make(map[string]pathfinding.PathStep)  
	currentlySolving := make(map[string]bool)
	memo := make(map[string]bool)          
	visitedCount := 0                      

	success := dfsRecursiveHelperString(targetElementName, graph, pathSteps, currentlySolving, memo, &visitedCount)

	if success {
		finalPath := reconstructFullPathFromSteps(pathSteps, targetElementName, graph.BaseElements)
		return &pathfinding.Result{Path: finalPath, NodesVisited: visitedCount}, nil
	}

	nodesExploredFinal := visitedCount
	if !memo[targetElementName] {
		
		
		if _, inMemo := memo[targetElementName]; !inMemo && !graph.BaseElements[targetElementName] {
			nodesExploredFinal++
		}
		log.Printf("INFO: Elemen '%s' ditandai tidak dapat dibuat (memo=false) oleh DFSFindPathString.\n", targetElementName)
	}

	return nil, fmt.Errorf("tidak ditemukan jalur resep untuk elemen '%s' menggunakan DFSFindPathString (Nodes Explored: %d)", targetElementName, nodesExploredFinal)
}

func reconstructFullPathFromSteps(
	steps map[string]pathfinding.PathStep,
	targetElementName string,
	baseElements map[string]bool,
) []pathfinding.PathStep {
	var path []pathfinding.PathStep
	queue := list.New() 
	
	if _, exists := steps[targetElementName]; !exists && !baseElements[targetElementName] {
		if targetElementName != "" { 
		}
		return path 
	}

	queue.PushBack(targetElementName)
	processedForThisPathReconstruction := make(map[string]bool) 

	for queue.Len() > 0 {
		currentElementName := queue.Remove(queue.Front()).(string)

		if baseElements[currentElementName] || processedForThisPathReconstruction[currentElementName] {
			continue
		}

		step, exists := steps[currentElementName]
		if !exists {
			continue 
		}
		
		path = append(path, step) 
		processedForThisPathReconstruction[currentElementName] = true 

		if !baseElements[step.Parent1Name] && !processedForThisPathReconstruction[step.Parent1Name] {
			queue.PushBack(step.Parent1Name)
		}
		if !baseElements[step.Parent2Name] && !processedForThisPathReconstruction[step.Parent2Name] {
			queue.PushBack(step.Parent2Name)
		}
	}

	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}

// func main() {
//   // --- (Pastikan fungsi LoadFlexibleRecipes dipanggil sebelum ini) ---
//    recipeData, err := loadrecipes.LoadBiGraph("elements.json")
//    if err != nil {
//      log.Fatalf("FATAL: Gagal memuat data resep: %v", err)
//    }

//    fmt.Println("\n--- DFS SEARCH (Single-Threaded) ---")
//    targetDFS := "Picnic" // Ganti dengan target yg diinginkan
//    resultDFS, errDFS := DFSFindPathString(recipeData, targetDFS)
//    if errDFS != nil {
//      fmt.Printf("Error mencari resep DFS untuk %s: %v\n", targetDFS, errDFS)
//    } else {
//      fmt.Printf("Resep DFS (salah satu jalur) untuk %s (Nodes Explored: %d):\n", targetDFS, resultDFS.NodesVisited)
//      if len(resultDFS.Path) == 0 {
//        fmt.Println("- Elemen dasar.")
//      } else {
//        // Tampilkan path (urutan sudah dibalik)
//        for _, step := range resultDFS.Path {
//          fmt.Printf("  %s = %s + %s\n", step.ChildName, step.Parent1Name, step.Parent2Name)
//        }
//      }
//    }

//    fmt.Println("\n-------------------\n")

//    targetDFS = "Hedgehog" // Contoh lain
//    resultDFS, errDFS = DFSFindPathString(recipeData, targetDFS)
//    if errDFS != nil {
//      fmt.Printf("Error mencari resep DFS untuk %s: %v\n", targetDFS, errDFS)
//    } else {
//      fmt.Printf("Resep DFS (salah satu jalur) untuk %s (Nodes Explored: %d):\n", targetDFS, resultDFS.NodesVisited)
//      if len(resultDFS.Path) == 0 {
//        fmt.Println("- Elemen dasar.")
//      } else {
//        for _, step := range resultDFS.Path {
//          fmt.Printf("  %s = %s + %s\n", step.ChildName, step.Parent1Name, step.Parent2Name)
//        }
//      }
//    }
// }
