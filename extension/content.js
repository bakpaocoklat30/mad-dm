// MAD-DM Video Sniffer & Floating Download Button
(function () {
  const processedContainers = new WeakSet();

  function isYouTube() {
    return window.location.hostname.includes("youtube.com") || window.location.hostname.includes("youtu.be");
  }

  function isStreamingHost() {
    const host = window.location.hostname;
    return (
      host.includes("youtube.com") ||
      host.includes("youtu.be") ||
      host.includes("tiktok.com") ||
      host.includes("instagram.com") ||
      host.includes("twitter.com") ||
      host.includes("x.com") ||
      host.includes("facebook.com") ||
      host.includes("vimeo.com")
    );
  }

  function scanVideos() {
    // 1. YouTube specific handler
    if (isYouTube()) {
      const player = document.querySelector("#movie_player, .html5-video-player");
      const video = document.querySelector("video");
      if (player && video && !player.querySelector(".mad-dm-badge-container")) {
        attachDownloadBadge(player, window.location.href, true);
      }
      return;
    }

    // 2. Generic HTML5 video handler
    const videos = document.querySelectorAll("video");
    videos.forEach((video) => {
      const parent = video.parentElement || video;
      if (processedContainers.has(parent) || parent.querySelector(".mad-dm-badge-container")) return;

      let src = video.currentSrc || video.src || getSourceTag(video);

      // On streaming hosts (TikTok, Twitter, etc.), use page URL if blob is used
      if ((!src || src.startsWith("blob:")) && isStreamingHost()) {
        src = window.location.href;
      }

      if (src && !src.startsWith("blob:") && !src.startsWith("data:")) {
        attachDownloadBadge(video, src, false);
        processedContainers.add(parent);
      }

      video.addEventListener("play", () => {
        let dynamicSrc = video.currentSrc || video.src || getSourceTag(video);
        if ((!dynamicSrc || dynamicSrc.startsWith("blob:")) && isStreamingHost()) {
          dynamicSrc = window.location.href;
        }
        if (dynamicSrc && !parent.querySelector(".mad-dm-badge-container")) {
          attachDownloadBadge(video, dynamicSrc, false);
          processedContainers.add(parent);
        }
      });
    });
  }

  function getSourceTag(video) {
    const source = video.querySelector("source");
    return source ? source.src : null;
  }

  function attachDownloadBadge(container, url, isYT) {
    chrome.storage.local.get({ showFloatingBadge: true }, (res) => {
      if (!res.showFloatingBadge) return;

      const parent = container.tagName === "VIDEO" ? container.parentElement : container;
      if (!parent || parent.querySelector(".mad-dm-badge-container")) return;

      const computedStyle = window.getComputedStyle(parent);
      if (computedStyle.position === "static") {
        parent.style.position = "relative";
      }

      const isPlaylist = isYT && (window.location.search.includes("list=") || window.location.pathname.includes("/playlist"));

      const badgeContainer = document.createElement("div");
      badgeContainer.className = "mad-dm-badge-container";

      badgeContainer.innerHTML = `
        <button class="mad-dm-video-badge" type="button">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
            <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path>
            <polyline points="7 10 12 15 17 10"></polyline>
            <line x1="12" y1="15" x2="12" y2="3"></line>
          </svg>
          <span class="badge-text">${isYT ? "Download YouTube ⚡" : "Download Video ⚡"}</span>
          <span style="font-size: 9px; opacity: 0.7;">▼</span>
        </button>
        <div class="mad-dm-format-menu">
          <div class="mad-dm-format-item" data-format="mp4">🎬 MP4 Video (HD)</div>
          <div class="mad-dm-format-item" data-format="mp3">🎵 MP3 Audio (320k)</div>
          <div class="mad-dm-format-item" data-format="flac">🎼 FLAC Lossless</div>
          ${isPlaylist ? `
            <div class="mad-dm-format-item" data-playlist="true" data-merge="false" style="border-top: 1px solid rgba(255,255,255,0.1); color: #ec4899;">📑 Download Playlist (Separate)</div>
            <div class="mad-dm-format-item" data-playlist="true" data-merge="true" style="color: #00f0ff;">⚡ Merge Playlist to 1 File</div>
          ` : ''}
        </div>
      `;

      const mainBtn = badgeContainer.querySelector(".mad-dm-video-badge");
      const badgeText = badgeContainer.querySelector(".badge-text");

      function triggerDownload(format, downloadPlaylist = false, merge = false) {
        const targetUrl = isYT ? window.location.href : url;
        mainBtn.classList.add("loading");
        badgeText.textContent = "Dispatching...";

        chrome.runtime.sendMessage(
          {
            type: "DOWNLOAD_URL",
            url: targetUrl,
            format: format,
            isPlaylist: downloadPlaylist,
            merge: merge
          },
          (response) => {
            if (response && response.success) {
              mainBtn.classList.remove("loading");
              mainBtn.classList.add("success");
              badgeText.textContent = merge ? "✓ Merge Queued!" : (downloadPlaylist ? "✓ Playlist Queued!" : `✓ ${format.toUpperCase()} Added!`);
              setTimeout(() => {
                mainBtn.classList.remove("success");
                badgeText.textContent = isYT ? "Download YouTube ⚡" : "Download Video ⚡";
              }, 3000);
            } else {
              mainBtn.classList.remove("loading");
              mainBtn.classList.add("error");
              badgeText.textContent = "❌ Failed to connect";
              setTimeout(() => {
                mainBtn.classList.remove("error");
                badgeText.textContent = isYT ? "Download YouTube ⚡" : "Download Video ⚡";
              }, 3000);
            }
          }
        );
      }

      mainBtn.addEventListener("click", (e) => {
        e.preventDefault();
        e.stopPropagation();
        triggerDownload("mp4", false, false);
      });

      badgeContainer.querySelectorAll(".mad-dm-format-item").forEach((item) => {
        item.addEventListener("click", (e) => {
          e.preventDefault();
          e.stopPropagation();
          const fmt = item.getAttribute("data-format") || "mp4";
          const isPl = item.getAttribute("data-playlist") === "true";
          const isMerge = item.getAttribute("data-merge") === "true";
          triggerDownload(fmt, isPl, isMerge);
        });
      });

      parent.appendChild(badgeContainer);
    });
  }

  // Scan immediately & listen to DOM mutations / YouTube SPA page transitions
  scanVideos();
  const observer = new MutationObserver(() => scanVideos());
  observer.observe(document.body, { childList: true, subtree: true });

  window.addEventListener("yt-navigate-finish", () => {
    setTimeout(scanVideos, 500);
  });
})();
