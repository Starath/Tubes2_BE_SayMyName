// File: scrape/scrapper.go
package scrape

import (
	"bytes"
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
}

func Scrapping() {
	url := "https://little-alchemy.fandom.com/wiki/Elements_(Little_Alchemy_2)"
	fmt.Println("Memulai scraping dari:", url)

	// --- Bagian HTTP Request dan Pembacaan Body (Tetap Sama) ---
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

	// --- Tahap 1: Scrape Semua Data Awal ---
	var rawElements []Element // Slice untuk menampung hasil scraping awal
	baseElements := map[string]bool{"Air": true, "Earth": true, "Fire": true, "Water": true}

	fmt.Println("Mencari dan memproses tabel elemen (scrape awal)...")
	tables := doc.Find("table.list-table")
	tables.Each(func(tableIndex int, table *goquery.Selection) {
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
								// Validasi sederhana, bisa diperketat jika perlu
								if singleRecipePair[0] != "" && singleRecipePair[1] != "" {
									currentElement.Recipes = append(currentElement.Recipes, singleRecipePair)
								} else {
									log.Printf("[WARN SCRAPE] Resep tidak lengkap untuk %s: %v", elementName, singleRecipePair)
								}
							}
						})
					}
					// Cek duplikasi sebelum append (opsional tapi bagus)
					// Untuk simpelnya, kita biarkan duplikasi dan tangani nanti jika perlu
					rawElements = append(rawElements, currentElement)
				}
			})
		})
	})
	fmt.Printf("Scraping awal selesai. Ditemukan %d entri elemen (mungkin termasuk duplikat).\n", len(rawElements))

	// --- Tahap 2: Identifikasi Elemen Invalid Awal ---
	invalidElements := make(map[string]bool)
	log.Println("Filter Tahap 1: Mengidentifikasi elemen invalid awal (tanpa resep, bukan base)...")
	elementNamesFound := make(map[string]bool) // Untuk cek duplikasi nama
	uniqueRawElements := []Element{} // Untuk menyimpan elemen unik
	for _, elem := range rawElements {
		if _, exists := elementNamesFound[elem.Name]; exists {
			// Jika nama sudah ada, mungkin gabungkan resep atau abaikan duplikat
			// Untuk simpelnya, kita abaikan duplikat setelah yang pertama
			continue
		}
		elementNamesFound[elem.Name] = true
		uniqueRawElements = append(uniqueRawElements, elem) // Tambahkan elemen unik

		if len(elem.Recipes) == 0 && !baseElements[elem.Name] {
			invalidElements[elem.Name] = true
			log.Printf("[FILTER] Menandai elemen invalid awal: %s", elem.Name)
		}
	}
	rawElements = uniqueRawElements // Gunakan elemen unik untuk proses selanjutnya
	log.Printf("Filter Tahap 1: Ditemukan %d elemen unik. Ditemukan %d elemen invalid awal.", len(rawElements), len(invalidElements))


	// --- Tahap 3: Filter Elemen Invalid Awal & Resepnya ---
	intermediateElements := make([]Element, 0)
	log.Println("Filter Tahap 2: Menghapus elemen invalid awal & resep yang menggunakannya...")
	recipesRemovedCount := 0
	for _, elem := range rawElements {
		// Lewati elemen yang memang invalid dari awal
		if invalidElements[elem.Name] {
			continue
		}

		// Filter resep untuk elemen yang tersisa
		validRecipes := make([][]string, 0)
		for _, recipe := range elem.Recipes {
			// Asumsi resep selalu punya 2 parent setelah scrape awal
			if len(recipe) != 2 { continue } // Safety check
			parent1 := recipe[0]
			parent2 := recipe[1]
			// Resep valid jika KEDUA parent TIDAK ADA di daftar invalid
			if !invalidElements[parent1] && !invalidElements[parent2] {
				validRecipes = append(validRecipes, recipe)
			} else {
				recipesRemovedCount++
				// log.Printf("[FILTER] Resep dihapus dari %s: %v (karena %s atau %s invalid)", elem.Name, recipe, parent1, parent2)
			}
		}
		// Tambahkan elemen (yang tidak invalid) dengan resep yang sudah difilter
		intermediateElements = append(intermediateElements, Element{Name: elem.Name, Recipes: validRecipes})
	}
	log.Printf("Filter Tahap 2: %d resep dihapus karena menggunakan elemen invalid. Sisa elemen (sementara): %d.", recipesRemovedCount, len(intermediateElements))


	// --- Tahap 4: Filter Elemen yang Kehabisan Resep ---
	finalFilteredElements := make([]Element, 0)
	log.Println("Filter Tahap 3: Menghapus elemen (non-base) yang menjadi tanpa resep...")
	elementsRemovedCountStage3 := 0
	finalElementNames := make(map[string]bool) // Set nama elemen final
	for _, elem := range intermediateElements {
		// Simpan elemen dasar ATAU elemen yang masih punya resep setelah filter tahap 2
		if baseElements[elem.Name] || len(elem.Recipes) > 0 {
			finalFilteredElements = append(finalFilteredElements, elem)
			finalElementNames[elem.Name] = true // Catat nama elemen yang lolos
		} else {
			elementsRemovedCountStage3++
			log.Printf("[FILTER] Menghapus elemen karena kehabisan resep: %s", elem.Name)
		}
	}
	log.Printf("Filter Tahap 3: %d elemen tambahan dihapus. Elemen final: %d.", elementsRemovedCountStage3, len(finalFilteredElements))

	// --- Tahap 5 (Opsional tapi Direkomendasikan): Verifikasi Resep Final ---
	// Pastikan semua parent dalam resep final ada di daftar elemen final
	log.Println("Filter Tahap 4: Verifikasi final resep (memastikan semua parent ada di hasil akhir)...")
	verifiedFinalElements := make([]Element, 0, len(finalFilteredElements))
	recipesRemovedCountStage4 := 0
	for _, elem := range finalFilteredElements {
			verifiedRecipes := make([][]string, 0, len(elem.Recipes))
			for _, recipe := range elem.Recipes {
					parent1Exists := finalElementNames[recipe[0]]
					parent2Exists := finalElementNames[recipe[1]]
					if parent1Exists && parent2Exists {
							verifiedRecipes = append(verifiedRecipes, recipe)
					} else {
							recipesRemovedCountStage4++
							log.Printf("[FILTER FINAL] Resep dihapus dari %s: %v (karena parent %s atau %s tidak lolos filter)", elem.Name, recipe, recipe[0], recipe[1])
					}
			}
			// Tambahkan lagi, kali ini dengan resep yang sudah diverifikasi final
			// Kita juga harus cek lagi apakah elemen jadi kosong resepnya SETELAH verifikasi ini
			if baseElements[elem.Name] || len(verifiedRecipes) > 0 {
					verifiedFinalElements = append(verifiedFinalElements, Element{Name: elem.Name, Recipes: verifiedRecipes})
			} else {
				 log.Printf("[FILTER FINAL] Menghapus elemen %s karena kehabisan resep SETELAH verifikasi final.", elem.Name)
			}
	}
	log.Printf("Filter Tahap 4: %d resep tambahan dihapus saat verifikasi final. Elemen final: %d.", recipesRemovedCountStage4, len(verifiedFinalElements))

	// --- Simpan Hasil Akhir ke JSON ---
	if len(verifiedFinalElements) > 0 {
		fmt.Println("Memulai proses marshaling data yang sudah difilter ke JSON...")
		// Gunakan verifiedFinalElements untuk output final
		jsonData, err := json.MarshalIndent(verifiedFinalElements, "", "  ")
		if err != nil {
			log.Fatalf("FATAL: Error marshaling data final ke JSON: %s", err)
		}
		fmt.Println("Marshal JSON berhasil.")

		outputFileName := "elements_filtered.json" // Simpan ke nama file berbeda agar tidak menimpa yg asli
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