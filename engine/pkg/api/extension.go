package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func FindExtensionDir() string {
	candidates := []string{}

	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidates = append(candidates,
			filepath.Join(exeDir, "..", "resources", "extension"),
			filepath.Join(exeDir, "..", "Resources", "extension"),
			filepath.Join(exeDir, "resources", "extension"),
			filepath.Join(exeDir, "extension"),
			filepath.Join(exeDir, "..", "extension"),
			filepath.Join(exeDir, "..", "..", "extension"),
		)
	}

	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(cwd, "extension"),
			filepath.Join(cwd, "..", "extension"),
			filepath.Join(cwd, "resources", "extension"),
			filepath.Join(cwd, "ui", "resources", "extension"),
		)
	}

	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates,
			filepath.Join(home, ".mad-dm", "extension"),
			filepath.Join(home, "AppData", "Local", "Programs", "MAD-DM", "resources", "extension"),
			"C:\\Program Files\\MAD-DM\\resources\\extension",
			"/Applications/MAD-DM.app/Contents/Resources/extension",
		)
	}

	for _, c := range candidates {
		manifest := filepath.Join(c, "manifest.json")
		if _, err := os.Stat(manifest); err == nil {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}

	return ""
}

func (s *Server) handleExtensionInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	extDir := FindExtensionDir()
	res := map[string]interface{}{
		"os":           runtime.GOOS,
		"found":        extDir != "",
		"path":         extDir,
		"extension_id": "iadpkcpkmlgajpdffbfodnkighihdflf",
	}
	_ = json.NewEncoder(w).Encode(res)
}

func (s *Server) handleExtensionInstall(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	extDir := FindExtensionDir()
	if extDir == "" {
		http.Error(w, "Extension directory not found on disk", http.StatusNotFound)
		return
	}

	extID := "iadpkcpkmlgajpdffbfodnkighihdflf"
	var messages []string

	if runtime.GOOS == "windows" {
		targets := []string{
			`Software\Google\Chrome\Extensions\` + extID,
			`Software\Microsoft\Edge\Extensions\` + extID,
			`Software\BraveSoftware\Brave-Browser\Extensions\` + extID,
		}
		for _, key := range targets {
			_ = exec.Command("reg", "add", "HKCU\\"+key, "/v", "path", "/t", "REG_SZ", "/d", extDir, "/f").Run()
			_ = exec.Command("reg", "add", "HKCU\\"+key, "/v", "version", "/t", "REG_SZ", "/d", "1.0.0", "/f").Run()
		}
		_ = exec.Command("reg", "add", "HKCU\\Software\\Policies\\Google\\Chrome\\ExtensionInstallForcelist", "/v", "991", "/t", "REG_SZ", "/d", extID+";"+extDir, "/f").Run()
		_ = exec.Command("reg", "add", "HKCU\\Software\\Policies\\Microsoft\\Edge\\ExtensionInstallForcelist", "/v", "991", "/t", "REG_SZ", "/d", extID+";"+extDir, "/f").Run()
		messages = append(messages, "Ekstensi berhasil didaftarkan ke Windows Registry untuk Chrome, Edge, dan Brave.")
	} else if runtime.GOOS == "darwin" {
		messages = append(messages, "macOS: Gunakan tombol 'Load Unpacked' di chrome://extensions atau buka folder ekstensi.")
	} else {
		messages = append(messages, "Linux: Gunakan tombol 'Load Unpacked' di chrome://extensions.")
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"os":       runtime.GOOS,
		"path":     extDir,
		"messages": messages,
	})
}

func (s *Server) handleExtensionReveal(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	extDir := FindExtensionDir()
	if extDir == "" {
		http.Error(w, "Extension directory not found", http.StatusNotFound)
		return
	}

	manifest := filepath.Join(extDir, "manifest.json")
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer.exe", "/select,"+manifest)
	case "darwin":
		cmd = exec.Command("open", "-R", manifest)
	default:
		cmd = exec.Command("xdg-open", extDir)
	}

	if err := cmd.Start(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to reveal folder: %v", err), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"path":    extDir,
	})
}

func (s *Server) handleExtensionLaunch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	extDir := FindExtensionDir()
	if extDir == "" {
		http.Error(w, "Extension directory not found", http.StatusNotFound)
		return
	}

	var req struct {
		Browser string `json:"browser"` // "chrome", "edge", "brave"
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	targetBrowser := strings.ToLower(req.Browser)
	if targetBrowser == "" {
		targetBrowser = "chrome"
	}

	var browserExe string
	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		programFiles := os.Getenv("ProgramFiles")
		programFilesX86 := os.Getenv("ProgramFiles(x86)")
		if programFiles == "" {
			programFiles = "C:\\Program Files"
		}
		if programFilesX86 == "" {
			programFilesX86 = "C:\\Program Files (x86)"
		}

		switch targetBrowser {
		case "edge":
			candidates := []string{
				filepath.Join(programFilesX86, "Microsoft", "Edge", "Application", "msedge.exe"),
				filepath.Join(programFiles, "Microsoft", "Edge", "Application", "msedge.exe"),
			}
			for _, c := range candidates {
				if _, err := os.Stat(c); err == nil {
					browserExe = c
					break
				}
			}
		case "brave":
			candidates := []string{
				filepath.Join(programFiles, "BraveSoftware", "Brave-Browser", "Application", "brave.exe"),
				filepath.Join(localAppData, "BraveSoftware", "Brave-Browser", "Application", "brave.exe"),
			}
			for _, c := range candidates {
				if _, err := os.Stat(c); err == nil {
					browserExe = c
					break
				}
			}
		default: // chrome
			candidates := []string{
				filepath.Join(programFiles, "Google", "Chrome", "Application", "chrome.exe"),
				filepath.Join(programFilesX86, "Google", "Chrome", "Application", "chrome.exe"),
				filepath.Join(localAppData, "Google", "Chrome", "Application", "chrome.exe"),
			}
			for _, c := range candidates {
				if _, err := os.Stat(c); err == nil {
					browserExe = c
					break
				}
			}
		}
	} else if runtime.GOOS == "darwin" {
		switch targetBrowser {
		case "edge":
			browserExe = "/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge"
		case "brave":
			browserExe = "/Applications/Brave Browser.app/Contents/MacOS/Brave Browser"
		default:
			browserExe = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
		}
	} else {
		switch targetBrowser {
		case "edge":
			browserExe = "microsoft-edge"
		case "brave":
			browserExe = "brave-browser"
		default:
			browserExe = "google-chrome"
		}
	}

	if browserExe == "" {
		http.Error(w, fmt.Sprintf("Browser '%s' executable not found on system", targetBrowser), http.StatusNotFound)
		return
	}

	loadArg := fmt.Sprintf("--load-extension=%s", extDir)
	cmd := exec.Command(browserExe, loadArg, "chrome://extensions")
	if err := cmd.Start(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to launch browser: %v", err), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"browser": targetBrowser,
		"exe":     browserExe,
	})
}
