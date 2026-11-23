@echo off
setlocal enabledelayedexpansion
title MAD-DM - Windows Installer Builder
cd /d "%~dp0"

echo =======================================================
echo    ⚡ MAD-DM Desktop Windows Installer Builder
echo =======================================================
echo.

:: 1. Periksa Node.js & npm
where node >nul 2>nul
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Node.js tidak terdeteksi!
    echo Silakan unduh dan install Node.js ^(versi LTS^) dari https://nodejs.org/
    echo Setelah selesai install, buka kembali Command Prompt dan jalankan script ini.
    echo.
    pause
    exit /b 1
)

where npm >nul 2>nul
if %ERRORLEVEL% neq 0 (
    echo [ERROR] npm tidak terdeteksi! Pastikan Node.js terinstall lengkap.
    pause
    exit /b 1
)

echo [1/5] Node.js dan npm terdeteksi:
call node -v
call npm -v
echo.

:: 2. Buat direktori bin jika belum ada
if not exist "bin" mkdir "bin"

:: 3. Kompilasi Engine Go Server
echo [2/5] Memeriksa Engine Go Server...
where go >nul 2>nul
if %ERRORLEVEL% equ 0 goto compile_go
if exist "bin\mad-dm-server-windows-amd64.exe" goto go_ready

echo [INFO] Golang tidak terdeteksi di PATH sistem.
where winget >nul 2>nul
if %ERRORLEVEL% equ 0 (
    echo Windows Package Manager ^(winget^) terdeteksi.
    set /p INSTALL_GO="Ingin menginstall Golang secara otomatis sekarang via winget? [Y/N]: "
    if /i "!INSTALL_GO!"=="Y" (
        echo.
        echo Mengunduh dan memasang Golang... Mohon tunggu sebentar.
        winget install --id GoLang.Go -e --accept-source-agreements --accept-package-agreements
        echo.
        echo [SELESAI] Golang telah dipasang!
        echo Silakan TUTUP Command Prompt ini dan KLIK KEMBALI build-installer-windows.bat agar PATH baru terbaca.
        pause
        exit /b 0
    )
)

echo.
echo Pilihan solusi:
echo 1. Unduh installer resmi Golang dari: https://go.dev/dl/
echo    ^(Pilih go1.x.x.windows-amd64.msi, install sampai selesai, lalu jalankan file .bat ini lagi^).
echo 2. ATAU salin file 'MAD-DM Setup 1.0.0.exe' langsung dari Mac Anda tanpa perlu build!
echo.
pause
exit /b 1

:compile_go
echo    Golang terdeteksi. Mengompilasi engine Go server ^& CLI untuk Windows x64...
cd engine
set CGO_ENABLED=0
set GOOS=windows
set GOARCH=amd64
go build -ldflags="-s -w" -o "..\bin\mad-dm-server-windows-amd64.exe" .\cmd\server
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Kompilasi Go server gagal.
    cd ..
    pause
    exit /b 1
)
go build -ldflags="-s -w" -o "..\bin\mad-dm-cli-windows-amd64.exe" .\cmd\cli
cd ..
echo    -^> Engine Go server berhasil dikompilasi ke bin\mad-dm-server-windows-amd64.exe

:go_ready
if exist "bin\mad-dm-server-windows-amd64.exe" (
    echo    -^> Engine Go siap di bin\mad-dm-server-windows-amd64.exe
)
echo.

:: 4. Periksa / Download yt-dlp.exe
echo [3/5] Memeriksa yt-dlp.exe di folder bin...
if exist "bin\yt-dlp.exe" goto ytdlp_done

echo    yt-dlp.exe belum ada, mengunduh biner resmi dari GitHub releases...
powershell -NoProfile -ExecutionPolicy Bypass -Command "[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12; (New-Object Net.WebClient).DownloadFile('https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp.exe', 'bin\yt-dlp.exe')"

if not exist "bin\yt-dlp.exe" (
    echo [WARNING] Gagal mengunduh yt-dlp.exe secara otomatis.
    echo Anda dapat meletakkan file yt-dlp.exe secara manual di folder bin\
) else (
    echo    -^> yt-dlp.exe berhasil diunduh ke folder bin\
)

:ytdlp_done
echo    -^> yt-dlp.exe siap di folder bin\
echo.

:: 5. Periksa / Download ffmpeg.exe
echo [4/5] Memeriksa ffmpeg.exe di folder bin...
if exist "bin\ffmpeg.exe" goto ffmpeg_done

where ffmpeg >nul 2>nul
if %ERRORLEVEL% equ 0 (
    echo    ffmpeg ditemukan di PATH sistem, menyalin ke folder bin...
    for /f "delims=" %%I in ('where ffmpeg') do (
        if not exist "bin\ffmpeg.exe" copy "%%I" "bin\ffmpeg.exe" >nul
    )
)

if exist "bin\ffmpeg.exe" goto ffmpeg_done

echo    ffmpeg.exe belum ada di folder bin, mengunduh paket FFmpeg Windows...
powershell -NoProfile -ExecutionPolicy Bypass -Command "[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12; Write-Host '   Downloading FFmpeg archive...'; (New-Object Net.WebClient).DownloadFile('https://github.com/yt-dlp/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl.zip', 'bin\ffmpeg.zip'); Write-Host '   Extracting ffmpeg.exe...'; Expand-Archive -Path 'bin\ffmpeg.zip' -DestinationPath 'bin\ffmpeg_temp' -Force; $ff = Get-ChildItem -Path 'bin\ffmpeg_temp' -Filter 'ffmpeg.exe' -Recurse | Select-Object -First 1; if ($ff) { Move-Item -Path $ff.FullName -Destination 'bin\ffmpeg.exe' -Force }; Remove-Item 'bin\ffmpeg_temp' -Recurse -Force; Remove-Item 'bin\ffmpeg.zip' -Force"

if not exist "bin\ffmpeg.exe" (
    echo [WARNING] Gagal mengunduh otomatis ffmpeg.exe.
    echo Anda dapat meletakkan file ffmpeg.exe secara manual di folder bin\
) else (
    echo    -^> ffmpeg.exe berhasil diekstrak ke folder bin\
)

:ffmpeg_done
echo    -^> ffmpeg.exe siap di folder bin\
echo.

:: 6. Build Electron Desktop Installer
echo [5/5] Menginstal dependencies dan membangun installer Windows...
cd ui
echo    Menjalankan npm install...
call npm install
if %ERRORLEVEL% neq 0 (
    echo [ERROR] npm install gagal. Periksa koneksi internet Anda.
    cd ..
    pause
    exit /b 1
)

echo.
echo    Menjalankan electron-builder untuk membuat Installer Windows...
set CSC_IDENTITY_AUTO_DISCOVERY=false
set WIN_CSC_LINK=
call npx electron-builder --win --x64
if %ERRORLEVEL% neq 0 (
    echo.
    echo ===================================================================
    echo [ERROR] electron-builder gagal mengekstrak paket winCodeSign.
    echo.
    echo Penyebab: Windows membatasi hak akses pembuatan symbolic link.
    echo.
    echo SOLUSI CEPAT:
    echo 1. Tutup jendela CMD ini.
    echo 2. KLIK KANAN file 'build-installer-windows.bat'
    echo    lalu pilih: 'Run as administrator' (Jalankan sebagai administrator).
    echo.
    echo ATAU aktifkan Developer Mode:
    echo    Buka Windows Settings -^> Privacy & Security -^> For developers -^> ON.
    echo ===================================================================
    cd ..
    pause
    exit /b 1
)
cd ..

echo.
echo =======================================================
echo    🎉 SUKSES! INSTALLER WINDOWS BERHASIL DIBUAT!
echo =======================================================
echo.
echo File installer telah dibuat dan tersimpan di:
echo    - NSIS Installer (.exe) : ui\dist\MAD-DM Setup 1.0.0.exe
echo    - Standalone Portable   : ui\dist\MAD-DM 1.0.0.exe
echo.

if exist "ui\dist\MAD-DM Setup 1.0.0.exe" (
    echo Membuka folder output ui\dist di Windows Explorer...
    explorer.exe "ui\dist"
)

pause
