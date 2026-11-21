const autoIntercept = document.getElementById("autoIntercept");
const filterExtensions = document.getElementById("filterExtensions");
const extensionList = document.getElementById("extensionList");
const showFloatingBadge = document.getElementById("showFloatingBadge");
const showNotifications = document.getElementById("showNotifications");
const saveBanner = document.getElementById("saveBanner");

// Load stored settings (default: autoIntercept = false)
chrome.storage.local.get(
  {
    autoIntercept: false,
    filterExtensions: false,
    extensionList: "zip,iso,dmg,exe,tar,gz,rar,7z,mp4,mkv,bin,pkg",
    showFloatingBadge: true,
    showNotifications: true
  },
  (settings) => {
    autoIntercept.checked = settings.autoIntercept;
    filterExtensions.checked = settings.filterExtensions;
    extensionList.value = settings.extensionList;
    showFloatingBadge.checked = settings.showFloatingBadge;
    showNotifications.checked = settings.showNotifications;
  }
);

function save() {
  chrome.storage.local.set(
    {
      autoIntercept: autoIntercept.checked,
      filterExtensions: filterExtensions.checked,
      extensionList: extensionList.value.trim(),
      showFloatingBadge: showFloatingBadge.checked,
      showNotifications: showNotifications.checked
    },
    () => {
      saveBanner.style.display = "block";
      setTimeout(() => {
        saveBanner.style.display = "none";
      }, 2000);
    }
  );
}

autoIntercept.addEventListener("change", save);
filterExtensions.addEventListener("change", save);
extensionList.addEventListener("change", save);
showFloatingBadge.addEventListener("change", save);
showNotifications.addEventListener("change", save);
