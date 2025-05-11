// File: scrape/scrapper.go
package scrape

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/andybalholm/brotli"
)

type Element struct {
	Name    string     `json:"name"`
	Recipes [][]string `json:"recipes"`
	Tier    int        `json:"tier"`
}

// memfilter resep berdasarkan keberadaan parent di validElements
func filterRecipesWithValidParents(recipes [][]string, validElements map[string]bool) [][]string {
	newRecipes := make([][]string, 0, len(recipes))
	for _, recipe := range recipes {
		if len(recipe) == 2 {
			parent1 := recipe[0]
			parent2 := recipe[1]
			if validElements[parent1] && validElements[parent2] {
				newRecipes = append(newRecipes, recipe)
			}
		}
	}
	return newRecipes
}

// memfilter resep berdasarkan aturan tier
func filterRecipesByTier(recipes [][]string, childName string, elementTiers map[string]int) [][]string {
	childTier, ok := elementTiers[childName]
	if !ok { // Seharusnya tidak terjadi jika map tier lengkap
		return [][]string{}
	}

	newRecipes := make([][]string, 0, len(recipes))
	for _, recipe := range recipes {
		if len(recipe) == 2 {
			parent1 := recipe[0]
			parent2 := recipe[1]
			parent1Tier, p1Ok := elementTiers[parent1]
			parent2Tier, p2Ok := elementTiers[parent2]

			if p1Ok && p2Ok && parent1Tier < childTier && parent2Tier < childTier {
				newRecipes = append(newRecipes, recipe)
			}
		}
	}
	return newRecipes
}

func Scrapping() {
	url := "https://little-alchemy.fandom.com/wiki/Elements_(Little_Alchemy_2)"
	fmt.Println("Memulai scraping dari:", url)

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", url, nil); if err != nil { log.Fatalf("FATAL: Error request: %s", err) }
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/108.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,id;q=0.8"); req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("DNT", "1"); req.Header.Set("Upgrade-Insecure-Requests", "1")
	res, err := client.Do(req); if err != nil { log.Fatalf("FATAL: Error GET: %s", err) }; defer res.Body.Close()
	if res.StatusCode != 200 { log.Printf("WARNING: Status %d", res.StatusCode) }
	var reader io.ReadCloser = res.Body; encoding := res.Header.Get("Content-Encoding")
	switch strings.ToLower(encoding) {
	case "br": brotliReader := brotli.NewReader(res.Body); reader = io.NopCloser(brotliReader)
	case "gzip": gzipReader, err := gzip.NewReader(res.Body); if err != nil { log.Fatalf("FATAL: gzip reader: %s", err) }; reader = gzipReader
	}; defer reader.Close()
	bodyBytes, err := io.ReadAll(reader); if err != nil { log.Fatalf("FATAL: read body: %s", err) }
	fmt.Printf("Body response berhasil dibaca (%d bytes).\n", len(bodyBytes))

	fmt.Println("Memulai parsing HTML dengan goquery...")
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(bodyBytes))
	if err != nil { log.Fatalf("FATAL: Error parsing HTML: %s", err) }
	fmt.Println("Parsing HTML selesai.")

	// --- Tahap 1: Scrape Semua Data Awal (Termasuk Tier) ---
	var initialScrapedElements []Element
	baseElements := map[string]bool{"Air": true, "Earth": true, "Fire": true, "Water": true}

	fmt.Println("Mencari dan memproses tabel elemen (scrape awal)...")
	tables := doc.Find("table.list-table")
	tables.Each(func(tableIndex int, table *goquery.Selection) {
		currentTier := tableIndex + 1
		tbodies := table.Find("tbody")
		tbodies.Each(func(tbodyIndex int, tbody *goquery.Selection) {
			rows := tbody.Find("tr")
			rows.Each(func(rowIndex int, row *goquery.Selection) {
				elementLink := row.Find("td:nth-child(1) a[title]").First()
				elementName, nameExists := elementLink.Attr("title")
				elementName = strings.TrimSpace(elementName)
				if nameExists && elementName != "" {
					currentElement := Element{ Name: elementName, Recipes: [][]string{}, Tier: currentTier }
					recipeCell := row.Find("td:nth-child(2)")
					if recipeCell.Length() > 0 {
						lis := recipeCell.Find("li")
						lis.Each(func(liIndex int, li *goquery.Selection) {
							recipeParentLinks := li.Find("a[title]")
							var singleRecipePair []string
							recipeParentLinks.Each(func(j int, link *goquery.Selection) {
								parentName, _ := link.Attr("title"); parentName = strings.TrimSpace(parentName)
								if parentName != "" { singleRecipePair = append(singleRecipePair, parentName) }
							})
							if len(singleRecipePair) == 2 && singleRecipePair[0] != "" && singleRecipePair[1] != "" {
								currentElement.Recipes = append(currentElement.Recipes, singleRecipePair)
							}
						})
					}
					initialScrapedElements = append(initialScrapedElements, currentElement)
				}
			})
		})
	})
	fmt.Printf("Scraping awal selesai. Ditemukan %d entri elemen.\n", len(initialScrapedElements))

	// --- Tahap 2: Buat Map Elemen Unik dan Map Tier ---
	log.Println("Memproses elemen unik dan membuat map tier...")
	elementsMap := make(map[string]Element) // Name -> Element struct
	elementTiers := make(map[string]int)    // Name -> Tier

	for _, elem := range initialScrapedElements {
		if existingElem, exists := elementsMap[elem.Name]; !exists {
			elementsMap[elem.Name] = elem
			elementTiers[elem.Name] = elem.Tier
		} else {
			// Jika elemen sudah ada, gabungkan resep (hindari duplikasi resep)
			// Dan mungkin update tier (misal ambil tier terendah jika muncul di beberapa tabel)
			if elem.Tier < existingElem.Tier { // Ambil tier terendah jika muncul di tabel lebih awal
				existingElem.Tier = elem.Tier
				elementTiers[elem.Name] = elem.Tier
			}
			// Gabungkan resep, hindari duplikat resep untuk elemen yang sama
			for _, newRecipe := range elem.Recipes {
				isDuplicateRecipe := false
				for _, existingRecipe := range existingElem.Recipes {
					if (existingRecipe[0] == newRecipe[0] && existingRecipe[1] == newRecipe[1]) ||
					   (existingRecipe[0] == newRecipe[1] && existingRecipe[1] == newRecipe[0]) {
						isDuplicateRecipe = true
						break
					}
				}
				if !isDuplicateRecipe {
					existingElem.Recipes = append(existingElem.Recipes, newRecipe)
				}
			}
			elementsMap[elem.Name] = existingElem // Update elemen di map
		}
	}
	log.Printf("Ditemukan %d elemen unik. Map tier dibuat.", len(elementsMap))


	// --- Tahap 3: Filter Resep Berdasarkan Tier (Fungsi 1) ---
	log.Println("Filter Tahap Awal: Menerapkan filter tier (parent < child)...")
	recipesRemovedByTierFilter := 0
	for name, elem := range elementsMap {
		originalRecipeCount := len(elem.Recipes)
		elem.Recipes = filterRecipesByTier(elem.Recipes, name, elementTiers)
		recipesRemovedByTierFilter += (originalRecipeCount - len(elem.Recipes))
		elementsMap[name] = elem // Update elemen di map
	}
	log.Printf("%d resep dihapus oleh filter tier awal.", recipesRemovedByTierFilter)


	// --- Tahap 4: Loop Iteratif untuk Filter (Fungsi 2 & 3) ---
	log.Println("Memulai loop filter iteratif...")
	iteration := 0
	for {
		iteration++
		log.Printf("--- Iterasi Filter ke-%d ---", iteration)
		currentElementCount := len(elementsMap)
		
		// Buat daftar elemen yang valid saat ini untuk Fungsi 3
		validElementNames := make(map[string]bool)
		for name := range elementsMap {
			validElementNames[name] = true
		}

		// Fungsi 3: Hapus resep yang bahannya tidak ada (sudah dihapus di iterasi sebelumnya atau dari awal)
		recipesRemovedInvalidParent := 0
		tempElementMapForRecipeFilter := make(map[string]Element)
		for name, elem := range elementsMap {
			originalRecipeCount := len(elem.Recipes)
			elem.Recipes = filterRecipesWithValidParents(elem.Recipes, validElementNames)
			recipesRemovedInvalidParent += (originalRecipeCount - len(elem.Recipes))
			tempElementMapForRecipeFilter[name] = elem
		}
		elementsMap = tempElementMapForRecipeFilter // Update map utama
		if recipesRemovedInvalidParent > 0 {
			log.Printf("  Iterasi %d - Fungsi 3: %d resep dihapus (parent tidak valid).", iteration, recipesRemovedInvalidParent)
		}


		// Fungsi 2: Hapus elemen yang tidak punya resep (dan bukan elemen dasar)
		elementsRemovedNoRecipe := 0
		tempElementMapForElementFilter := make(map[string]Element)
		for name, elem := range elementsMap {
			if baseElements[name] || len(elem.Recipes) > 0 {
				tempElementMapForElementFilter[name] = elem
			} else {
				elementsRemovedNoRecipe++
			}
		}
		elementsMap = tempElementMapForElementFilter // Update map utama
		if elementsRemovedNoRecipe > 0 {
			log.Printf("  Iterasi %d - Fungsi 2: %d elemen (non-base) dihapus (tanpa resep).", iteration, elementsRemovedNoRecipe)
		}


		// Cek kondisi berhenti
		if len(elementsMap) == currentElementCount && recipesRemovedInvalidParent == 0 && elementsRemovedNoRecipe == 0 {
			log.Printf("Loop filter stabil setelah %d iterasi.", iteration)
			break
		}
		if iteration > 10 { // Pengaman untuk menghindari infinite loop jika ada bug
			log.Println("PERINGATAN: Loop filter mencapai batas iterasi maksimum (10). Menghentikan.")
			break
		}
	}

	// Konversi map kembali ke slice untuk output
	finalFilteredElements := make([]Element, 0, len(elementsMap))
	for _, elem := range elementsMap {
		finalFilteredElements = append(finalFilteredElements, elem)
	}
	// (Opsional: Urutkan finalFilteredElements berdasarkan tier atau nama jika perlu)


	// --- Simpan Hasil Akhir ke JSON ---
	if len(finalFilteredElements) > 0 {
		fmt.Println("Memulai proses marshaling data yang sudah difilter ke JSON...")
		jsonData, err := json.MarshalIndent(finalFilteredElements, "", "  ")
		if err != nil { log.Fatalf("FATAL: Error marshaling JSON: %s", err) }
		fmt.Println("Marshal JSON berhasil.")
		outputFileName := "elements_filtered.json"
		fmt.Printf("Menulis data JSON ke file '%s'...\n", outputFileName)
		err = os.WriteFile(outputFileName, jsonData, 0644)
		if err != nil { log.Fatalf("FATAL: Error menulis file JSON: %s", err) }
		fmt.Printf("Berhasil menulis data ke '%s'. Jumlah elemen: %d\n", outputFileName, len(finalFilteredElements))
	} else {
		fmt.Println("Tidak ada elemen yang tersisa setelah filter, file JSON tidak dibuat.")
	}
	fmt.Println("\nScraping dan Filtering Selesai.")
}