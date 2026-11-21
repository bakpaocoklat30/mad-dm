const ENGINE_API = "http://localhost:17890";

// In-memory media store per tab (TabId -> Array of Media URLs)
const tabMedia = new Map();

// Context Menu: "Download with MAD-DM"
chrome.runtime.onInstalled.addListener(() => {
  chrome.contextMenus.create({
    id: "mad-dm-download-link",
    title: "⚡ Download with MAD-DM (16 Threads)",
    contexts: ["link", "image", "video", "audio"]
  });
});

chrome.contextMenus.onClicked.addListener((info, tab) => {
  const targetURL = info.linkUrl || info.srcUrl;
  if (targetURL) {
    sendToEngine(targetURL);
  }
});

// Sniff multimedia streams via webRequest
chrome.webRequest.onHeadersReceived.addListener(
  (details) => {
    if (details.tabId < 0) return;

    const url = details.url;
    let contentType = "";
    let contentLength = 0;

    for (const h of details.responseHeaders || []) {
      const name = h.name.toLowerCase();
      if (name === "content-type") contentType = h.value.toLowerCase();
      if (name === "content-length") contentLength = parseInt(h.value, 10);
    }

    const isMedia =
      contentType.includes("video/") ||
      contentType.includes("audio/") ||
      contentType.includes("application/vnd.apple.mpegurl") ||
      contentType.includes("application/x-mpegurl") ||
      url.match(/\.(mp4|mkv|m3u8|webm|ts|mov|avi|flv|mp3|zip|iso|dmg)(\?|$)/i);

    if (isMedia && !url.startsWith("blob:") && !url.startsWith("data:")) {
      let list = tabMedia.get(details.tabId) || [];
      if (!list.some((item) => item.url === url)) {
        list.push({
          url: url,
          type: contentType || "media",
          size: contentLength || 0,
          timestamp: Date.now()
        });
        tabMedia.set(details.tabId, list);

        // Update badge count on extension icon
        chrome.action.setBadgeText({
          tabId: details.tabId,
          text: String(list.length)
        });
        chrome.action.setBadgeBackgroundColor({
          tabId: details.tabId,
          color: "#00f0ff"
        });
      }
    }
  },
  { urls: ["<all_urls>"] },
  ["responseHeaders"]
);

// Clear tab media on tab close
chrome.tabs.onRemoved.addListener((tabId) => {
  tabMedia.delete(tabId);
});

// Intercept browser downloads ONLY if user has explicitly enabled auto-intercept
chrome.downloads.onDeterminingFilename.addListener((item, suggest) => {
  chrome.storage.local.get({ autoIntercept: false, filterExtensions: false, extensionList: "zip,iso,dmg,exe,tar,gz,rar,7z,mp4,mkv" }, (res) => {
    // If auto-intercept is OFF, let browser handle normally!
    if (!res.autoIntercept) {
      return;
    }

    if (!item.url || item.url.startsWith("blob:") || item.url.startsWith("data:")) {
      return;
    }

    // Optional filter by extension
    if (res.filterExtensions && res.extensionList) {
      const allowed = res.extensionList.split(",").map(e => e.trim().toLowerCase());
      const ext = (item.filename.split(".").pop() || "").toLowerCase();
      if (!allowed.includes(ext)) {
        return; // Skip non-matching file types
      }
    }

    chrome.downloads.cancel(item.id, () => {
      sendToEngine(item.url, item.filename);
    });
  });
});

// Message bus
chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
  if (msg.type === "DOWNLOAD_URL") {
    sendToEngine(msg.url, msg.filename, msg.format || "", msg.isPlaylist || false, msg.merge || false)
      .then((data) => sendResponse({ success: true, data }))
      .catch((err) => sendResponse({ success: false, error: err.message }));
    return true;
  }

  if (msg.type === "GET_TAB_MEDIA") {
    const list = tabMedia.get(msg.tabId) || [];
    sendResponse({ media: list });
    return true;
  }
});

async function sendToEngine(url, filename = "", format = "", isPlaylist = false, merge = false) {
  try {
    let endpoint = `${ENGINE_API}/api/tasks`;
    let body = {
      url: url,
      connections: 16,
      format: format || "",
      auto_start: true
    };

    if (isPlaylist || (url.includes("list=") && isPlaylist)) {
      endpoint = `${ENGINE_API}/api/playlist/download`;
      body.merge = merge;
      body.keep_parts = true;
    }

    const res = await fetch(endpoint, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body)
    });

    if (!res.ok) {
      throw new Error(`Engine HTTP ${res.status}`);
    }

    const data = await res.json();
    showNotification(
      "⚡ MAD-DM Accelerated",
      merge
        ? `Playlist queued & will be merged into 1 ${format.toUpperCase()} file!`
        : (isPlaylist
            ? `Queued playlist tracks in ${format.toUpperCase()} format!`
            : `Download captured: ${data.filename || filename || "New File"} [${format.toUpperCase()}]`)
    );
    return data;
  } catch (err) {
    showNotification(
      "❌ MAD-DM Engine Offline",
      "Please ensure MAD-DM Desktop App is open on localhost:17890"
    );
    throw err;
  }
}

function showNotification(title, message) {
  if (chrome.notifications) {
    chrome.notifications.create({
      type: "basic",
      iconUrl: "icons/icon128.png",
      title: title,
      message: message
    });
  }
}
