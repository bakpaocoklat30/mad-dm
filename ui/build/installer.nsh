; MAD-DM NSIS Custom Installer Script
; Automatically terminates old processes before install/update, and registers Chrome & Edge extensions

!macro customInit
  DetailPrint "Menutup proses MAD-DM yang sedang berjalan..."
  nsExec::Exec 'taskkill /F /IM "MAD-DM.exe" /T'
  nsExec::Exec 'taskkill /F /IM "mad-dm-server-windows-amd64.exe" /T'
  nsExec::Exec 'taskkill /F /IM "yt-dlp.exe" /T'
  nsExec::Exec 'taskkill /F /IM "ffmpeg.exe" /T'
  Sleep 1000
!macroend

!macro customInstall
  DetailPrint "Registering MAD-DM Browser Integration Extension in Windows Registry..."

  ; Google Chrome Extension Registry
  WriteRegStr HKCU "Software\Google\Chrome\Extensions\iadpkcpkmlgajpdffbfodnkighihdflf" "path" "$INSTDIR\resources\extension"
  WriteRegStr HKCU "Software\Google\Chrome\Extensions\iadpkcpkmlgajpdffbfodnkighihdflf" "version" "1.0.0"

  ; Microsoft Edge Extension Registry
  WriteRegStr HKCU "Software\Microsoft\Edge\Extensions\iadpkcpkmlgajpdffbfodnkighihdflf" "path" "$INSTDIR\resources\extension"
  WriteRegStr HKCU "Software\Microsoft\Edge\Extensions\iadpkcpkmlgajpdffbfodnkighihdflf" "version" "1.0.0"

  ; Brave Browser Extension Registry
  WriteRegStr HKCU "Software\BraveSoftware\Brave-Browser\Extensions\iadpkcpkmlgajpdffbfodnkighihdflf" "path" "$INSTDIR\resources\extension"
  WriteRegStr HKCU "Software\BraveSoftware\Brave-Browser\Extensions\iadpkcpkmlgajpdffbfodnkighihdflf" "version" "1.0.0"

  ; Enterprise Policy Force-Install (Automatically enables extension without manual drag-and-drop)
  WriteRegStr HKCU "Software\Policies\Google\Chrome\ExtensionInstallForcelist" "991" "iadpkcpkmlgajpdffbfodnkighihdflf;$INSTDIR\resources\extension"
  WriteRegStr HKCU "Software\Policies\Microsoft\Edge\ExtensionInstallForcelist" "991" "iadpkcpkmlgajpdffbfodnkighihdflf;$INSTDIR\resources\extension"
!macroend

!macro customUnInit
  nsExec::Exec 'taskkill /F /IM "MAD-DM.exe" /T'
  nsExec::Exec 'taskkill /F /IM "mad-dm-server-windows-amd64.exe" /T'
  Sleep 1000
!macroend

!macro customUnInstall
  ; Cleanup on Uninstall
  DeleteRegKey HKCU "Software\Google\Chrome\Extensions\iadpkcpkmlgajpdffbfodnkighihdflf"
  DeleteRegKey HKCU "Software\Microsoft\Edge\Extensions\iadpkcpkmlgajpdffbfodnkighihdflf"
  DeleteRegKey HKCU "Software\BraveSoftware\Brave-Browser\Extensions\iadpkcpkmlgajpdffbfodnkighihdflf"
  DeleteRegValue HKCU "Software\Policies\Google\Chrome\ExtensionInstallForcelist" "991"
  DeleteRegValue HKCU "Software\Policies\Microsoft\Edge\ExtensionInstallForcelist" "991"
!macroend
