package main

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

func main() {
	url := "https://little-alchemy.fandom.com/wiki/Elements_(Little_Alchemy_2)"
	fmt.Println("Memulai scraping dari:", url)

	client := &http.Client{ Timeout: 30 * time.Second }
	req, err := http.NewRequest("GET", url, nil)
	if err != nil { log.Fatalf("FATAL: Error request: %s", err) }
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/108.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,id;q=0.8")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("DNT", "1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	res, err := client.Do(req)
	if err != nil { log.Fatalf("FATAL: Error GET: %s", err) }
	defer res.Body.Close()
	if res.StatusCode != 200 { log.Printf("WARNING: Status %d", res.StatusCode) }
	var reader io.ReadCloser = res.Body
	encoding := res.Header.Get("Content-Encoding")
	switch strings.ToLower(encoding) {
	case "br":
		brotliReader := brotli.NewReader(res.Body); reader = io.NopCloser(brotliReader)
	}
	defer reader.Close()
	bodyBytes, err := io.ReadAll(reader)
	if err != nil { log.Fatalf("FATAL: read body: %s", err) }
	fmt.Printf("Body response berhasil dibaca (%d bytes).\n", len(bodyBytes))
	

	fmt.Println("Memulai parsing HTML dengan goquery...")
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(bodyBytes))
	if err != nil { log.Fatalf("FATAL: Error parsing HTML: %s", err) }
	fmt.Println("Parsing HTML selesai.")

	var elements []Element // Slice untuk menampung hasil

	fmt.Println("Mencari dan memproses tabel elemen...")
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
								currentElement.Recipes = append(currentElement.Recipes, singleRecipePair)
							}
						}) 
					} 
					elements = append(elements, currentElement)
				}
			}) 
		}) 
	}) 
	fmt.Printf("Proses scraping selesai. Ditemukan %d elemen.\n", len(elements))


	if len(elements) > 0 {
		fmt.Println("Memulai proses marshaling data ke JSON...")
		jsonData, err := json.MarshalIndent(elements, "", "  ") 
		if err != nil {
			log.Fatalf("FATAL: Error marshaling data ke JSON: %s", err)
		}
		fmt.Println("Marshal JSON berhasil.")

		outputFileName := "elements.json"
		fmt.Printf("Menulis data JSON ke file '%s'...\n", outputFileName)

		err = os.WriteFile(outputFileName, jsonData, 0644)
		if err != nil {
			log.Fatalf("FATAL: Error menulis file JSON: %s", err)
		}
		fmt.Printf("Berhasil menulis data ke '%s'.\n", outputFileName)

	} else {
		fmt.Println("Tidak ada elemen yang diekstrak, file JSON tidak dibuat.")
	}

	fmt.Println("\nScraping Selesai.")
}