const { app, BrowserWindow, shell } = require('electron');
const path = require('path');
const os = require('os');
const fs = require('fs');
const { spawn } = require('child_process');
const http = require('http');

let mainWindow;
let serverProcess = null;
const SERVER_PORT = 17890;
const SERVER_URL = `http://127.0.0.1:${SERVER_PORT}`;

function isServerRunning() {
  return new Promise((resolve) => {
    const req = http.get(`${SERVER_URL}/api/health`, (res) => {
      resolve(res.statusCode === 200);
    });
    req.on('error', () => resolve(false));
    req.setTimeout(500, () => {
      req.destroy();
      resolve(false);
    });
  });
}

function getBinaryName() {
  if (process.platform === 'win32') {
    return 'mad-dm-server-windows-amd64.exe';
  } else if (process.platform === 'linux') {
    return 'mad-dm-server-linux-amd64';
  } else if (process.platform === 'darwin') {
    return process.arch === 'arm64' ? 'mad-dm-server-mac-arm64' : 'mad-dm-server-mac-intel';
  }
  return 'mad-dm-server';
}

function getBinaryPath() {
  const binaryName = getBinaryName();

  // 1. Packaged extraResources path
  if (process.resourcesPath) {
    const packagedPath = path.join(process.resourcesPath, 'bin', binaryName);
    if (fs.existsSync(packagedPath)) return packagedPath;
  }

  // 2. Dev relative path
  const devPath = path.join(__dirname, '..', 'bin', binaryName);
  if (fs.existsSync(devPath)) return devPath;

  return binaryName;
}

async function startEngine() {
  if (await isServerRunning()) {
    console.log('[MAD-DM] Engine daemon already active on', SERVER_URL);
    return;
  }

  const binaryPath = getBinaryPath();
  console.log('[MAD-DM] Launching engine binary:', binaryPath);

  // Ensure executable permission
  try {
    fs.chmodSync(binaryPath, 0o755);
  } catch (e) {}

  const stateDir = path.join(os.homedir(), '.mad-dm');
  const downloadsDir = path.join(os.homedir(), 'Downloads');
  fs.mkdirSync(stateDir, { recursive: true });

  const logFile = fs.openSync(path.join(stateDir, 'engine.log'), 'a');

  try {
    serverProcess = spawn(binaryPath, [
      '-port', String(SERVER_PORT),
      '-dir', downloadsDir,
      '-state', stateDir
    ], {
      cwd: os.homedir(),
      stdio: ['ignore', logFile, logFile],
      detached: false
    });

    serverProcess.on('error', (err) => {
      console.error('[MAD-DM] Engine spawn error:', err);
    });

    // Wait up to 3 seconds for server to respond
    for (let i = 0; i < 30; i++) {
      await new Promise(r => setTimeout(r, 100));
      if (await isServerRunning()) {
        console.log('[MAD-DM] Engine ready and listening!');
        break;
      }
    }
  } catch (err) {
    console.error('[MAD-DM] Failed to launch server process:', err);
  }
}

function registerWindowsExtension() {
  if (process.platform !== 'win32') return;

  let extDir = '';
  if (process.resourcesPath) {
    const candidate = path.join(process.resourcesPath, 'extension');
    if (fs.existsSync(candidate)) extDir = candidate;
  }
  if (!extDir) {
    const devPath = path.join(__dirname, '..', 'extension');
    if (fs.existsSync(devPath)) extDir = devPath;
  }

  if (!extDir) return;

  const extId = 'iadpkcpkmlgajpdffbfodnkighihdflf';
  console.log('[MAD-DM] Auto-registering browser extension for Windows at:', extDir);

  const targets = [
    `Software\\Google\\Chrome\\Extensions\\${extId}`,
    `Software\\Microsoft\\Edge\\Extensions\\${extId}`,
    `Software\\BraveSoftware\\Brave-Browser\\Extensions\\${extId}`
  ];

  targets.forEach((key) => {
    try {
      spawn('reg', ['add', `HKCU\\${key}`, '/v', 'path', '/t', 'REG_SZ', '/d', extDir, '/f'], { stdio: 'ignore' });
      spawn('reg', ['add', `HKCU\\${key}`, '/v', 'version', '/t', 'REG_SZ', '/d', '1.0.0', '/f'], { stdio: 'ignore' });
    } catch (e) {}
  });

  try {
    spawn('reg', ['add', 'HKCU\\Software\\Policies\\Google\\Chrome\\ExtensionInstallForcelist', '/v', '991', '/t', 'REG_SZ', '/d', `${extId};${extDir}`, '/f'], { stdio: 'ignore' });
    spawn('reg', ['add', 'HKCU\\Software\\Policies\\Microsoft\\Edge\\ExtensionInstallForcelist', '/v', '991', '/t', 'REG_SZ', '/d', `${extId};${extDir}`, '/f'], { stdio: 'ignore' });
  } catch (e) {}
}

async function createWindow() {
  // Ensure engine daemon is up and running first
  await startEngine();
  registerWindowsExtension();

  mainWindow = new BrowserWindow({
    width: 1020,
    height: 680,
    minWidth: 840,
    minHeight: 540,
    title: 'MAD-DM | Hyper-Speed Multi-Thread Downloader',
    backgroundColor: '#040711',
    show: false,
    webPreferences: {
      nodeIntegration: false,
      contextIsolation: true
    }
  });

  // Load local HTML instantly (0ms load time)
  mainWindow.loadFile(path.join(__dirname, 'index.html'));

  mainWindow.once('ready-to-show', () => {
    mainWindow.show();
  });

  // External links open in browser
  mainWindow.webContents.setWindowOpenHandler(({ url }) => {
    shell.openExternal(url);
    return { action: 'deny' };
  });

  // Shortcut to toggle DevTools (Cmd+Option+I or Ctrl+Shift+I)
  mainWindow.webContents.on('before-input-event', (event, input) => {
    if ((input.key === 'I' || input.key === 'i') && (input.control || input.meta) && input.alt) {
      mainWindow.webContents.toggleDevTools();
      event.preventDefault();
    }
    if (input.key === 'F12') {
      mainWindow.webContents.toggleDevTools();
      event.preventDefault();
    }
  });

  mainWindow.on('closed', () => {
    mainWindow = null;
  });
}

function stopEngine() {
  if (serverProcess) {
    try {
      if (process.platform === 'win32') {
        const { execSync } = require('child_process');
        try { execSync(`taskkill /pid ${serverProcess.pid} /T /F`, { stdio: 'ignore' }); } catch (e) {}
        try { execSync(`taskkill /F /IM mad-dm-server-windows-amd64.exe /T`, { stdio: 'ignore' }); } catch (e) {}
        try { execSync(`taskkill /F /IM yt-dlp.exe /T`, { stdio: 'ignore' }); } catch (e) {}
        try { execSync(`taskkill /F /IM ffmpeg.exe /T`, { stdio: 'ignore' }); } catch (e) {}
      } else {
        serverProcess.kill('SIGTERM');
        serverProcess.kill('SIGKILL');
      }
    } catch (e) {}
    serverProcess = null;
  }
}

app.whenReady().then(createWindow);

app.on('window-all-closed', () => {
  stopEngine();
  app.quit();
});

app.on('before-quit', () => {
  stopEngine();
});

app.on('will-quit', () => {
  stopEngine();
});

process.on('exit', () => {
  stopEngine();
});

app.on('activate', () => {
  if (BrowserWindow.getAllWindows().length === 0) createWindow();
});
