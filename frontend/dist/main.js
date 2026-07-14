const state = {
  catalog: null,
  activeCategoryId: "",
  activeSubcategoryId: "",
  selectedIds: new Set(),
  selectedPackageId: "",
  activeSidebarTab: "queue",
  searchQuery: "",
  vendorFilter: "",
  running: false,
  settingsOpen: false,
};

const elements = {
  mainPanel: document.querySelector(".main-panel"),
  nav: document.querySelector("#nav"),
  location: document.querySelector("#location"),
  locationDescription: document.querySelector("#location-description"),
  packageCount: document.querySelector("#package-count"),
  queuePill: document.querySelector("#queue-pill"),
  packageList: document.querySelector("#package-list"),
  sidebarTitle: document.querySelector("#sidebar-title"),
  sidebarStatus: document.querySelector("#sidebar-status"),
  queueTab: document.querySelector("#queue-tab"),
  detailsTab: document.querySelector("#details-tab"),
  queueView: document.querySelector("#queue-view"),
  detailsView: document.querySelector("#details-view"),
  queueBody: document.querySelector("#queue-body"),
  detailBody: document.querySelector("#detail-body"),
  log: document.querySelector("#log"),
  runState: document.querySelector("#run-state"),
  searchInput: document.querySelector("#search-input"),
  vendorFilter: document.querySelector("#vendor-filter"),
  settingsButton: document.querySelector("#settings-button"),
  settingsOverlay: document.querySelector("#settings-overlay"),
  settingsCloseButton: document.querySelector("#settings-close-button"),
  settingsCancelButton: document.querySelector("#settings-cancel-button"),
  settingsApplyButton: document.querySelector("#settings-apply-button"),
  desktopIconSelect: document.querySelector("#desktop-icon-select"),
  settingsStatus: document.querySelector("#settings-status"),
  refreshButton: document.querySelector("#refresh-button"),
  clearButton: document.querySelector("#clear-button"),
  runButton: document.querySelector("#run-button"),
  backToTopButton: document.querySelector("#back-to-top-button"),
  packageCardTemplate: document.querySelector("#package-card-template"),
};

const softwareTypeMeta = {
  standalone: {
    label: "Standalone",
    icon: "/assets/images/icons/standalone-logo.png",
  },
  clap: {
    label: "CLAP",
    icon: "/assets/images/icons/clap-logo.png",
  },
  vst: {
    label: "VST",
    icon: "/assets/images/icons/vst-logo.png",
  },
  lv2: {
    label: "LV2",
    icon: "/assets/images/icons/lv2-logo.svg",
  },
};

function backend() {
  const bound = window.go?.guiapp?.App || window.go?.main?.App;
  if (!bound) {
    throw new Error("Wails backend bindings are not available.");
  }
  return bound;
}

async function boot() {
  bindEvents();
  await loadCatalog();
}

function bindEvents() {
  window.addEventListener("scroll", updateBackToTopVisibility, { passive: true });
  elements.mainPanel.addEventListener("scroll", updateBackToTopVisibility, { passive: true });
  elements.backToTopButton.addEventListener("click", () => {
    window.scrollTo({
      top: 0,
      behavior: "smooth",
    });
    elements.mainPanel.scrollTo({
      top: 0,
      behavior: "smooth",
    });
  });

  elements.queueTab.addEventListener("click", () => {
    setActiveSidebarTab("queue");
  });

  elements.detailsTab.addEventListener("click", () => {
    setActiveSidebarTab("details");
  });

  elements.searchInput.addEventListener("input", (event) => {
    state.searchQuery = event.target.value || "";
    syncSelectionWithVisiblePackages();
    ensureActiveLocation();
    render();
  });

  elements.vendorFilter.addEventListener("change", (event) => {
    state.vendorFilter = event.target.value || "";
    syncSelectionWithVisiblePackages();
    ensureActiveLocation();
    render();
  });

  elements.settingsButton.addEventListener("click", () => {
    openSettings();
  });

  elements.settingsCloseButton.addEventListener("click", () => {
    closeSettings();
  });

  elements.settingsCancelButton.addEventListener("click", () => {
    closeSettings();
  });

  elements.settingsOverlay.addEventListener("click", (event) => {
    if (event.target === elements.settingsOverlay) {
      closeSettings();
    }
  });

  elements.settingsApplyButton.addEventListener("click", async () => {
    await applyDesktopIcon();
  });

  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && state.settingsOpen) {
      closeSettings();
    }
  });

  elements.refreshButton.addEventListener("click", async () => {
    await loadCatalog({ keepSelection: true });
    appendLog("event", "Catalog state refreshed.");
  });

  elements.clearButton.addEventListener("click", () => {
    state.selectedIds.clear();
    render();
    appendLog("event", "Queue cleared.");
  });

  elements.runButton.addEventListener("click", async () => {
    if (state.running) {
      return;
    }

    const ids = [...state.selectedIds];
    if (ids.length === 0) {
      appendLog("event", "Select one or more actionable packages first.");
      return;
    }

    const names = ids
      .map((id) => findPackageById(id)?.name)
      .filter(Boolean)
      .join(", ");

    if (!window.confirm(`Run queued actions for: ${names}?`)) {
      return;
    }

    try {
      await backend().RunSelection(ids);
      state.running = true;
      updateRunState("Running", "running");
      appendLog("event", `Queued ${ids.length} package action(s).`);
      render();
    } catch (error) {
      appendLog("stderr", error?.message || String(error));
      updateRunState("Error", "error");
    }
  });

  if (window.runtime?.EventsOn) {
    window.runtime.EventsOn("installer:run-started", (payload) => {
      state.running = true;
      updateRunState("Running", "running");
      appendLog("event", `Starting run for ${payload.jobs.length} package(s).`);
      if (payload.skipped?.length) {
        appendLog("event", `Skipped: ${payload.skipped.join(", ")}`);
      }
      render();
    });

    window.runtime.EventsOn("installer:job-started", (payload) => {
      appendLog("event", `[${payload.index}/${payload.total}] ${payload.packageName} (${payload.mode})`);
    });

    window.runtime.EventsOn("installer:action-started", (payload) => {
      appendLog("event", `-> ${payload.packageName}: ${payload.title}`);
    });

    window.runtime.EventsOn("installer:action-output", (payload) => {
      appendLog(payload.stream === "stderr" ? "stderr" : "stdout", payload.message);
    });

    window.runtime.EventsOn("installer:run-finished", (payload) => {
      state.running = false;
      state.catalog = payload.catalog;
      state.selectedIds.clear();
      ensureActiveLocation();
      render();

      const failures = payload.results.filter((result) => !result.success);
      if (failures.length > 0) {
        updateRunState("Finished with errors", "error");
      } else {
        updateRunState("Run complete", "installed");
      }

      payload.results.forEach((result) => {
        if (result.success) {
          appendLog("event", `${result.packageName}: ${result.mode} complete`);
          return;
        }
        appendLog("stderr", `${result.packageName}: ${result.error}`);
      });

      if (failures.length > 0) {
        const reportLine = document.createElement("div");
        reportLine.className = "log-line event";
        const prefix = document.createTextNode("Report this issue at: ");
        const reportLink = document.createElement("a");
        reportLink.href = "https://github.com/caracal-dev/caracal-software-installer/issues";
        reportLink.textContent = "https://github.com/caracal-dev/caracal-software-installer/issues";
        reportLink.target = "_blank";
        reportLink.className = "log-link";
        reportLine.appendChild(prefix);
        reportLine.appendChild(reportLink);
        elements.log.appendChild(reportLine);
        elements.log.scrollTop = elements.log.scrollHeight;
        const queueView = document.getElementById("queue-view");
        if (queueView) {
          queueView.scrollTop = queueView.scrollHeight;
        }
      }
    });
  }
}

async function loadCatalog({ keepSelection = false } = {}) {
  const payload = await backend().GetCatalog();
  state.catalog = payload;
  if (!keepSelection) {
    state.selectedIds.clear();
  } else {
    const validIds = new Set(allPackages().map((pkg) => pkg.id));
    state.selectedIds = new Set([...state.selectedIds].filter((id) => validIds.has(id)));
  }

  populateVendorFilter();
  ensureActiveLocation();
  render();
}

function ensureActiveLocation() {
  const categories = state.catalog?.categories || [];
  const visible = visiblePackages();

  if (hasActiveFilters()) {
    if (visible.length === 0) {
      state.selectedPackageId = "";
      return;
    }

    const selectedVisible = visible.find((item) => item.id === state.selectedPackageId);
    if (!selectedVisible) {
      state.selectedPackageId = visible[0].id;
    }
    return;
  }

  if (categories.length === 0) {
    state.activeCategoryId = "";
    state.activeSubcategoryId = "";
    state.selectedPackageId = "";
    return;
  }

  let category = categories.find((item) => item.id === state.activeCategoryId);
  if (!category) {
    category = categories[0];
    state.activeCategoryId = category.id;
  }

  let subcategory = category.subcategories.find((item) => item.id === state.activeSubcategoryId);
  if (!subcategory) {
    subcategory = category.subcategories[0];
    state.activeSubcategoryId = subcategory?.id || "";
  }

  if (!subcategory) {
    state.selectedPackageId = "";
    return;
  }

  const selectedPackage = subcategory.packages.find((item) => item.id === state.selectedPackageId);
  if (!selectedPackage) {
    state.selectedPackageId = subcategory.packages[0]?.id || "";
  }
}

function render() {
  window.scrollTo({ top: 0, behavior: "instant" });
  renderNav();
  renderPackages();
  renderSidebar();
  renderSettingsOverlay();
  elements.queuePill.textContent = `${state.selectedIds.size} selected`;
  elements.runButton.disabled = state.selectedIds.size === 0 || state.running;
  elements.clearButton.disabled = state.selectedIds.size === 0 || state.running;
  updateBackToTopVisibility();
}

async function openSettings() {
  state.settingsOpen = true;
  renderSettingsOverlay();
  await loadIconSettings();
  elements.desktopIconSelect.focus();
}

function closeSettings() {
  state.settingsOpen = false;
  renderSettingsOverlay();
}

function renderSettingsOverlay() {
  elements.settingsOverlay.classList.toggle("is-hidden", !state.settingsOpen);
}

async function loadIconSettings() {
  elements.settingsStatus.textContent = "Loading icon options...";
  elements.settingsApplyButton.disabled = true;

  try {
    const payload = await backend().GetIconSettings();
    renderIconOptions(payload);
    elements.settingsStatus.textContent = "Ready.";
  } catch (error) {
    elements.settingsStatus.textContent = error?.message || String(error);
  } finally {
    elements.settingsApplyButton.disabled = false;
  }
}

function renderIconOptions(payload) {
  elements.desktopIconSelect.replaceChildren();
  const icons = payload?.icons || [];

  for (const icon of icons) {
    const option = document.createElement("option");
    option.value = icon.id;
    option.textContent = icon.label;
    elements.desktopIconSelect.appendChild(option);
  }

  elements.desktopIconSelect.value = payload?.activeId || "appicon.png";
}

async function applyDesktopIcon() {
  const iconID = elements.desktopIconSelect.value || "appicon.png";
  elements.settingsApplyButton.disabled = true;
  elements.settingsStatus.textContent = "Applying desktop icon...";

  try {
    const payload = await backend().SetDesktopIcon(iconID);
    renderIconOptions(payload);
    elements.settingsStatus.textContent = `Desktop icon set to ${payload.activeId || iconID}.`;
    appendLog("event", `Desktop icon set to ${payload.activeId || iconID}.`);
  } catch (error) {
    elements.settingsStatus.textContent = error?.message || String(error);
    appendLog("stderr", error?.message || String(error));
  } finally {
    elements.settingsApplyButton.disabled = false;
  }
}

function renderNav() {
  elements.nav.replaceChildren();

  for (const category of state.catalog.categories) {
    const wrapper = document.createElement("section");
    wrapper.className = "nav-category";

    const title = document.createElement("h3");
    title.className = "nav-category-title";
    title.textContent = category.name;
    wrapper.appendChild(title);

    for (const subcategory of category.subcategories) {
      const button = document.createElement("button");
      button.type = "button";
      button.className = "nav-subcategory";
      if (category.id === state.activeCategoryId && subcategory.id === state.activeSubcategoryId) {
        button.classList.add("active");
      }
      button.textContent = `${subcategory.name} (${subcategory.packages.length})`;
      button.addEventListener("click", () => {
        state.activeCategoryId = category.id;
        state.activeSubcategoryId = subcategory.id;
        state.selectedPackageId = subcategory.packages[0]?.id || "";
        render();
      });
      wrapper.appendChild(button);
    }

    elements.nav.appendChild(wrapper);
  }
}

function renderPackages() {
  const subcategory = activeSubcategory();
  const category = activeCategory();
  const filtered = visiblePackages();

  if (hasActiveFilters()) {
    const brand = state.vendorFilter || "All brands";
    elements.location.textContent = "Search Results";
    elements.locationDescription.textContent = state.searchQuery
      ? `Filtered across the full catalog for "${state.searchQuery}" • ${brand}`
      : `Filtered across the full catalog • ${brand}`;
  } else {
    elements.location.textContent = category && subcategory ? `${category.name} / ${subcategory.name}` : "Catalog";
    elements.locationDescription.textContent = subcategory?.description || category?.description || "";
  }

  elements.packageCount.textContent = `${filtered.length} package${filtered.length === 1 ? "" : "s"}`;

  elements.packageList.replaceChildren();

  if (filtered.length === 0) {
    elements.packageList.appendChild(emptyState(hasActiveFilters() ? "No packages match the current filters." : "No packages are available in this section yet."));
    return;
  }

  for (const pkg of filtered) {
    const card = elements.packageCardTemplate.content.firstElementChild.cloneNode(true);
    if (state.selectedIds.has(pkg.id)) {
      card.classList.add("selected");
    }

    attachThumbnail(
      card.querySelector(".package-thumbnail-image"),
      card.querySelector(".package-thumbnail-fallback"),
      card.querySelector(".package-thumbnail-token"),
      pkg,
    );

    card.querySelector(".package-vendor").textContent = pkg.vendor || "Catalog";
    card.querySelector(".package-name").textContent = pkg.name;
    card.querySelector(".package-path").textContent = `${pkg.categoryName} / ${pkg.subcategoryName}`;
    card.querySelector(".package-summary").textContent = pkg.summary;
    renderSoftwareTypes(card.querySelector(".package-software-types"), pkg);
    renderPackageTraits(card.querySelector(".package-traits"), pkg);

    const badge = card.querySelector(".status-badge");
    badge.textContent = pkg.state.statusLabel;
    badge.classList.add(statusClass(pkg.state.statusLabel));

    const detailButton = card.querySelector(".package-detail-button");
    detailButton.addEventListener("click", () => {
      state.selectedPackageId = pkg.id;
      setActiveSidebarTab("details");
    });

    const queueButton = card.querySelector(".package-queue-button");
    queueButton.textContent = state.selectedIds.has(pkg.id) && pkg.state.actionKind === "install" ? "Remove from queue" : pkg.state.actionLabel;
    queueButton.classList.add(buttonVariantClass(pkg));
    queueButton.disabled = !pkg.state.actionable || state.running;
    queueButton.addEventListener("click", async () => {
      if (!pkg.state.actionable || state.running) {
        return;
      }
      if (pkg.state.actionKind === "link") {
        if (!pkg.state.actionUrl) {
          appendLog("stderr", `No external URL configured for ${pkg.name}.`);
          return;
        }
        try {
          await backend().OpenLink(pkg.state.actionUrl);
          appendLog("event", `Opened ${pkg.name} in the browser.`);
        } catch (error) {
          appendLog("stderr", error?.message || String(error));
        }
        return;
      }

      if (pkg.state.actionKind === "uninstall") {
        if (!window.confirm(`Uninstall ${pkg.name}?`)) {
          return;
        }
        try {
          await backend().RunSelection([pkg.id]);
          state.running = true;
          updateRunState("Running", "running");
          appendLog("event", `Starting uninstall for ${pkg.name}.`);
          render();
        } catch (error) {
          appendLog("stderr", error?.message || String(error));
          updateRunState("Error", "error");
        }
        return;
      }

      toggleSelection(pkg.id);
    });

    elements.packageList.appendChild(card);
  }
}

function renderSoftwareTypes(node, pkg) {
  if (!node) {
    return;
  }
 
  node.replaceChildren();
 
  const types = (pkg.softwareTypes || []).map((kind) => softwareTypeMeta[kind]).filter(Boolean);
  if (types.length === 0) {
    node.classList.add("is-hidden");
    return;
  }
 
  node.classList.remove("is-hidden");
 
  for (const type of types) {
    const item = document.createElement("div");
    item.className = "package-software-type";
    item.title = type.label;
    item.setAttribute("aria-label", type.label);
    item.innerHTML = `
      <img class="package-software-type-icon" src="${type.icon}" alt="${type.label}" />
      <span class="package-software-type-label">${escapeHtml(type.label)}</span>
    `;
    node.appendChild(item);
  }
}
function renderPackageTraits(node, pkg) {
  if (!node) {
    return;
  }
 
  node.replaceChildren();

  const traits = [
    {
      label: pkg.openSource ? "Open Source" : "Proprietary",
      kind: pkg.openSource ? "open" : "proprietary",
    },
  ];

  if (pkg.openSource && pkg.license?.label) {
    traits.push({
      label: pkg.license.label,
      kind: `license package-license-${licenseKindClass(pkg.license.kind)}`,
      url: pkg.license.url,
    });
  }

  if (pkg.hasFreeVersion === false) {
    traits.push({
      label: "$$ No Free Version",
      kind: "paid",
    });
  }

  if (traits.length === 0) {
    node.classList.add("is-hidden");
    return;
  }

  node.classList.remove("is-hidden");

  for (const trait of traits) {
    const item = document.createElement(trait.url ? "button" : "span");
    item.className = `package-trait package-trait-${trait.kind}`;
    item.textContent = trait.label;
    if (trait.url) {
      item.type = "button";
      item.title = `${trait.label} license`;
      item.addEventListener("click", async () => {
        try {
          await backend().OpenLink(trait.url);
        } catch (error) {
          appendLog("stderr", error?.message || String(error));
        }
      });
    }
    node.appendChild(item);
  }
}

function buttonVariantClass(pkg) {
  switch (pkg.state.actionKind) {
    case "uninstall":
      return "action-uninstall";
    case "link":
      return "action-link";
    case "install":
    default:
      return "action-install";
  }
}

function renderSidebar() {
  const showQueue = state.activeSidebarTab !== "details";
  const pkg = selectedPackage();

  elements.queueTab.classList.toggle("is-active", showQueue);
  elements.detailsTab.classList.toggle("is-active", !showQueue);
  elements.queueTab.setAttribute("aria-selected", String(showQueue));
  elements.detailsTab.setAttribute("aria-selected", String(!showQueue));
  elements.queueView.classList.toggle("is-hidden", !showQueue);
  elements.detailsView.classList.toggle("is-hidden", showQueue);

  if (showQueue) {
    elements.sidebarTitle.textContent = "Download Queue";
    elements.sidebarStatus.textContent = `${state.selectedIds.size} selected`;
    elements.sidebarStatus.className = "status-badge neutral";
  } else {
    elements.sidebarTitle.textContent = pkg?.name || "Package Details";
    elements.sidebarStatus.textContent = pkg?.state?.statusLabel || "Catalog";
    elements.sidebarStatus.className = `status-badge ${statusClass(pkg?.state?.statusLabel || "")}`;
  }

  renderQueue();
  renderDetails();
}

function renderQueue() {
  const queuedPackages = [...state.selectedIds]
    .map((id) => findPackageById(id))
    .filter(Boolean);

  elements.queueBody.replaceChildren();

  if (queuedPackages.length === 0) {
    elements.queueBody.appendChild(emptyState("Queue installs or removals from the package list to see them here."));
    return;
  }

  const list = document.createElement("div");
  list.className = "queue-list";

  for (const pkg of queuedPackages) {
    const item = document.createElement("article");
    item.className = "queue-item";

    item.innerHTML = `
      <div class="queue-item-topline">
        <div>
          <p class="package-vendor">${escapeHtml(pkg.vendor || "Catalog")}</p>
          <h4 class="queue-item-name">${escapeHtml(pkg.name)}</h4>
        </div>
        <span class="status-badge ${statusClass(pkg.state.statusLabel)}">${escapeHtml(pkg.state.actionLabel)}</span>
      </div>
      <p class="package-path">${escapeHtml(pkg.categoryName)} / ${escapeHtml(pkg.subcategoryName)}</p>
      <p class="queue-item-summary">${escapeHtml(pkg.summary)}</p>
      <div class="queue-item-actions">
        <button class="ghost-button queue-details-button" type="button">Details</button>
        <button class="ghost-button queue-remove-button" type="button">Remove</button>
      </div>
    `;

    item.querySelector(".queue-details-button").addEventListener("click", () => {
      state.selectedPackageId = pkg.id;
      setActiveSidebarTab("details");
    });

    item.querySelector(".queue-remove-button").addEventListener("click", () => {
      toggleSelection(pkg.id);
    });

    list.appendChild(item);
  }

  elements.queueBody.appendChild(list);
}

function renderDetails() {
  const pkg = selectedPackage();
  if (!pkg) {
    elements.detailBody.innerHTML = "";
    elements.detailBody.appendChild(emptyState("Choose Details on a package card or queued item to inspect it here."));
    return;
  }

  const fragments = [];
  fragments.push(`
    <div class="detail-thumbnail">
      <img class="detail-thumbnail-image" alt="" />
      <div class="detail-thumbnail-fallback">
        <span class="detail-thumbnail-token"></span>
      </div>
    </div>
  `);
  fragments.push(`<p>${escapeHtml(pkg.description)}</p>`);
  fragments.push(`<p class="package-path">${escapeHtml(pkg.categoryName)} / ${escapeHtml(pkg.subcategoryName)}</p>`);

  if (pkg.availabilityNote) {
    fragments.push(`<div class="detail-note">${escapeHtml(pkg.availabilityNote)}</div>`);
  }

  if (pkg.notes?.length) {
    fragments.push("<h4>Notes</h4>");
    fragments.push(`<ul>${pkg.notes.map((note) => `<li>${escapeHtml(note)}</li>`).join("")}</ul>`);
  }

  if (pkg.openSource && pkg.license?.label) {
    fragments.push("<h4>License</h4>");
    if (pkg.license.url) {
      fragments.push(`<div class="detail-actions"><button class="inline-link detail-license-button package-license-${licenseKindClass(pkg.license.kind)}" type="button">${escapeHtml(pkg.license.label)}</button></div>`);
    } else {
      fragments.push(`<div class="detail-actions"><span class="inline-link package-license-${licenseKindClass(pkg.license.kind)}">${escapeHtml(pkg.license.label)}</span></div>`);
    }
  }

  if (pkg.installActions?.length || pkg.uninstallActions?.length) {
    fragments.push("<h4>Installer Actions</h4>");
    const actions = [...pkg.installActions, ...pkg.uninstallActions];
    fragments.push(`<div class="detail-actions">${actions.map((action) => `<span class="inline-link">${escapeHtml(action.title)}</span>`).join("")}</div>`);
  }

  fragments.push('<h4>Support</h4>');
  fragments.push('<div class="detail-actions"><button class="ghost-button report-link-button" type="button">Report Broken Link</button></div>');

  if (pkg.links?.length) {
    fragments.push("<h4>Links</h4>");
    fragments.push(`<div class="detail-links">${pkg.links.map((link, index) => `<button class="ghost-button detail-link-button" data-index="${index}" type="button">${escapeHtml(link.label)}</button>`).join("")}</div>`);
   if (pkg.links.some((link) => link.label === "Site")) {
      fragments.push('<p class="detail-support-note">Please visit and support the developers when you can. Many of these projects rely on web traffic and donations to keep building.</p>');
    }
  }
  elements.detailBody.innerHTML = fragments.join("");

  attachThumbnail(
    elements.detailBody.querySelector(".detail-thumbnail-image"),
    elements.detailBody.querySelector(".detail-thumbnail-fallback"),
    elements.detailBody.querySelector(".detail-thumbnail-token"),
    pkg,
  );

  elements.detailBody.querySelectorAll(".detail-link-button").forEach((button) => {
    button.addEventListener("click", async () => {
      const index = Number(button.dataset.index);
      const link = pkg.links[index];
      if (!link) {
        return;
      }
      try {
        await backend().OpenLink(link.url);
      } catch (error) {
        appendLog("stderr", error?.message || String(error));
      }
    });
  });

  const licenseButton = elements.detailBody.querySelector(".detail-license-button");
  if (licenseButton && pkg.license?.url) {
    licenseButton.addEventListener("click", async () => {
      try {
        await backend().OpenLink(pkg.license.url);
      } catch (error) {
        appendLog("stderr", error?.message || String(error));
      }
    });
  }

  const reportButton = elements.detailBody.querySelector(".report-link-button");
  if (reportButton) {
    reportButton.addEventListener("click", async () => {
      try {
        await backend().OpenLink(buildBugReportURL(pkg));
      } catch (error) {
        appendLog("stderr", error?.message || String(error));
      }
    });
  }
}

function toggleSelection(id) {
  if (state.selectedIds.has(id)) {
    state.selectedIds.delete(id);
  } else {
    state.selectedIds.add(id);
  }
  render();
}

function appendLog(kind, message) {
  const line = document.createElement("div");
  line.className = `log-line ${kind}`;
  if (message.startsWith("http")) {
    const link = document.createElement("a");
    link.href = message;
    link.textContent = message;
    link.target = "_blank";
    link.className = "log-link";
    line.appendChild(link);
  } else {
    line.textContent = message;
  }
  elements.log.appendChild(line);
  elements.log.scrollTop = elements.log.scrollHeight;
  const queueView = document.getElementById("queue-view");
  if (queueView) {
    queueView.scrollTop = queueView.scrollHeight;
  }
}

function attachThumbnail(imageNode, fallbackNode, tokenNode, pkg) {
  if (!imageNode || !fallbackNode || !tokenNode || !pkg) {
    return;
  }

  tokenNode.textContent = thumbnailToken(pkg);

  const candidates = thumbnailCandidates(pkg);
  if (candidates.length === 0) {
    imageNode.removeAttribute("src");
    imageNode.classList.remove("is-visible");
    fallbackNode.classList.remove("is-hidden");
    return;
  }

  let candidateIndex = 0;

  const tryNext = () => {
    if (candidateIndex >= candidates.length) {
      imageNode.removeAttribute("src");
      imageNode.classList.remove("is-visible");
      fallbackNode.classList.remove("is-hidden");
      return;
    }
    imageNode.src = candidates[candidateIndex++];
  };

  imageNode.onload = () => {
    imageNode.classList.add("is-visible");
    fallbackNode.classList.add("is-hidden");
  };

  imageNode.onerror = () => {
    imageNode.classList.remove("is-visible");
    fallbackNode.classList.remove("is-hidden");
    tryNext();
  };

  tryNext();
}

function thumbnailCandidates(pkg) {
  const extensions = ["webp", "png", "jpg", "jpeg", "svg"];
  return extensions.map((extension) => `/assets/images/thumbnails/${pkg.id}.${extension}`);
}

function thumbnailToken(pkg) {
  const source = pkg.name || pkg.id || "plugin";
  const parts = source
    .split(/[\s-]+/)
    .map((part) => part.trim())
    .filter(Boolean);

  if (parts.length >= 2) {
    return `${parts[0][0] || ""}${parts[1][0] || ""}`.toUpperCase();
  }

  return source.slice(0, 2).toUpperCase();
}

function buildBugReportURL(pkg) {
  const title = `[Bug] Broken link: ${pkg.name}`;
  const lines = [
    "## Broken link report",
    "",
    `- Package: ${pkg.name}`,
    `- Package ID: ${pkg.id}`,
    `- Vendor: ${pkg.vendor || "Unknown"}`,
    `- Category: ${pkg.categoryName || "Unknown"} / ${pkg.subcategoryName || "Unknown"}`,
    "",
    "### Links shown in the app",
    ...(pkg.links?.length
      ? pkg.links.map((link) => `- ${link.label}: ${link.url}`)
      : ["- No links were shown in the app"]),
    "",
    "### What happened",
    "",
    "",
  ];

  const query = new URLSearchParams({
    title,
    body: lines.join("\n"),
  });

  return `https://github.com/caracal-dev/caracal-software-installer/issues/new?${query.toString()}`;
}

function updateRunState(label, kind) {
  elements.runState.textContent = label;
  elements.runState.className = `status-badge ${kind}`;
}

function updateBackToTopVisibility() {
  const scrollTop = Math.max(window.scrollY, elements.mainPanel.scrollTop);
  elements.backToTopButton.classList.toggle("is-visible", scrollTop > 420);
}

function activeCategory() {
  return state.catalog?.categories.find((category) => category.id === state.activeCategoryId) || null;
}

function activeSubcategory() {
  return activeCategory()?.subcategories.find((subcategory) => subcategory.id === state.activeSubcategoryId) || null;
}

function selectedPackage() {
  return findPackageById(state.selectedPackageId);
}

function allPackages() {
  return (state.catalog?.categories || []).flatMap((category) =>
    category.subcategories.flatMap((subcategory) => subcategory.packages),
  );
}

function visiblePackages() {
  const query = state.searchQuery.trim().toLowerCase();
  const vendor = state.vendorFilter;

  if (!hasActiveFilters()) {
    return activeSubcategory()?.packages || [];
  }

  return allPackages().filter((pkg) => {
    if (vendor && pkg.vendor !== vendor) {
      return false;
    }

    if (!query) {
      return true;
    }

    const haystack = [
      pkg.name,
      pkg.vendor,
      pkg.summary,
      pkg.description,
      pkg.categoryName,
      pkg.subcategoryName,
    ]
      .filter(Boolean)
      .join(" ")
      .toLowerCase();

    return haystack.includes(query);
  });
}

function hasActiveFilters() {
  return Boolean(state.searchQuery.trim() || state.vendorFilter);
}

function populateVendorFilter() {
  const current = state.vendorFilter;
  const vendors = Array.from(new Set(allPackages().map((pkg) => pkg.vendor).filter(Boolean))).sort((left, right) =>
    left.localeCompare(right),
  );

  elements.vendorFilter.replaceChildren();

  const allOption = document.createElement("option");
  allOption.value = "";
  allOption.textContent = "All brands";
  elements.vendorFilter.appendChild(allOption);

  for (const vendor of vendors) {
    const option = document.createElement("option");
    option.value = vendor;
    option.textContent = vendor;
    elements.vendorFilter.appendChild(option);
  }

  elements.vendorFilter.value = vendors.includes(current) ? current : "";
  state.vendorFilter = elements.vendorFilter.value;
}

function syncSelectionWithVisiblePackages() {
  const visibleIds = new Set(visiblePackages().map((pkg) => pkg.id));
  state.selectedIds = new Set([...state.selectedIds].filter((id) => visibleIds.has(id) || allPackages().some((pkg) => pkg.id === id)));
}

function findPackageById(id) {
  return allPackages().find((pkg) => pkg.id === id) || null;
}

function setActiveSidebarTab(tab) {
  if (tab === "details" && !selectedPackage()) {
    state.activeSidebarTab = "queue";
  } else {
    state.activeSidebarTab = tab;
  }
  render();
}

function emptyState(message) {
  const node = document.createElement("div");
  node.className = "empty-state";
  node.textContent = message;
  return node;
}

function statusClass(label) {
  switch (label) {
    case "Installed":
      return "installed";
    case "Available":
      return "available";
    case "Available on site":
      return "catalog";
    case "Run complete":
      return "installed";
    case "Catalog only":
      return "catalog";
    default:
      return "neutral";
  }
}

function licenseKindClass(kind) {
  return String(kind || "other").replaceAll(/[^a-z0-9-]/gi, "-").toLowerCase();
}

function escapeHtml(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

boot().catch((error) => {
  appendLog("stderr", error?.message || String(error));
  updateRunState("Error", "error");
});
