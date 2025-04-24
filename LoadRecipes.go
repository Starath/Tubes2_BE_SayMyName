package tubes2besaymyname

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
)

// Contains pair of materials (for recipe)
type PairMats struct {
	Mat1, Mat2 string
}

// Make sorted PairMats(mat1 first alphabet lower than mat2)
func constructPair(mat1, mat2 string) PairMats {
	if mat1 < mat2 {	
		return PairMats{Mat1: mat1, Mat2: mat2}
	}
	return PairMats{Mat1: mat2, Mat2: mat1}
}

// Bidirectional Graph (Flexible for BFS and DFS)
type BiGraphAlchemy struct {
	ChildToParents 			map[string][]PairMats
	ParentPairToChild		map[PairMats]string
	BaseElements	  		map[string]bool
	AllElements					map[string]bool	
}


func LoadBiGraph(filepath string) (*BiGraphAlchemy, error) {
	// Read the JSON file
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Get the byte codes
	data, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	// Unmarshal the JSON data into a map
	var elements []Element
	err = json.Unmarshal(data, &elements)
	if err != nil {
		return nil, err
	}

	// Create a new BiGraphAlchemy instance
	graphData := &BiGraphAlchemy{
		ChildToParents: 		make(map[string][]PairMats),
		ParentPairToChild: 	make(map[PairMats]string),
		BaseElements: 			map[string]bool{
													"Air": true,
													"Fire": true,
													"Earth": true,
													"Water": true,
												},
		AllElements: 				make(map[string]bool),
	}

	// Iterate over the elements and populate the graph data
	for _, element := range elements {
		if graphData.BaseElements[element.Name] { continue }
		graphData.AllElements[element.Name] = true
		validRecipesForChild := []PairMats{}
		for _, recipe := range element.Recipes {
			if len(recipe) != 2 { // Invalid recipe format
				log.Printf("[WARNING] Invalid recipe for element %s: %v", element.Name, recipe)
				continue
			}

			// Construct recipe
			parent1, parent2 := recipe[0], recipe[1]
			graphData.AllElements[parent1] = true
			graphData.AllElements[parent2] = true
			
			pair := constructPair(parent1, parent2)

			// Forward direction: Parents -> Child
			graphData.ParentPairToChild[pair] = element.Name
			// Backward direction: Child -> Parents
			validRecipesForChild = append(validRecipesForChild, pair)
		}
		// Only add to ChildToParents if there are valid recipes (Just in case)
		if len(validRecipesForChild) > 0 {
			graphData.ChildToParents[element.Name] = validRecipesForChild
		}
	}
  log.Printf("[INFO] Data succesfully loaded. Total Elements: %d. Forward Relations: %d. Backward Relations: %d.\n",
    len(graphData.AllElements), len(graphData.ParentPairToChild), len(graphData.ChildToParents))

	return graphData, nil
}