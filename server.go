package main

import (
	"log"
	"net/http" // Dipertahankan untuk konstanta status HTTP
	"strconv"  // Diperlukan untuk parsing maxPaths nanti
	"time"

	"github.com/Starath/Tubes2_BE_SayMyName/loadrecipes"
	"github.com/gin-gonic/gin" // Import Gin
)

// Global variable to hold the loaded graph data
// Akan diisi saat server start oleh LoadBiGraph
var alchemyGraph *loadrecipes.BiGraphAlchemy

// Definisikan struktur data untuk hasil pencarian
// TODO (Person 1 & 3): Finalisasi struktur RecipeTree agar sesuai untuk visualisasi frontend
type SearchResult struct {
	RecipeTree     interface{} `json:"recipeTree"` // Tipe sementara, akan diganti struktur tree sebenarnya
	TimeTakenMs    float64     `json:"timeTakenMs"`
	NodesVisited   int         `json:"nodesVisited"`
	Found          bool        `json:"found"`
	TargetElement  string      `json:"targetElement"`
	Algorithm      string      `json:"algorithm"`
	SearchType     string      `json:"searchType"`
	ErrorMessage   string      `json:"errorMessage,omitempty"` // Untuk menampilkan pesan error ke frontend
}

// Contoh struktur node tree untuk mock data (bisa disesuaikan)
type MockRecipeNode struct {
	Name     string            `json:"name"`
	Children []*MockRecipeNode `json:"children,omitempty"`
}

// Middleware untuk menangani CORS (Cross-Origin Resource Sharing)
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO (Opsional): Buat origin ini lebih fleksibel atau gunakan env variable untuk production
		c.Writer.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// Handler untuk endpoint /search
func searchHandlerGin(c *gin.Context) {
	// --- Parsing Parameter Request ---
	targetElement := c.Query("target")
	algorithm := c.Query("algorithm")   // "bfs" atau "dfs"
	searchType := c.Query("searchType") // "shortest" atau "multiple"
	maxPathsStr := c.Query("maxPaths")  // String, perlu di-parse ke int jika 'multiple'

	// TODO (Phase 2/3 - Person 2): Tambahkan validasi input yang lebih robust
	// - Cek apakah elemen target ada di `alchemyGraph.AllElements`?
	// - Cek apakah algoritma dan searchType valid?
	// - Pastikan maxPaths adalah integer positif jika searchType="multiple".
	if targetElement == "" || algorithm == "" || searchType == "" {
		log.Printf("[WARN] Bad request: Missing parameters. Target=%s, Algo=%s, Type=%s", targetElement, algorithm, searchType)
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Missing required parameters: target, algorithm, searchType"})
		return
	}

	log.Printf("[INFO] Received search request: Target=%s, Algorithm=%s, Type=%s, MaxPaths=%s\n", targetElement, algorithm, searchType, maxPathsStr)

	// Inisialisasi variabel hasil
	var result SearchResult
	startTime := time.Now()

	// --- BAGIAN LOGIKA PENCARIAN SEBENARNYA (PLACEHOLDER FASE 1) ---
	// TODO (Phase 2 - Person 1): Ganti seluruh blok 'if searchType' ini dengan pemanggilan fungsi BFS/DFS yang sebenarnya.
	// Fungsi tersebut harus menerima `alchemyGraph`, `targetElement`, `algorithm`, `searchType`, dan `maxPathsInt`.
	// Fungsi harus mengembalikan struktur data `RecipeTree` yang sesuai, `nodesVisited`, dan status `found`.

	if searchType == "shortest" {
		// --- START PLACEHOLDER SHORTEST PATH ---
		log.Println("[INFO] Using MOCK logic for shortest path...")
		if targetElement == "Mud" {
			result = SearchResult{
				RecipeTree: &MockRecipeNode{
					Name: "Mud",
					Children: []*MockRecipeNode{
						{Name: "Water"}, {Name: "Earth"},
					},
				},
				NodesVisited: 5, Found: true,
			}
		} else if targetElement == "Brick" {
			result = SearchResult{
				RecipeTree: &MockRecipeNode{
					Name: "Brick",
					Children: []*MockRecipeNode{
						{Name: "Mud", Children: []*MockRecipeNode{{Name: "Water"}, {Name: "Earth"}}},
						{Name: "Fire"},
					},
				},
				NodesVisited: 15, Found: true,
			}
		} else {
			// Mock: Elemen tidak ditemukan
			result = SearchResult{RecipeTree: nil, NodesVisited: 2, Found: false, ErrorMessage: "Element not found or mock not implemented"}
		}
		// --- END PLACEHOLDER SHORTEST PATH ---

	} else if searchType == "multiple" {
		// TODO (Phase 2 - Person 2): Parse `maxPathsStr` ke integer (`maxPathsInt`)
		maxPathsInt, err := strconv.Atoi(maxPathsStr)
		if err != nil || maxPathsInt <= 0 {
			log.Printf("[WARN] Invalid maxPaths parameter: %s", maxPathsStr)
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid or missing positive integer for maxPaths parameter when searchType is 'multiple'"})
			return
		}

		// --- START PLACEHOLDER MULTIPLE PATH ---
		log.Printf("[INFO] Using MOCK logic for multiple paths (max: %d)...", maxPathsInt)
		// TODO (Phase 2 - Person 1): Implementasikan pemanggilan fungsi pencarian multiple path di sini.
		// TODO (Phase 2 - Person 2): Pastikan implementasi ini menggunakan multithreading sesuai spek[cite: 17].
		// Hasilnya mungkin berupa array dari RecipeTree atau struktur lain. Untuk mock, kita return 'not found'.
		result = SearchResult{RecipeTree: []interface{}{}, NodesVisited: 3, Found: false, ErrorMessage: "Multiple path search mock not implemented"} // Placeholder array kosong
		// --- END PLACEHOLDER MULTIPLE PATH ---

	} else {
		log.Printf("[WARN] Invalid searchType: %s", searchType)
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid searchType parameter. Use 'shortest' or 'multiple'"})
		return
	}
	// --- AKHIR BAGIAN LOGIKA PENCARIAN SEBENARNYA ---

	// --- Finalisasi Hasil & Pengiriman Respons ---
	// TODO (Phase 2 - Person 1): Pastikan TimeTakenMs dan NodesVisited diisi dengan nilai aktual dari hasil pencarian.
	result.TimeTakenMs = float64(time.Since(startTime).Microseconds()) / 1000.0 // Hitung waktu aktual di Phase 2
	result.TargetElement = targetElement
	result.Algorithm = algorithm
	result.SearchType = searchType

	if !result.Found && result.ErrorMessage == "" { // Tambahkan pesan error default jika tidak ditemukan
		result.ErrorMessage = "Recipe path not found for the given element."
	}

	// Kirim respons JSON
	if result.Found {
		c.IndentedJSON(http.StatusOK, result)
	} else {
		// Bisa gunakan status OK tapi dengan flag found=false, atau Not Found (404)
		// Menggunakan OK agar frontend lebih mudah handle struktur respons yang konsisten
		c.IndentedJSON(http.StatusOK, result) 
		// Alternatif: c.IndentedJSON(http.StatusNotFound, result) 
	}
	log.Printf("[INFO] Sent response for Target=%s (Found: %t)\n", targetElement, result.Found)
}

// Fungsi untuk memulai server Gin
func StartServer() {
	// Muat data graf saat server dimulai
	var err error
	graphPath := "elements.json" // Sesuaikan path jika perlu
	alchemyGraph, err = loadrecipes.LoadBiGraph(graphPath)
	if err != nil {
		log.Fatalf("[FATAL] Gagal memuat data graf dari '%s': %v\n", graphPath, err)
	}
	if alchemyGraph == nil {
		log.Fatalf("[FATAL] alchemyGraph adalah nil setelah LoadBiGraph, cek implementasi LoadBiGraph atau file JSON.")
	}
	log.Printf("[INFO] Graf Little Alchemy berhasil dimuat dari %s. (Total Elemen: %d)\n", graphPath, len(alchemyGraph.AllElements))

	// Inisialisasi Gin router
	router := gin.Default()

	// Gunakan middleware CORS
	router.Use(CORSMiddleware())

	// Definisikan route API
	router.GET("/search", searchHandlerGin)
	// TODO (Opsional): Tambahkan endpoint lain jika diperlukan (misal: /elements untuk list semua elemen)

	// Jalankan server
	port := ":8080"
	log.Printf("[INFO] Menjalankan server Gin di port %s\n", port)
	err = router.Run(port)
	if err != nil {
		log.Fatalf("[FATAL] Gagal menjalankan server Gin: %v\n", err)
	}
}

// Uncomment fungsi main di bawah ini jika Anda ingin file ini menjadi executable utama
// func main() {
// 	StartServer()
// }