package main

import (
	"container/list"
	"fmt"
	"log"

	"github.com/Starath/Tubes2_BE_SayMyName/loadrecipes"
	"github.com/Starath/Tubes2_BE_SayMyName/pathfinding"
)


// iddfsFindPath implements Iterative Deepening DFS to find the shortest path to a target element
func iddfsFindPath(graph *loadrecipes.BiGraphAlchemy, targetElementName string) (*pathfinding.DFSResult, error) {
	if _, targetExists := graph.AllElements[targetElementName]; !targetExists {
		return nil, fmt.Errorf("target element '%s' not found", targetElementName)
	}

	// Base element case
	if graph.BaseElements[targetElementName] {
		return &pathfinding.DFSResult{Path: []pathfinding.PathStep{}, NodesVisited: 1}, nil
	}

	totalNodesVisited := 0
	maxDepth := 1

	// Max possible depth in Little Alchemy 2 (adjust if needed)
	// The number of combinations required to get most complex elements shouldn't exceed 20-30 steps
	maxPossibleDepth := 30

	for maxDepth <= maxPossibleDepth {
		// Reset for each depth iteration
		pathSteps := make(map[string]pathfinding.PathStep)
		currentlySolving := make(map[string]bool)
		memo := make(map[string]bool)
		depthMemo := make(map[string]int) // Track minimum depth at which element was found
		nodesVisitedThisDepth := 0

		// Attempt DFS with current depth limit
		found := iddfsRecursiveHelper(
			targetElementName,
			graph,
			pathSteps,
			currentlySolving,
			memo,
			depthMemo,
			&nodesVisitedThisDepth,
			0,
			maxDepth,
		)

		totalNodesVisited += nodesVisitedThisDepth

		if found {
			// Reconstruct path from pathSteps
			finalPath := reconstructPath(targetElementName, pathSteps, graph)
			return &pathfinding.DFSResult{Path: finalPath, NodesVisited: totalNodesVisited}, nil
		}

		// If all nodes were explored within this depth but target wasn't found,
		// it means there's no path to the target
		if nodesVisitedThisDepth == 0 {
			break
		}

		// Increase depth for next iteration
		maxDepth++
	}

	return nil, fmt.Errorf("no recipe path found for element '%s' within reasonable depth (nodes explored: %d)",
		targetElementName, totalNodesVisited)
}

// iddfsRecursiveHelper is the core recursive function for IDDFS
func iddfsRecursiveHelper(
	elementName string,
	graph *loadrecipes.BiGraphAlchemy,
	pathSteps map[string]pathfinding.PathStep,
	currentlySolving map[string]bool,
	memo map[string]bool,
	depthMemo map[string]int,
	visitedCounter *int,
	currentDepth int,
	maxDepth int,
) bool {
	// Check if we've exceeded max depth
	if currentDepth > maxDepth {
		return false
	}

	// Check if we've seen this element at a better (lower) depth
	if minDepth, exists := depthMemo[elementName]; exists && minDepth <= currentDepth {
		return memo[elementName]
	}

	// Count unique visits
	*visitedCounter++

	// Check base case (basic element)
	if graph.BaseElements[elementName] {
		memo[elementName] = true
		depthMemo[elementName] = currentDepth
		return true
	}

	// Check for cycles in current path
	if currentlySolving[elementName] {
		return false
	}

	// Mark as being solved
	currentlySolving[elementName] = true
	defer delete(currentlySolving, elementName)

	// Check if element has recipes
	parentPairs, hasRecipes := graph.ChildToParents[elementName]
	if !hasRecipes {
		memo[elementName] = false
		depthMemo[elementName] = currentDepth
		return false
	}

	// Try each recipe
	foundPath := false
	for _, pair := range parentPairs {
		// Try to solve parent 1
		canMakeP1 := iddfsRecursiveHelper(
			pair.Mat1,
			graph,
			pathSteps,
			currentlySolving,
			memo,
			depthMemo,
			visitedCounter,
			currentDepth+1,
			maxDepth,
		)
		if !canMakeP1 {
			continue
		}

		// Try to solve parent 2
		canMakeP2 := iddfsRecursiveHelper(
			pair.Mat2,
			graph,
			pathSteps,
			currentlySolving,
			memo,
			depthMemo,
			visitedCounter,
			currentDepth+1,
			maxDepth,
		)
		if !canMakeP2 {
			continue
		}

		// If both parents can be made
		if canMakeP1 && canMakeP2 {
			// Save this recipe step
			pathSteps[elementName] = pathfinding.PathStep{
				ChildName:   elementName,
				Parent1Name: pair.Mat1,
				Parent2Name: pair.Mat2,
			}
			foundPath = true
			break // We just need one valid path at the current depth
		}
	}

	// Store result in memo
	memo[elementName] = foundPath
	depthMemo[elementName] = currentDepth
	return foundPath
}

// reconstructPath creates the final ordered path from pathSteps
func reconstructPath(targetElementName string, pathSteps map[string]pathfinding.PathStep, graph *loadrecipes.BiGraphAlchemy) []pathfinding.PathStep {
	// Create an ordered list of steps from base elements to target
	finalPath := make([]pathfinding.PathStep, 0)
	elementsToProcess := list.New()
	elementsToProcess.PushBack(targetElementName)
	processed := make(map[string]bool)

	for elementsToProcess.Len() > 0 {
		// Get the next element to process
		elem := elementsToProcess.Front()
		elementsToProcess.Remove(elem)
		currentElement := elem.Value.(string)

		// Skip if already processed or it's a base element
		if processed[currentElement] || graph.BaseElements[currentElement] {
			continue
		}

		// Get the recipe for this element
		step, exists := pathSteps[currentElement]
		if !exists {
			continue
		}

		// Add its parents to processing queue
		if !graph.BaseElements[step.Parent1Name] && !processed[step.Parent1Name] {
			elementsToProcess.PushBack(step.Parent1Name)
		}
		if !graph.BaseElements[step.Parent2Name] && !processed[step.Parent2Name] {
			elementsToProcess.PushBack(step.Parent2Name)
		}

		// Add step to path and mark as processed
		finalPath = append(finalPath, step)
		processed[currentElement] = true
	}

	// Reverse the path to get correct order from base elements to target
	for i, j := 0, len(finalPath)-1; i < j; i, j = i+1, j-1 {
		finalPath[i], finalPath[j] = finalPath[j], finalPath[i]
	}

	return finalPath
}

// topologicalSort orders the path steps from base elements to target
func topologicalSort(pathSteps []pathfinding.PathStep, graph *loadrecipes.BiGraphAlchemy) []pathfinding.PathStep {
	// Create dependency graph
	dependencies := make(map[string][]string)
	elements := make(map[string]bool)
	
	for _, step := range pathSteps {
		dependencies[step.Parent1Name] = append(dependencies[step.Parent1Name], step.ChildName)
		dependencies[step.Parent2Name] = append(dependencies[step.Parent2Name], step.ChildName)
		elements[step.ChildName] = true
		elements[step.Parent1Name] = true
		elements[step.Parent2Name] = true
	}
	
	// Find elements with no dependencies (base elements or missing steps)
	var baseElements []string
	for elem := range elements {
		if graph.BaseElements[elem] || !elementInPath(elem, pathSteps) {
			baseElements = append(baseElements, elem)
		}
	}
	
	// Perform topological sort
	sorted := make([]pathfinding.PathStep, 0, len(pathSteps))
	visited := make(map[string]bool)
	tempMark := make(map[string]bool)
	
	var visit func(string)
	visit = func(node string) {
		if visited[node] {
			return
		}
		if tempMark[node] {
			// Cycle detected, but should not happen in a valid recipe path
			return
		}
		
		tempMark[node] = true
		for _, dep := range dependencies[node] {
			visit(dep)
		}
		tempMark[node] = false
		visited[node] = true
		
		// Add step to sorted list if it exists
		for _, step := range pathSteps {
			if step.ChildName == node {
				sorted = append(sorted, step)
				break
			}
		}
	}
	
	// Start with base elements
	for _, elem := range baseElements {
		visit(elem)
	}
	
	// Check if any elements were not visited
	for _, step := range pathSteps {
		if !visited[step.ChildName] {
			visit(step.ChildName)
		}
	}
	
	return sorted
}

// elementInPath checks if an element appears as a child in any step
func elementInPath(elem string, steps []pathfinding.PathStep) bool {
	for _, step := range steps {
		if step.ChildName == elem {
			return true
		}
	}
	return false
}

// Example usage function
func IDDFSExample() {
	// Load recipe data
	recipeData, err := loadrecipes.LoadBiGraph("elements.json")
	if err != nil {
		log.Fatalf("FATAL: Failed to load recipe data: %v", err)
	}

	fmt.Println("\n--- ITERATIVE DEEPENING DFS (Shortest Path) ---")
	
	// Test with multiple elements
	testElements := []string{"Brick", "Human", "Hedgehog", "Sandpaper", "Computer"}
	
	for _, target := range testElements {
		result, err := iddfsFindPath(recipeData, target)
		if err != nil {
			fmt.Printf("Error finding path for %s: %v\n", target, err)
			continue
		}
		
		fmt.Printf("\nShortest recipe path for %s (Nodes explored: %d):\n", target, result.NodesVisited)
		if len(result.Path) == 0 {
			fmt.Println("- Base element.")
		} else {
			fmt.Printf("- Path length: %d steps\n", len(result.Path))
			for _, step := range result.Path {
				fmt.Printf("  %s = %s + %s\n", step.ChildName, step.Parent1Name, step.Parent2Name)
			}
		}
	}
}

func main() {
	IDDFSExample()
}