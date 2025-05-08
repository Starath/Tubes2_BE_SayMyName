package pathfinding

import (
	"container/list"
	"fmt"
	"log"

  "github.com/Starath/Tubes2_BE_SayMyName/loadrecipes"
  "github.com/Starath/Tubes2_BE_SayMyName/pathfinding"
)

// dfsRecursiveHelperString adalah fungsi rekursif inti untuk DFS mundur (versi string).
// Mengembalikan true jika path ditemukan, false jika tidak.
// pathSteps: map untuk menyimpan langkah-langkah resep yang ditemukan (hanya langkah terakhir per elemen).
// currentlySolving: set untuk deteksi siklus dalam path rekursif saat ini.
// memo: map untuk menyimpan hasil elemen yang sudah dihitung (true=dapat dibuat, false=tidak dapat dibuat).
// visitedCounter: pointer ke integer untuk menghitung node unik yang dieksplorasi.
func dfsRecursiveHelperString(
  elementName string,
  graph *loadrecipes.BiGraphAlchemy,
  pathSteps map[string]pathfinding.PathStep, // map[childName]pathfinding.PathStep
  currentlySolving map[string]bool,
  memo map[string]bool,
  visitedCounter *int,
) bool {

  // 1. Cek Memoization dan Hitung Kunjungan Unik
  if canBeMade, exists := memo[elementName]; exists {
    return canBeMade // Kembalikan hasil yg sudah disimpan
  }
  // Jika belum ada di memo, berarti ini eksplorasi pertama untuk node ini
  *visitedCounter++

  // 2. Cek Base Case (Elemen Dasar)
  if graph.BaseElements[elementName] {
    memo[elementName] = true // Elemen dasar bisa "dibuat"
    return true
  }

  // 3. Cek Siklus
  if currentlySolving[elementName] {
    // Tidak simpan di memo karena ini hanya siklus di path *saat ini*
    return false // Siklus terdeteksi
  }

  // 4. Tandai sedang diselesaikan
  currentlySolving[elementName] = true
  // 'defer' akan dijalankan tepat sebelum fungsi return
  defer delete(currentlySolving, elementName) // Hapus tanda saat backtrack

  // 5. Cek apakah elemen punya resep (menggunakan map mundur)
  parentPairs, hasRecipes := graph.ChildToParents[elementName]
  if !hasRecipes {
    memo[elementName] = false // Tidak bisa dibuat jika tidak ada resep
    return false
  }

  // 6. Coba setiap resep secara rekursif
  foundPath := false // Flag apakah salah satu resep berhasil
  for _, pair := range parentPairs {
    // Coba selesaikan parent 1
    canMakeP1 := dfsRecursiveHelperString(pair.Mat1, graph, pathSteps, currentlySolving, memo, visitedCounter)
    if !canMakeP1 {
      continue // Jika parent 1 tidak bisa dibuat, coba resep lain
    }

    // Coba selesaikan parent 2
    canMakeP2 := dfsRecursiveHelperString(pair.Mat2, graph, pathSteps, currentlySolving, memo, visitedCounter)
    if !canMakeP2 {
      continue // Jika parent 2 tidak bisa dibuat, coba resep lain
    }

    // Jika KEDUA parent bisa dibuat
    if canMakeP1 && canMakeP2 {
      // Simpan langkah resep ini (langkah terakhir yg berhasil utk elemen ini)
      pathSteps[elementName] = pathfinding.PathStep{ChildName: elementName, Parent1Name: pair.Mat1, Parent2Name: pair.Mat2}
      foundPath = true // Setidaknya satu resep berhasil
      // PENTING: Untuk DFS dasar (cari 1 jalur), kita bisa langsung return true di sini
      // setelah menemukan satu cara. Jika ingin mencari semua jalur atau jalur 'terbaik'
      // menurut kriteria lain, kita tidak return di sini tapi lanjutkan loop.
      break // Hentikan loop resep setelah menemukan 1 cara valid
    }
  }

  // 7. Simpan hasil ke memo berdasarkan flag foundPath
  memo[elementName] = foundPath
  return foundPath
}

// DFSFindPathString memulai pencarian DFS mundur (versi string).
func DFSFindPathString(graph *loadrecipes.BiGraphAlchemy, targetElementName string) (*pathfinding.DFSResult, error) {
  if _, targetExists := graph.AllElements[targetElementName]; !targetExists {
    return nil, fmt.Errorf(fmt.Sprintf("Elemen target '%s' tidak ditemukan.", targetElementName))
  }

  if graph.BaseElements[targetElementName] {
    return &pathfinding.DFSResult{Path: []pathfinding.PathStep{}, NodesVisited: 1}, nil // Elemen dasar
  }

  pathSteps := make(map[string]pathfinding.PathStep)   // Hanya menyimpan langkah terakhir per elemen
  currentlySolving := make(map[string]bool) // Deteksi siklus per path
  memo := make(map[string]bool)           // Memoization global untuk run ini
  visitedCount := 0                       // Counter node unik yg dieksplorasi

  success := dfsRecursiveHelperString(targetElementName, graph, pathSteps, currentlySolving, memo, &visitedCount)

  if success {
    // Rekonstruksi path lengkap dari pathSteps
    finalPath := make([]pathfinding.PathStep, 0, len(pathSteps))
    reconstructionQueue := list.New()
    reconstructionQueue.PushBack(targetElementName)
    addedToFinalPath := make(map[string]bool) // Hindari duplikasi elemen dalam path

    for reconstructionQueue.Len() > 0 {
      elem := reconstructionQueue.Front()
      reconstructionQueue.Remove(elem)
      currName := elem.Value.(string)

      // Jangan proses elemen dasar atau yg sudah ada di final path
      if graph.BaseElements[currName] || addedToFinalPath[currName] {
        continue
      }

      step, exists := pathSteps[currName]
      if exists {
        // Masukkan step ke hasil, tandai, dan masukkan parent ke queue
        finalPath = append(finalPath, step)
        addedToFinalPath[currName] = true

        if !addedToFinalPath[step.Parent1Name] {
          reconstructionQueue.PushBack(step.Parent1Name)
        }
        if !addedToFinalPath[step.Parent2Name] {
          reconstructionQueue.PushBack(step.Parent2Name)
        }
      } else if !graph.BaseElements[currName] {
        // Ini bisa terjadi jika elemen dibutuhkan tapi tidak ada step yg tercatat
        // (misalnya jika elemen tersebut dibuat oleh resep yg cabangnya gagal)
        // Ini menandakan rekonstruksi sederhana ini mungkin tidak lengkap jika DFS
        // tidak mencatat *semua* step yang berhasil.
        log.Printf("WARNING: Tidak ada path step tercatat untuk '%s' saat rekonstruksi DFS.", currName)
        // Kita bisa coba lanjutkan atau return error di sini tergantung kebutuhan.
        // Untuk DFS dasar, path yang ditemukan mungkin hanya sebagian jika strukturnya kompleks.
      }
    }
    // Balikkan slice agar urutan dari dasar ke target
    for i, j := 0, len(finalPath)-1; i < j; i, j = i+1, j-1 {
        finalPath[i], finalPath[j] = finalPath[j], finalPath[i]
    }

    return &pathfinding.DFSResult{Path: finalPath, NodesVisited: visitedCount}, nil
  }

  // Jika success == false
  // Periksa memo untuk melihat apakah target memang tidak bisa dibuat
  nodesExploredFinal := visitedCount
  if !memo[targetElementName] {
       // Pastikan hitungan visited sudah final jika gagal
       if _, visited := memo[targetElementName]; !visited {
           nodesExploredFinal++
       }
      log.Printf("INFO: Elemen '%s' ditandai tidak dapat dibuat (memo=false).\n", targetElementName)
  }


  return nil, fmt.Errorf(fmt.Sprintf("Tidak ditemukan jalur resep untuk elemen '%s' (Nodes Explored: %d).", targetElementName, nodesExploredFinal))
}

// --- Contoh Penggunaan ---
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