// File: imagedownloader/downloader.go
package imagedownloader

import (
	"crypto/tls" // Dari referensi Anda
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time" // Dari referensi Anda
)

// Definisikan ulang struct Element di sini, pastikan konsisten dengan scrapper.go
type Element struct {
	Name     string     `json:"name"`
	Recipes  [][]string `json:"recipes"` // Mungkin tidak digunakan di sini tapi ada di JSON
	Tier     int        `json:"tier"`    // Mungkin tidak digunakan di sini tapi ada di JSON
	ImageURL string     `json:"imageUrl,omitempty"`
}

const (
	// Sesuaikan path ini jika perlu, atau terima sebagai argumen fungsi
	outputDirRoot          = "downloaded_images" // Nama folder output utama
	maxConcurrentDownloads = 10
	requestTimeout         = 30 * time.Second
)

// Fungsi sanitizeFilename dari referensi Anda
func sanitizeFilename(name string) string {
	replacer := strings.NewReplacer(
		"/", "_", "\\", "_", ":", "_", "*", "_", "?", "_",
		"\"", "_", "<", "_", ">", "_", "|", "_", " ", "_",
		"!", "", // Menghapus tanda seru juga bisa bagus
	)
	return replacer.Replace(name)
}

// Fungsi getFileExtension dari referensi Anda
func getFileExtension(rawURL string, contentType string) string {
	parsedURL, err := url.Parse(rawURL)
	if err == nil {
		ext := filepath.Ext(parsedURL.Path)
		if ext != "" && len(ext) > 1 {
			// Bersihkan parameter query dari ekstensi
			cleanExt := strings.Split(strings.ToLower(ext), "?")[0]
			// Validasi sederhana untuk panjang ekstensi
			if len(cleanExt) > 1 && len(cleanExt) <= 5 { // e.g., .svg, .png
				return cleanExt
			}
		}
	}
	// Fallback ke Content-Type jika ekstensi dari URL tidak valid/ada
	if contentType != "" {
		switch strings.ToLower(strings.Split(contentType, ";")[0]) {
		case "image/jpeg":
			return ".jpg"
		case "image/png":
			return ".png"
		case "image/gif":
			return ".gif"
		case "image/svg+xml":
			return ".svg"
		case "image/webp":
			return ".webp"
		case "image/avif":
			return ".avif"
		}
	}
	log.Printf("Tidak dapat menentukan ekstensi untuk URL: %s, Content-Type: '%s'. Menggunakan default .bin (binary)\n", rawURL, contentType)
	return ".bin" // Default ke .bin jika tidak ada yang cocok
}

func downloadIndividualFile(element Element, fullOutputDir string, client *http.Client, wg *sync.WaitGroup, sem chan struct{}) {
	defer wg.Done()
	defer func() { <-sem }() // Lepaskan slot semaphore

	// Menggunakan element.Name untuk baseFileName, lalu disanitasi
	baseFileName := sanitizeFilename(element.Name)

	fmt.Printf("Mencoba mengunduh: '%s' dari %s\n", element.Name, element.ImageURL)

	if element.ImageURL == "" {
		log.Printf("URL gambar kosong untuk elemen: '%s'\n", element.Name)
		return
	}

	req, err := http.NewRequest("GET", element.ImageURL, nil)
	if err != nil {
		log.Printf("Gagal membuat request untuk '%s' (%s): %v\n", element.Name, element.ImageURL, err)
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Gagal mengunduh '%s' (%s): %v\n", element.Name, element.ImageURL, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Gagal mengunduh '%s' (%s): status code %d\n", element.Name, element.ImageURL, resp.StatusCode)
		return
	}

	contentType := resp.Header.Get("Content-Type")
	fileExt := getFileExtension(element.ImageURL, contentType)
	finalFileName := baseFileName + fileExt // Nama file menggunakan nama elemen yang disanitasi
	filePath := filepath.Join(fullOutputDir, finalFileName)

	// Cek apakah file sudah ada, jika iya, lewati
	if _, err := os.Stat(filePath); err == nil {
		// fmt.Printf("INFO: File '%s' sudah ada. Melewati.\n", filePath)
		return
	}

	out, err := os.Create(filePath)
	if err != nil {
		log.Printf("Gagal membuat file '%s' untuk '%s': %v\n", filePath, element.Name, err)
		return
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		log.Printf("Gagal menyimpan gambar '%s' ke '%s': %v\n", element.Name, filePath, err)
		os.Remove(filePath) // Hapus file jika gagal copy
		return
	}

	fmt.Printf("Berhasil mengunduh '%s' -> '%s'\n", element.Name, filePath)
}

// DownloadAllImages membaca file JSON dan mengunduh semua gambar elemen.
func DownloadAllImages(jsonInputPath string) {
	log.Println("Mulai proses pengunduhan gambar...")

	jsonData, err := os.ReadFile(jsonInputPath)
	if err != nil {
		log.Fatalf("Gagal membaca file JSON '%s': %v\n", jsonInputPath, err)
	}

	var elements []Element // Menggunakan struct Element kita
	err = json.Unmarshal(jsonData, &elements)
	if err != nil {
		log.Fatalf("Gagal unmarshal data JSON: %v\n", err)
	}

	if len(elements) == 0 {
		log.Println("Tidak ada elemen ditemukan dalam file JSON.")
		return
	}
	fmt.Printf("Ditemukan %d elemen gambar untuk diunduh.\n", len(elements))

	// Buat direktori output
	absFinalOutputDir, err := filepath.Abs(outputDirRoot)
	if err != nil {
		log.Fatalf("Gagal mendapatkan path absolut untuk direktori output '%s': %v\n", outputDirRoot, err)
	}

	if _, err := os.Stat(absFinalOutputDir); os.IsNotExist(err) {
		errDir := os.MkdirAll(absFinalOutputDir, 0755)
		if errDir != nil {
			log.Fatalf("Gagal membuat direktori output '%s': %v\n", absFinalOutputDir, errDir)
		}
		fmt.Printf("Direktori output '%s' berhasil dibuat.\n", absFinalOutputDir)
	} else {
		fmt.Printf("Direktori output '%s' sudah ada.\n", absFinalOutputDir)
	}

	// Setup untuk unduhan konkurensi
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrentDownloads)

	// HTTP Client dari referensi Anda
	customTransport := &http.Transport{
		TLSNextProto: make(map[string]func(authority string, c *tls.Conn) http.RoundTripper),
	}
	client := &http.Client{
		Timeout:   requestTimeout,
		Transport: customTransport,
	}

	// Iterasi dan unduh setiap gambar
	for _, el := range elements {
		if el.ImageURL == "" {
			// log.Printf("Melewati elemen '%s' karena URL gambar kosong.\n", el.Name)
			continue
		}
		wg.Add(1)
		sem <- struct{}{} // Ambil slot semaphore
		go downloadIndividualFile(el, absFinalOutputDir, client, &wg, sem)
	}

	wg.Wait() // Tunggu semua goroutine selesai
	close(sem) // Tutup channel semaphore

	log.Println("Proses pengunduhan gambar selesai.")
	fmt.Printf("Gambar seharusnya telah diunduh ke: %s\n", absFinalOutputDir)
}