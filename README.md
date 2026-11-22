# ⚡ MAD-DM (Multi-thread Accelerated Downloader)

MAD-DM adalah aplikasi *Download Manager* modern, berkecepatan tinggi, dan *cross-platform* (macOS, Windows, Linux) yang dirancang dengan fitur-fitur setara **Internet Download Manager (IDM)**.

---

## 🌟 Fitur Utama (IDM Parity)

1. **Multi-Thread & Dynamic Segmentation:** Membagi file menjadi beberapa bagian (4, 8, 16, hingga 32 koneksi simultan) menggunakan HTTP `Range: bytes=start-end`.
2. **Zero-Assembly High-Speed Disk I/O:** Setiap potongan data langsung ditulis ke offset target (`os.File.WriteAt`), tanpa proses penggabungan lambat di akhir unduhan.
3. **Pause & Resume Handal:** Menyimpan status segmen secara persisten ke disk. Jika koneksi terputus atau komputer restart, unduhan dapat dilanjutkan tanpa mengulang dari 0%.
4. **Queue & Auto-Scheduler:** Manajemen antrean unduhan dengan batas konkuren (*worker pool*).
5. **Modern Dashboard GUI:** UI web/desktop interaktif dengan progress bar per-segmen, grafik kecepatan, dan kontrol unduhan.
6. **Integrasi Browser & Video Sniffer (Manifest V3):** Ekstensi Chrome/Firefox/Edge dengan fitur *floating badge* "⚡ Download Video with MAD-DM" di atas pemutar video (seperti tombol grabber IDM).
7. **Multi-OS Native Binary:** Siap di-compile untuk Windows (`.exe`), macOS (`Apple Silicon & Intel`), dan Linux.

---

## 📂 Struktur Project

```text
mad-dm/
├── bin/                          # Biner hasil kompilasi semua OS
│   ├── mad-dm-cli-mac-arm64
│   ├── mad-dm-cli-windows-amd64.exe
│   ├── mad-dm-cli-linux-amd64
│   ├── mad-dm-server-mac-arm64
│   ├── mad-dm-server-windows-amd64.exe
│   └── mad-dm-server-linux-amd64
├── engine/                       # Core Downloader Engine (Go)
│   ├── cmd/
│   │   ├── cli/                  # Standalone CLI tool
│   │   └── server/               # Daemon API & Web Dashboard
│   ├── pkg/
│   │   ├── downloader/           # Multi-part chunker & task lifecycle
│   │   ├── state/                # State persistence & recovery
│   │   └── api/                  # REST API, SSE & Embedded Web GUI
├── extension/                    # Chrome / Firefox Extension (Manifest V3)
│   ├── manifest.json
│   ├── background.js             # Context menu & download interceptor
│   ├── content.js                # Video sniffer & floating download button
│   └── popup.html                # Quick download popup
├── ui/                           # Electron Desktop Window Wrapper
├── scripts/
│   └── build.sh                  # Multi-OS Cross-compiler script
└── Makefile
```

---

## 🚀 Cara Menjalankan

### 1. Menggunakan CLI (Command-Line)
Biner CLI sudah dikompilasi di folder `bin/`:

```bash
# macOS (Apple Silicon):
./bin/mad-dm-cli-mac-arm64 -c 8 -d ./downloads "https://example.com/file.zip"

# Windows:
mad-dm-cli-windows-amd64.exe -c 8 -d ./downloads "https://example.com/file.zip"

# Linux:
./bin/mad-dm-cli-linux-amd64 -c 8 -d ./downloads "https://example.com/file.zip"
```

Opsi CLI:
* `-c <jumlah>`: Jumlah koneksi simultan (default 8)
* `-d <folder>`: Folder tujuan unduhan (default `./downloads`)
* `-o <nama_file>`: Nama file custom

---

### 2. Menjalankan Server & Web Dashboard GUI
Biner server sudah menyertakan Web UI di dalamnya:

```bash
./bin/mad-dm-server-mac-arm64 -port 17890
```

Buka browser di: **`http://localhost:17890`**  
Anda dapat melihat daftar unduhan, visualisasi segmen real-time, tombol pause/resume, dan menambahkan unduhan baru.

---

### 3. Memasang Ekstensi Browser (Chrome / Edge / Brave)
1. Buka browser dan pergi ke `chrome://extensions/`
2. Aktifkan **"Developer mode"** di pojok kanan atas.
3. Klik tombol **"Load unpacked"** dan pilih folder:  
   `/Users/rahmadrifai/Documents/project/mad-dm/extension`
4. Fitur ekstensi aktif:
   * Klik kanan pada tautan / video -> Pilih **"⚡ Download with MAD-DM"**
   * Saat memutar video di web apa pun, tombol melayang **"Download Video (MAD-DM)"** akan otomatis muncul di sudut player!

---

### 4. Cross-Compile Ulang ke Semua OS (Mac, Win, Linux)
Kapan pun ada perubahan kode, jalankan:

```bash
make build-all
# atau
./scripts/build.sh
```

Ini akan meng-compile ulang biner untuk Windows (`.exe`), Linux 64-bit, macOS Intel, dan macOS Apple Silicon dalam hitungan detik.

---

### 5. Membuat Installer Windows (.exe) di Komputer Windows
Jika Anda melakukan `git clone` di komputer Windows:

**Cara Otomatis (1-Klik):**
1. Pastikan **Node.js (LTS)** sudah terinstall dari [nodejs.org](https://nodejs.org/).
2. Cukup klik ganda (double-click) file **`build-installer-windows.bat`**.
3. Script otomatis akan:
   - Mengompilasi engine Go server ke `bin\mad-dm-server-windows-amd64.exe` (jika ada Go).
   - Mengunduh dependensi `yt-dlp.exe` & `ffmpeg.exe` ke folder `bin\` jika belum ada.
   - Menjalankan `npm install` dan `electron-builder` di folder `ui\`.
   - Menghasilkan installer `ui\dist\MAD-DM Setup 1.0.0.exe` dan langsung membuka foldernya di Windows Explorer.

**Cara Manual via Command Prompt:**
```cmd
cd ui
npm install
npx electron-builder --win --x64
```
Hasil installer akan berada di folder `ui\dist\`.
