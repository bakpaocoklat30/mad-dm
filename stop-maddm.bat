@echo off
title MAD-DM Process Terminator
color 0c
echo ======================================================================
echo   ⚡ MAD-DM Process Terminator (Force Close Engine & GUI) ⚡
echo ======================================================================
echo.
echo Menghentikan seluruh proses MAD-DM yang sedang berjalan di Windows...
echo.

taskkill /F /IM "MAD-DM.exe" /T >nul 2>&1
taskkill /F /IM "mad-dm-server-windows-amd64.exe" /T >nul 2>&1
taskkill /F /IM "yt-dlp.exe" /T >nul 2>&1
taskkill /F /IM "ffmpeg.exe" /T >nul 2>&1

echo [OK] Seluruh proses MAD-DM telah dihentikan secara tuntas.
echo File lock telah dilepas. Anda sekarang dapat menginstal pembaruan MAD-DM!
echo.
pause
