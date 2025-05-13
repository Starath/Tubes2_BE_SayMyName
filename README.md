# Tubes2_SayMyName
  
</br>

<H2 align="center">Pemanfaatan Algoritma BFS dan DFS dalam Pencarian Recipe pada Permainan Little Alchemy 2</p>


##  Authors: Kelompok XX - SayMyName

<div align="center">

<table border="1" cellspacing="0" cellpadding="8"> 
  <tr> <th>NIM</th> <th>Nama</th> </tr> 
  <tr> <td>13523095</td> <td>Rafif Farras</td> </tr> 
  <tr> <td>13523096</td> <td>Muhammad Edo Raduputu Aprima</td> 
  </tr> <tr> <td>13523106</td> <td>Athian Nugraha Muarajuang</td> </tr> </table>

</div>

## Deskripsi Repository
Web untuk mencari cara membuat elemen tertentu dalam permainan Little Alchemy 2. Pengguna dapat memilih elemen target, metode pencarian (BFS atau DFS), mode pencarian (_single recipe_ atau _multiple recipes_), dan jumlah _recipe_ yang ingin ditampilkan. Hasil pencarian akan ditampilkan dalam bentuk visualisasi _tree_ yang interaktif serta informasi mengenai waktu eksekusi dan jumlah _node_ yang dikunjungi.

## Getting Started

First, run the main_server.go
```bash
go run main_server.go
```
Then go to FE directory to run the web. You can clone it by doing this command
```bash
git clone https://github.com/rafifrs/Tubes2_FE_SayMyName.git
```

### Fitur Wajib

| **No** | **Fitur**         | **Status** |
|--------|-------------------|------------|
| 1      | Aplikasi dapat dijalankan |   ✅      |
| 2      | Aplikasi dapat memperoleh data recipe melalui scraping| ✅      |
| 3      | Algoritma Depth First Search dan Breadth First Search dapat menemukan recipe elemen dengan benar| ✅      |
| 4      | Aplikasi dapat menampilkan visualisasi recipe elemen yang dicari sesuai dengan spesifikasi| ✅      |
| 5      | Aplikasi mengimplementasikan multithreading | ✅      |
| 6      | Membuat laporan sesuai dengan spesifikasi | ✅     |

### Fitur Bonus

| **No** | **Fitur**                        | **Status** |
|--------|----------------------------------|------------|
| 1      | Membuat bonus video dan diunggah pada Youtube                            | ✅         |
| 2      | Membuat bonus algoritma pencarian Bidirectional| ❌         |
| 3      | Membuat bonus Live Update| ❌         |
| 4      | Aplikasi di-containerize dengan Docker| ✅         |
| 5      | Aplikasi di-deploy dan dapat diakses melalui internet| ❌         |
