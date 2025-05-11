package loadrecipes

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"sort" // Diperlukan untuk memastikan urutan child konsisten jika diperlukan
)

type ElementInput struct {
	Name    string     `json:"name"`
	Recipes [][]string `json:"recipes"`
}

// Contains pair of materials (for recipe)
type PairMats struct {
	Mat1, Mat2 string
}

// Make sorted PairMats(mat1 first alphabet lower than mat2)
func ConstructPair(mat1, mat2 string) PairMats {
	if mat1 < mat2 {
		return PairMats{Mat1: mat1, Mat2: mat2}
	}
	return PairMats{Mat1: mat2, Mat2: mat1}
}

// Helper function to check if a slice contains a string
func ContainsString(slice []string, str string) bool {
	for _, item := range slice {
		if item == str {
			return true
		}
	}
	return false
}

// Bidirectional Graph (Flexible for BFS and DFS)
type BiGraphAlchemy struct {
	ChildToParents    map[string][]PairMats
	ParentPairToChild map[PairMats][]string // slice of string (ingredients can make more than 1 element(child))
	BaseElements      map[string]bool
	AllElements       map[string]bool
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

	// Unmarshal the JSON data into a slice of ElementInput
	var elements []ElementInput
	err = json.Unmarshal(data, &elements)
	if err != nil {
		return nil, err
	}

	// Create a new BiGraphAlchemy instance
	graphData := &BiGraphAlchemy{
		ChildToParents:    make(map[string][]PairMats),
		ParentPairToChild: make(map[PairMats][]string), // Diinisialisasi sebagai map ke slice string
		BaseElements: map[string]bool{
			"Air":   true,
			"Fire":  true,
			"Earth": true,
			"Water": true,
		},
		AllElements: make(map[string]bool),
	}

	// Tambahkan elemen dasar ke AllElements juga
	for baseElem := range graphData.BaseElements {
		graphData.AllElements[baseElem] = true
	}

	// Iterate over the elements and populate the graph data
	for _, element := range elements {
		graphData.AllElements[element.Name] = true

		if graphData.BaseElements[element.Name] {
			continue
		}

		validRecipesForChild := []PairMats{}
		for _, recipe := range element.Recipes {
			if len(recipe) != 2 {
				log.Printf("[WARNING] Invalid recipe format for element %s: %v. Skipping recipe.", element.Name, recipe)
				continue
			}

			parent1, parent2 := recipe[0], recipe[1]
			graphData.AllElements[parent1] = true
			graphData.AllElements[parent2] = true

			pair := ConstructPair(parent1, parent2)

			// Forward direction: Parents -> Children (plural)
			// Append child ke slice yang sudah ada, atau buat slice baru jika belum ada.
			// Pastikan tidak ada duplikasi child untuk pair yang sama.
			if !ContainsString(graphData.ParentPairToChild[pair], element.Name) {
				graphData.ParentPairToChild[pair] = append(graphData.ParentPairToChild[pair], element.Name)
			}

			// Backward direction: Child -> Parents
			// Pastikan tidak ada duplikasi pair untuk child yang sama (umumnya tidak masalah karena resep di JSON biasanya unik per child)
			alreadyExists := false
			for _, existingPair := range validRecipesForChild {
				if existingPair == pair {
					alreadyExists = true
					break
				}
			}
			if !alreadyExists {
				validRecipesForChild = append(validRecipesForChild, pair)
			}
		}

		if len(validRecipesForChild) > 0 {
			graphData.ChildToParents[element.Name] = validRecipesForChild
		} else if !graphData.BaseElements[element.Name] {
			log.Printf("[INFO] Element %s (non-base) has no valid recipes after loading.", element.Name)
		}
	}

	// (Opsional) Urutkan slice of children di ParentPairToChild untuk konsistensi (berguna untuk testing)
	for pair := range graphData.ParentPairToChild {
		sort.Strings(graphData.ParentPairToChild[pair])
	}

	log.Printf("[INFO] Data successfully loaded from '%s'. Total Unique Elements: %d. Unique Parent Pairs: %d. Child-to-Parent Relations: %d.\n",
		filepath, len(graphData.AllElements), len(graphData.ParentPairToChild), len(graphData.ChildToParents))

	return graphData, nil
}