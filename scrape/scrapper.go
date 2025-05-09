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
	Tier    int        `json:"tier"` // Field Tier ditambahkan
}

func Scrapping() {
	url := "https://little-alchemy.fandom.com/wiki/Elements_(Little_Alchemy_2)"
	fmt.Println("Memulai scraping dari:", url)

	// --- Bagian HTTP Request dan Pembacaan Body ---
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatalf("FATAL: Error request: %s", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/108.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,id;q=0.8")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("DNT", "1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	res, err := client.Do(req)
	if err != nil {
		log.Fatalf("FATAL: Error GET: %s", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Printf("WARNING: Status %d", res.StatusCode)
	}
	var reader io.ReadCloser = res.Body
	encoding := res.Header.Get("Content-Encoding")
	switch strings.ToLower(encoding) {
	case "br":
		brotliReader := brotli.NewReader(res.Body)
		reader = io.NopCloser(brotliReader)
	case "gzip": // Tambahkan penanganan gzip jika server mengirimkannya
		gzipReader, err := gzip.NewReader(res.Body)
		if err != nil {
			log.Fatalf("FATAL: Error membuat gzip reader: %s", err)
		}
		reader = gzipReader
	}
	defer reader.Close()
	bodyBytes, err := io.ReadAll(reader)
	if err != nil {
		log.Fatalf("FATAL: read body: %s", err)
	}
	fmt.Printf("Body response berhasil dibaca (%d bytes).\n", len(bodyBytes))
	// --- Akhir Bagian HTTP Request ---

	fmt.Println("Memulai parsing HTML dengan goquery...")
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(bodyBytes))
	if err != nil {
		log.Fatalf("FATAL: Error parsing HTML: %s", err)
	}
	fmt.Println("Parsing HTML selesai.")

	// --- Tahap 1: Scrape Semua Data Awal (Termasuk Tier) ---
	var rawElements []Element
	// Definisikan elemen dasar di sini, tier mereka akan diisi dari tabel jika mereka ada di tabel pertama.
	// Jika tidak, kita bisa set tier mereka secara manual setelah scraping atau sebelum membuat elementTiers map.
	// Untuk sekarang, kita biarkan tier mereka ditentukan oleh posisi tabel.
	baseElements := map[string]bool{"Air": true, "Earth": true, "Fire": true, "Water": true}

	fmt.Println("Mencari dan memproses tabel elemen (scrape awal)...")
	tables := doc.Find("table.list-table")
	tables.Each(func(tableIndex int, table *goquery.Selection) {
		currentTier := tableIndex + 1 // Tier dimulai dari 1 (0-indexed + 1)
		tbodies := table.Find("tbody")
		tbodies.Each(func(tbodyIndex int, tbody *goquery.Selection) {
			rows := tbody.Find("tr")
			rows.Each(func(rowIndex int, row *goquery.Selection) {
				elementLink := row.Find("td:nth-child(1) a[title]").First()
				elementName, nameExists := elementLink.Attr("title")
				elementName = strings.TrimSpace(elementName)

				if nameExists && elementName != "" {
					currentElement := Element{
						Name:    elementName,
						Recipes: [][]string{},
						Tier:    currentTier, // Isi field Tier
					}
					recipeCell := row.Find("td:nth-child(2)")
					if recipeCell.Length() > 0 {
						lis := recipeCell.Find("li")
						lis.Each(func(liIndex int, li *goquery.Selection) {
							recipeParentLinks := li.Find("a[title]")
							var singleRecipePair []string
							recipeParentLinks.Each(func(j int, link *goquery.Selection) {
								parentName, _ := link.Attr("title")
								parentName = strings.TrimSpace(parentName)
								if parentName != "" {
									singleRecipePair = append(singleRecipePair, parentName)
								}
							})
							if len(singleRecipePair) == 2 {
								if singleRecipePair[0] != "" && singleRecipePair[1] != "" {
									currentElement.Recipes = append(currentElement.Recipes, singleRecipePair)
								}
							}
						})
					}
					rawElements = append(rawElements, currentElement)
				}
			})
		})
	})
	fmt.Printf("Scraping awal selesai. Ditemukan %d entri elemen.\n", len(rawElements))

	// --- Tahap 2: Proses Unik dan Buat Map Tier ---
	// (Menggabungkan Tahap 1 dan sedikit Tahap 2 dari kode Anda)
	log.Println("Memproses elemen unik dan membuat map tier...")
	elementNamesFound := make(map[string]bool)
	uniqueElements := []Element{}
	elementTiers := make(map[string]int) // Map untuk menyimpan Name -> Tier

	for _, elem := range rawElements {
		if _, exists := elementNamesFound[elem.Name]; !exists {
			elementNamesFound[elem.Name] = true
			uniqueElements = append(uniqueElements, elem)
			elementTiers[elem.Name] = elem.Tier // Isi map tier
		}
		// Jika ada duplikasi, kita bisa mempertimbangkan untuk menggabungkan resep atau tier (misal ambil tier terendah)
		// Untuk sekarang, kita ambil yang pertama muncul.
	}
	rawElements = uniqueElements // Selanjutnya gunakan uniqueElements
	log.Printf("Ditemukan %d elemen unik. Map tier dibuat untuk %d elemen.", len(rawElements), len(elementTiers))

	// Penyesuaian Tier untuk elemen dasar JIKA PERLU (jika mereka tidak ada di tabel pertama/tier yang diharapkan)
	// Contoh: jika ingin memastikan elemen dasar selalu Tier 1
	// for baseElemName := range baseElements {
	// 	if tier, ok := elementTiers[baseElemName]; !ok || tier != 1 {
	// 		log.Printf("Menyesuaikan tier untuk elemen dasar %s menjadi 1", baseElemName)
	// 		elementTiers[baseElemName] = 1
	// 		// Update juga di rawElements jika perlu
	// 		for i, el := range rawElements {
	// 			if el.Name == baseElemName {
	// 				rawElements[i].Tier = 1
	// 				break
	// 			}
	// 		}
	// 	}
	// }


	// --- Tahap 3: Filter Elemen Invalid Awal (Berdasarkan resep kosong & bukan base) ---
	// (Menggunakan logika dari Tahap 1 di kode Anda)
	invalidElements := make(map[string]bool)
	log.Println("Filter Awal: Mengidentifikasi elemen invalid (tanpa resep, bukan base)...")
	for _, elem := range rawElements {
		if len(elem.Recipes) == 0 && !baseElements[elem.Name] {
			invalidElements[elem.Name] = true
			// log.Printf("[FILTER AWAL] Menandai elemen invalid: %s (Tier %d)", elem.Name, elem.Tier)
		}
	}
	log.Printf("Filter Awal: %d elemen ditandai sebagai invalid awal.", len(invalidElements))


	// --- Tahap 4: Filter Resep Berdasarkan Keberadaan Parent & Tier ---
	// (Menggabungkan Tahap 2 dan penambahan filter tier)
	log.Println("Filter Resep: Menghapus resep dengan parent invalid & validasi tier...")
	intermediateElements := []Element{}
	recipesRemovedDueToInvalidParent := 0
	recipesRemovedDueToTier := 0

	for _, elem := range rawElements {
		if invalidElements[elem.Name] { // Lewati elemen yang sudah ditandai invalid
			continue
		}

		elemTier, childTierExists := elementTiers[elem.Name]
		if !childTierExists {
			log.Printf("[WARN TIER] Tier untuk elemen anak '%s' tidak ditemukan saat filter resep. Melewati.", elem.Name)
			continue
		}

		validRecipes := [][]string{}
		for _, recipe := range elem.Recipes {
			if len(recipe) != 2 { continue }
			parent1 := recipe[0]
			parent2 := recipe[1]

			// Cek apakah parent invalid (dari filter awal)
			if invalidElements[parent1] || invalidElements[parent2] {
				recipesRemovedDueToInvalidParent++
				continue // Jangan tambahkan resep ini
			}

			// Cek tier parent
			parent1Tier, p1TierExists := elementTiers[parent1]
			parent2Tier, p2TierExists := elementTiers[parent2]

			if !p1TierExists || !p2TierExists {
				recipesRemovedDueToInvalidParent++ // Atau hitung sbg error tier berbeda
				// log.Printf("[WARN TIER] Tier untuk parent '%s' atau '%s' tidak ditemukan untuk resep elemen '%s'.", parent1, parent2, elem.Name)
				continue // Lewati resep jika tier parent tidak ditemukan
			}

			// Syarat TIER BARU: tier kedua parent harus LEBIH RENDAH dari tier elemen anak
			if parent1Tier < elemTier && parent2Tier < elemTier {
				validRecipes = append(validRecipes, recipe)
			} else {
				recipesRemovedDueToTier++
				// log.Printf("[FILTER TIER] Resep dihapus dari %s (Tier %d): %v (Parents: %s [T%d], %s [T%d])",
				// 	elem.Name, elemTier, recipe, parent1, parent1Tier, parent2, parent2Tier)
			}
		}
		intermediateElements = append(intermediateElements, Element{Name: elem.Name, Recipes: validRecipes, Tier: elemTier})
	}
	log.Printf("Filter Resep: %d resep dihapus (parent invalid), %d resep dihapus (aturan tier). Sisa elemen: %d.",
		recipesRemovedDueToInvalidParent, recipesRemovedDueToTier, len(intermediateElements))


	// --- Tahap 5: Filter Elemen yang Kehabisan Resep Setelah Filter Resep ---
	// (Menggunakan logika dari Tahap 3 di kode Anda)
	finalElementsStage1 := []Element{}
	log.Println("Filter Elemen (Pasca-Resep): Menghapus elemen (non-base) yang menjadi tanpa resep...")
	elementsRemovedNoRecipe := 0
	finalElementNames := make(map[string]bool) // Set nama elemen yang lolos tahap ini

	for _, elem := range intermediateElements {
		if baseElements[elem.Name] || len(elem.Recipes) > 0 {
			finalElementsStage1 = append(finalElementsStage1, elem)
			finalElementNames[elem.Name] = true
		} else {
			elementsRemovedNoRecipe++
			// log.Printf("[FILTER ELEMEN] Menghapus elemen %s (Tier %d) karena kehabisan resep.", elem.Name, elem.Tier)
		}
	}
	log.Printf("Filter Elemen (Pasca-Resep): %d elemen dihapus. Elemen sementara: %d.", elementsRemovedNoRecipe, len(finalElementsStage1))


	// --- Tahap 6: Verifikasi Final Resep (Memastikan Semua Parent Ada di Daftar Final) ---
	// (Menggunakan logika dari Tahap 4 di kode Anda, TAPI filter tier sudah dilakukan sebelumnya)
	log.Println("Verifikasi Final: Memastikan semua parent dalam resep ada di daftar elemen lolos...")
	verifiedFinalElements := []Element{}
	recipesRemovedFinalVerification := 0
	elementsRemovedFinalVerification := 0

	for _, elem := range finalElementsStage1 {
		elemTier := elem.Tier // Tier sudah ada di elem
		verifiedRecipes := [][]string{}
		for _, recipe := range elem.Recipes {
			if len(recipe) != 2 { continue }
			parent1 := recipe[0]
			parent2 := recipe[1]

			// Cek apakah parent ada di daftar elemen yang sudah lolos tahap sebelumnya
			_, parent1Exists := finalElementNames[parent1]
			_, parent2Exists := finalElementNames[parent2]

			if parent1Exists && parent2Exists {
				// Filter tier sudah dilakukan, jadi di sini kita hanya cek keberadaan
				verifiedRecipes = append(verifiedRecipes, recipe)
			} else {
				recipesRemovedFinalVerification++
				// log.Printf("[VERIFIKASI FINAL] Resep dihapus dari %s: %v (parent tidak ada di daftar final)", elem.Name, recipe)
			}
		}

		if baseElements[elem.Name] || len(verifiedRecipes) > 0 {
			verifiedFinalElements = append(verifiedFinalElements, Element{Name: elem.Name, Recipes: verifiedRecipes, Tier: elemTier})
		} else {
			elementsRemovedFinalVerification++
			// log.Printf("[VERIFIKASI FINAL] Menghapus elemen %s (Tier %d) karena kehabisan resep setelah verifikasi.", elem.Name, elem.Tier)
		}
	}
	log.Printf("Verifikasi Final: %d resep dihapus. %d elemen dihapus. Elemen final: %d.",
		recipesRemovedFinalVerification, elementsRemovedFinalVerification, len(verifiedFinalElements))


	// --- Simpan Hasil Akhir ke JSON ---
	if len(verifiedFinalElements) > 0 {
		fmt.Println("Memulai proses marshaling data yang sudah difilter ke JSON...")
		jsonData, err := json.MarshalIndent(verifiedFinalElements, "", "  ")
		if err != nil {
			log.Fatalf("FATAL: Error marshaling data final ke JSON: %s", err)
		}
		fmt.Println("Marshal JSON berhasil.")

		outputFileName := "elements_filtered_with_tier.json"
		fmt.Printf("Menulis data JSON yang sudah difilter ke file '%s'...\n", outputFileName)

		err = os.WriteFile(outputFileName, jsonData, 0644)
		if err != nil {
			log.Fatalf("FATAL: Error menulis file JSON final: %s", err)
		}
		fmt.Printf("Berhasil menulis data terfilter ke '%s'. Jumlah elemen: %d\n", outputFileName, len(verifiedFinalElements))

	} else {
		fmt.Println("Tidak ada elemen yang tersisa setelah filter, file JSON tidak dibuat.")
	}

	fmt.Println("\nScraping dan Filtering Selesai.")
}