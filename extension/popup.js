const ENGINE_API = "http://localhost:17890";

const statusBadge = document.getElementById("statusBadge");
const statusText = document.getElementById("statusText");
const urlInput = document.getElementById("urlInput");
const downloadBtn = document.getElementById("downloadBtn");
const autoInterceptToggle = document.getElementById("autoInterceptToggle");
const interceptStatusDesc = document.getElementById("interceptStatusDesc");
const mediaContainer = document.getElementById("mediaContainer");
const mediaCount = document.getElementById("mediaCount");
const openOptionsBtn = document.getElementById("openOptionsBtn");
const openSettingsLink = document.getElementById("openSettingsLink");
const extFormatSelect = document.getElementById("extFormatSelect");
const extResSelect = document.getElementById("extResSelect");

if (extFormatSelect && extResSelect) {
  extFormatSelect.addEventListener("change", () => {
    extResSelect.style.display = (extFormatSelect.value === "mp4") ? "block" : "none";
  });
}

// 1. Check engine health
async function checkEngine() {
  try {
    const res = await fetch(`${ENGINE_API}/api/health`);
    if (res.ok) {
      statusBadge.className = "status-badge online";
      statusText.textContent = "ONLINE";
    } else {
      throw new Error();
    }
  } catch (err) {
    statusBadge.className = "status-badge offline";
    statusText.textContent = "OFFLINE";
  }
}

// 2. Fetch detected media for current tab
async function loadTabMedia() {
  chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
    if (!tabs || tabs.length === 0) return;
    const tabId = tabs[0].id;

    chrome.runtime.sendMessage({ type: "GET_TAB_MEDIA", tabId }, (res) => {
      if (res && res.media && res.media.length > 0) {
        mediaCount.textContent = `${res.media.length} FOUND`;
        mediaContainer.innerHTML = res.media.map((item, idx) => {
          let cleanName = item.url.split("/").pop().split("?")[0] || `Stream_${idx + 1}`;
          if (cleanName.length > 25) cleanName = cleanName.substring(0, 22) + "...";
          return `
            <div class="media-item">
              <span class="media-name" title="${item.url}">${cleanName}</span>
              <button class="btn-sniff-dl" data-url="${item.url}">GRAB</button>
            </div>
          `;
        }).join("");

        mediaContainer.querySelectorAll(".btn-sniff-dl").forEach((btn) => {
          btn.addEventListener("click", () => {
            const mediaUrl = btn.getAttribute("data-url");
            const fmt = extFormatSelect ? extFormatSelect.value : "mp4";
            const res = extResSelect ? extResSelect.value : "best";
            btn.textContent = "QUEUED...";
            btn.disabled = true;
            chrome.runtime.sendMessage({ type: "DOWNLOAD_URL", url: mediaUrl, format: fmt, resolution: res });
          });
        });
      } else {
        mediaCount.textContent = "0 FOUND";
        mediaContainer.innerHTML = '<div class="empty-media">No streamable media detected on this tab.</div>';
      }
    });
  });
}

// 3. Auto Intercept Toggle (Default: false)
chrome.storage.local.get({ autoIntercept: false }, (data) => {
  autoInterceptToggle.checked = data.autoIntercept;
  updateInterceptDesc(data.autoIntercept);
});

function updateInterceptDesc(enabled) {
  if (enabled) {
    interceptStatusDesc.innerHTML = `<span style="color: #00f0ff; font-weight: 700;">ACTIVE</span> • MAD-DM captures normal file downloads`;
  } else {
    interceptStatusDesc.innerHTML = `<span style="color: #94a3b8; font-weight: 700;">DISABLED</span> • Browser handles normal downloads`;
  }
}

autoInterceptToggle.addEventListener("change", (e) => {
  const isEnabled = e.target.checked;
  chrome.storage.local.set({ autoIntercept: isEnabled }, () => {
    updateInterceptDesc(isEnabled);
  });
});

// 4. Quick Download
downloadBtn.addEventListener("click", () => {
  const url = urlInput.value.trim();
  if (!url) return;

  const fmt = extFormatSelect ? extFormatSelect.value : "mp4";
  const res = extResSelect ? extResSelect.value : "best";

  downloadBtn.disabled = true;
  downloadBtn.textContent = "DISPATCHING...";

  chrome.runtime.sendMessage({ type: "DOWNLOAD_URL", url, format: fmt, resolution: res }, (res) => {
    downloadBtn.disabled = false;
    downloadBtn.textContent = "⚡ DOWNLOAD (16 THREADS)";
    if (res && res.success) {
      urlInput.value = "";
    }
  });
});

// 5. Open Settings Page
function openOptions(e) {
  if (e) e.preventDefault();
  if (chrome.runtime.openOptionsPage) {
    chrome.runtime.openOptionsPage();
  } else {
    window.open(chrome.runtime.getURL("options.html"));
  }
}

openOptionsBtn.addEventListener("click", openOptions);
openSettingsLink.addEventListener("click", openOptions);

checkEngine();
loadTabMedia();
