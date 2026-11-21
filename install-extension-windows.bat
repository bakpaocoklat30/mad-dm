@echo off
title MAD-DM Browser Integration Installer
color 0b
echo ======================================================================
echo   ⚡ MAD-DM Accelerated Downloader - Browser Extension Installer ⚡
echo ======================================================================
echo.

set EXT_ID=iadpkcpkmlgajpdffbfodnkighihdflf

:: Detect extension folder
set EXT_DIR=%~dp0
if exist "%EXT_DIR%manifest.json" goto FOUND
set EXT_DIR=%~dp0extension\
if exist "%EXT_DIR%manifest.json" goto FOUND
set EXT_DIR=%~dp0resources\extension\
if exist "%EXT_DIR%manifest.json" goto FOUND
set EXT_DIR=%LOCALAPPDATA%\Programs\MAD-DM\resources\extension\
if exist "%EXT_DIR%manifest.json" goto FOUND
set EXT_DIR=C:\Program Files\MAD-DM\resources\extension\
if exist "%EXT_DIR%manifest.json" goto FOUND

echo [!] Could not automatically locate extension directory.
echo Please run this script from the MAD-DM root directory or extension folder.
pause
exit /b 1

:FOUND
echo [OK] Extension directory detected:
echo      %EXT_DIR%
echo.
echo Installing and registering extension in Windows Registry...
echo.

:: 1. Google Chrome (Extension Registry & Enterprise Policy)
echo [1/3] Registering in Google Chrome...
reg add "HKCU\Software\Google\Chrome\Extensions\%EXT_ID%" /v "path" /t REG_SZ /d "%EXT_DIR%" /f >nul 2>&1
reg add "HKCU\Software\Google\Chrome\Extensions\%EXT_ID%" /v "version" /t REG_SZ /d "1.0.0" /f >nul 2>&1
reg add "HKCU\Software\Policies\Google\Chrome\ExtensionInstallForcelist" /v "991" /t REG_SZ /d "%EXT_ID%;%EXT_DIR%" /f >nul 2>&1

:: 2. Microsoft Edge (Extension Registry & Enterprise Policy)
echo [2/3] Registering in Microsoft Edge...
reg add "HKCU\Software\Microsoft\Edge\Extensions\%EXT_ID%" /v "path" /t REG_SZ /d "%EXT_DIR%" /f >nul 2>&1
reg add "HKCU\Software\Microsoft\Edge\Extensions\%EXT_ID%" /v "version" /t REG_SZ /d "1.0.0" /f >nul 2>&1
reg add "HKCU\Software\Policies\Microsoft\Edge\ExtensionInstallForcelist" /v "991" /t REG_SZ /d "%EXT_ID%;%EXT_DIR%" /f >nul 2>&1

:: 3. Brave Browser
echo [3/3] Registering in Brave Browser...
reg add "HKCU\Software\BraveSoftware\Brave-Browser\Extensions\%EXT_ID%" /v "path" /t REG_SZ /d "%EXT_DIR%" /f >nul 2>&1
reg add "HKCU\Software\BraveSoftware\Brave-Browser\Extensions\%EXT_ID%" /v "version" /t REG_SZ /d "1.0.0" /f >nul 2>&1

echo.
echo ======================================================================
echo   🎉 SUCCESS! MAD-DM Extension successfully installed.
echo   Silakan restart Google Chrome atau Microsoft Edge Anda.
echo   Ikon MAD-DM akan langsung aktif di browser toolbar!
echo ======================================================================
echo.
pause
