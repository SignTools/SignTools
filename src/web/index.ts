import { Modal, Tab, Tooltip } from "bootstrap";
import Masonry from "masonry-layout";
import Uppy, { type UploadResult } from "@uppy/core";
import Tus from "@uppy/tus";
import { byId, errorMessage, eventElement, query } from "./dom";

type UploadMeta = { uploadType?: "ipa" | "tweak" };
type UploadBody = Record<string, never>;

const btnUploadApp = byId("btnUploadApp", HTMLAnchorElement);
const formFileSelect = byId("formFileSelect", HTMLInputElement);
const formFileId = byId("formFileId", HTMLInputElement);
const formFileText = byId("formFileText", HTMLInputElement);
const formFileUseUrl = byId("formFileUseUrl", HTMLInputElement);
const formFileUrl = byId("formFileUrl", HTMLInputElement);
const formTweakIds = byId("formTweakIds", HTMLInputElement);
const formTweaksSelect = byId("formTweaksSelect", HTMLInputElement);
const formTweaksUseLibrary = byId("formTweaksUseLibrary", HTMLInputElement);
const formSubmit = byId("formSubmit", HTMLButtonElement);
const uploadForm = byId("uploadForm", HTMLFormElement);
const modalBody = byId("modalBody", HTMLDivElement);
const formFileSpinner = byId("formFileSpinner", HTMLDivElement);
const modalElem = byId("uploadModal", HTMLDivElement);
const modal = new Modal(modalElem, {
  backdrop: "static",
  keyboard: false,
});
const btnModalClose = byId("btnModalClose", HTMLButtonElement);
const formProfile = byId("formProfile", HTMLSelectElement);
const formIdOriginal = byId("formIdOriginal", HTMLInputElement);
const formIdProv = byId("formIdProv", HTMLInputElement);
const formIdCustom = byId("formIdCustom", HTMLInputElement);
const formIdCustomText = byId("formIdCustomText", HTMLInputElement);
const formIdEncode = byId("formIdEncode", HTMLInputElement);
const inputSearchFilter = byId("inputSearchFilter", HTMLInputElement);
const btnSearchClear = byId("btnSearchClear", HTMLButtonElement);
const liveStatus = byId("liveStatus", HTMLSpanElement);
const liveStatusText = byId("liveStatusText", HTMLSpanElement);
const masonryRow = byId("masonryRow", HTMLDivElement);
const masonryTweaksRow = byId("masonryTweaksRow", HTMLDivElement);
const tweaksEmpty = byId("tweaksEmpty", HTMLParagraphElement);
const savedTweaksContainer = byId("savedTweaksContainer", HTMLFieldSetElement);

// captured before Masonry adds its inline styles, to detect which cards changed
const cardHtml = new WeakMap<Element, string>();
for (const el of document.querySelectorAll(".appItem, .tweakItem")) {
  cardHtml.set(el, el.outerHTML);
}

const masonry = new Masonry(masonryRow, {
  percentPosition: true,
  horizontalOrder: true,
  itemSelector: ".appItem",
  columnWidth: "#appSizeItem",
  // cards jump to their place; animating every search keystroke sends them flying
  transitionDuration: 0,
});
const navMain = new Tab(byId("nav-main-tab", HTMLButtonElement));
const viewToggle = byId("viewToggle", HTMLDivElement);
const viewTweaks = byId("viewTweaks", HTMLDivElement);
const masonryTweaks = new Masonry(masonryTweaksRow, {
  percentPosition: true,
  horizontalOrder: true,
  itemSelector: ".tweakItem",
  columnWidth: "#tweakSizeItem",
  transitionDuration: 0,
});

function layoutGrids() {
  masonry.layout();
  masonryTweaks.layout();
}

// apps without an icon keep the placeholder underneath
document.addEventListener(
  "error",
  (e) => {
    if (e.target instanceof HTMLImageElement && e.target.parentElement?.classList.contains("app-icon")) {
      e.target.hidden = true;
    }
  },
  true,
);
for (const img of document.querySelectorAll<HTMLImageElement>(".app-icon img")) {
  if (img.complete && img.naturalWidth === 0) img.hidden = true;
}

// Masonry only measures cards at layout time, so a card that grows later would overlap the next
let relayoutFrame = 0;
const cardResizeObserver = new ResizeObserver(() => {
  cancelAnimationFrame(relayoutFrame);
  relayoutFrame = requestAnimationFrame(() => layoutGrids());
});
for (const el of document.querySelectorAll(".appItem, .tweakItem")) {
  cardResizeObserver.observe(el);
}
const btnUploadTweaks = byId("btnUploadTweaks", HTMLAnchorElement);
const tweakSelect = byId("tweakSelect", HTMLInputElement);
const tweakForm = byId("tweakForm", HTMLFormElement);
const tweakSubmit = byId("tweakSubmit", HTMLButtonElement);
const tweakSpinner = byId("tweakSpinner", HTMLDivElement);
const btnTweakModalClose = byId("btnTweakModalClose", HTMLButtonElement);
const tweakModal = new Modal(byId("tweakModal", HTMLDivElement), {
  backdrop: "static",
  keyboard: false,
});

// X-Requested-With gets a 204 instead of a redirect; following redirects would hide e.g. a proxy login bounce
async function request(url: string, options: RequestInit = {}) {
  const res = await fetch(url, {
    credentials: "same-origin",
    redirect: "manual",
    ...options,
    headers: { "X-Requested-With": "fetch" },
  });
  if (res.ok) return;
  let message = res.type === "opaqueredirect" ? "Unexpected redirect" : await res.text();
  try {
    message = JSON.parse(message).message || message;
  } catch {}
  throw new Error(message || `${res.status} ${res.statusText}`);
}

function reportError(err: unknown) {
  console.error(err);
  alert(errorMessage(err));
}

function uploadIds(result: UploadResult<UploadMeta, UploadBody> | undefined, uploadType?: UploadMeta["uploadType"]) {
  // Uppy resolves with nothing when the upload was cancelled midway
  if (!result) throw new Error("Upload was cancelled");
  const failed = result.failed ?? [];
  if (failed.length > 0) {
    throw new Error(failed.map((f) => f.error).join("\n"));
  }
  return (result.successful ?? [])
    .filter((f) => !uploadType || f.meta.uploadType === uploadType)
    .map((f) => {
      const id = f.uploadURL?.split("/").pop();
      if (!id) throw new Error(`No upload URL for ${f.name}`);
      return id;
    });
}

function trackProgress(uploader: Uppy<UploadMeta, UploadBody>, progress: HTMLElement) {
  const bar = query(progress, ".progress-bar", HTMLDivElement);
  uploader.on("progress", (percent) => (bar.style.width = `${percent}%`));
  return (visible: boolean) => {
    progress.hidden = !visible;
    bar.style.width = "0%";
  };
}

function newUploader(id: string) {
  return new Uppy<UploadMeta, UploadBody>({ id }).use(Tus, { endpoint: "/tus/", parallelUploads: 6 });
}

const navbar = query(document, ".navbar", HTMLElement);
const navbarRow = query(navbar, ".container", HTMLDivElement);
const navbarTiers = [[], ["nav-tight"], ["nav-tight", "nav-icon"], ["nav-stacked"]];
function fitNavbar() {
  // the last tier always fits
  for (const tier of navbarTiers) {
    navbar.classList.remove("nav-tight", "nav-icon", "nav-stacked");
    navbar.classList.add(...tier);
    if (navbarRow.scrollWidth <= navbarRow.clientWidth) return;
  }
}
let navbarWidth = 0;
new ResizeObserver(() => {
  // only refit on width changes; stacking changes the height and would retrigger
  if (navbar.clientWidth === navbarWidth) return;
  navbarWidth = navbar.clientWidth;
  fitNavbar();
}).observe(navbar);
void document.fonts.ready.then(fitNavbar);

function setView(view: string | undefined) {
  if (view !== "tweaks") view = "apps";
  viewToggle.dataset.view = view;
  for (const btn of viewToggle.querySelectorAll("button")) {
    const active = btn.dataset.view === view;
    btn.classList.toggle("active", active);
    btn.setAttribute("aria-selected", String(active));
  }
  const isTweaks = view === "tweaks";
  masonryRow.hidden = isTweaks;
  viewTweaks.hidden = !isTweaks;
  btnUploadApp.hidden = isTweaks;
  btnUploadTweaks.hidden = !isTweaks;
  inputSearchFilter.placeholder = isTweaks ? "Search tweak name" : "Search app name";
  fitNavbar();
  history.replaceState(null, "", isTweaks ? "#tweaks" : location.pathname + location.search);
  // the newly shown grid was laid out while hidden (zero width), so snap it into
  // place instead of animating every card from the corner
  applySearchFilter();
}
for (const btn of viewToggle.querySelectorAll("button")) {
  btn.addEventListener("click", () => setView(btn.dataset.view));
}
window.addEventListener("hashchange", () => setView(location.hash.slice(1)));

// re-run whenever a live update replaces the picker
let updateSavedTweaksSummary = () => {};
function initSavedTweaks() {
  const dropdown = document.getElementById("savedTweaksDropdown");
  if (!dropdown) {
    updateSavedTweaksSummary = () => {};
    return;
  }
  const button = byId("savedTweaksButton", HTMLButtonElement);
  const search = byId("savedTweaksSearch", HTMLInputElement);
  const noMatch = byId("savedTweaksNoMatch", HTMLDivElement);
  const options = [...dropdown.getElementsByClassName("savedTweakOption")].map((option) => ({
    option,
    input: query(option, "input", HTMLInputElement),
    name: query(option, ".savedTweakName", HTMLSpanElement).textContent?.trim() ?? "",
  }));
  updateSavedTweaksSummary = () => {
    const names = options.filter((o) => o.input.checked).map((o) => o.name);
    if (names.length === 0) button.textContent = "Select saved tweaks...";
    else if (names.length === 1) button.textContent = names[0] ?? "";
    else button.textContent = `${names.length} files`;
    button.title = names.join("\n");
  };
  for (const o of options) o.input.addEventListener("change", updateSavedTweaksSummary);
  search.addEventListener("input", () => {
    const q = search.value.toLowerCase();
    let visible = 0;
    for (const o of options) {
      const match = o.name.toLowerCase().includes(q);
      o.option.classList.toggle("d-none", !match);
      if (match) visible++;
    }
    noMatch.hidden = visible > 0;
  });
  // don't submit the form on Enter in the search box
  search.addEventListener("keydown", (e) => {
    if (e.key === "Enter") e.preventDefault();
  });
  dropdown.addEventListener("shown.bs.dropdown", () => search.focus());
  dropdown.addEventListener("hidden.bs.dropdown", () => {
    search.value = "";
    search.dispatchEvent(new Event("input"));
  });
  updateSavedTweaksSummary();
}
initSavedTweaks();

const tweakUppy = newUploader("tweaks");
const showTweakProgress = trackProgress(tweakUppy, byId("tweakProgress", HTMLDivElement));
function setTweakSubmitting(submitting: boolean) {
  tweakSubmit.disabled = submitting;
  btnTweakModalClose.disabled = submitting;
  tweakSpinner.style.display = submitting ? "inline-block" : "none";
  showTweakProgress(submitting);
}
function resetTweakForm() {
  tweakUppy.cancelAll();
  tweakForm.reset();
}
btnUploadTweaks.addEventListener("click", () => {
  resetTweakForm();
  tweakModal.show();
});
btnTweakModalClose.addEventListener("click", () => tweakModal.hide());
tweakSelect.addEventListener("change", () => {
  tweakUppy.cancelAll();
  for (const file of tweakSelect.files ?? []) {
    try {
      tweakUppy.addFile({ name: file.name, type: file.type, data: file });
    } catch (err) {
      reportError(err);
    }
  }
});
tweakSubmit.addEventListener("click", () => {
  if (!tweakForm.reportValidity()) return;
  setTweakSubmitting(true);
  tweakUppy
    .upload()
    .then(async (result) => {
      const body = new URLSearchParams({ [formTweakIds.name]: uploadIds(result).join(",") });
      await request("/tweaks", { method: "POST", body });
      tweakModal.hide();
      resetTweakForm();
    })
    .catch((err) => {
      // finished uploads may already be consumed by the server, so start over
      resetTweakForm();
      reportError(err);
    })
    .finally(() => setTweakSubmitting(false));
});

const uppy = newUploader("apps");
const showUploadProgress = trackProgress(uppy, byId("uploadProgress", HTMLDivElement));

function removeUploads(uploadType: NonNullable<UploadMeta["uploadType"]>) {
  for (const f of uppy.getFiles()) {
    if (f.meta.uploadType === uploadType) uppy.removeFile(f.id);
  }
}

function addTusFileHook(formItem: HTMLInputElement, uploadType: NonNullable<UploadMeta["uploadType"]>) {
  formItem.addEventListener("change", () => {
    removeUploads(uploadType);
    for (const file of formItem.files ?? []) {
      try {
        uppy.addFile({ name: file.name, type: file.type, data: file, meta: { uploadType } });
      } catch (err) {
        reportError(err);
        formItem.value = "";
      }
    }
  });
}

addTusFileHook(formFileSelect, "ipa");
addTusFileHook(formTweaksSelect, "tweak");

modalElem.addEventListener("shown.bs.modal", () => {
  modalBody.style.minHeight = `30.5rem`;
});

// the server consumes uploads even when signing fails, so they can't be reused
function resetUploads() {
  uppy.cancelAll();
  formFileSelect.value = "";
  formTweaksSelect.value = "";
  formTweakIds.value = "";
  if (formFileText.hidden) formFileId.value = "";
}

function resetUploadForm() {
  uploadForm.reset();
  resetUploads();
  formIdCustomText.disabled = true;
  formProfile.dispatchEvent(new Event("change"));
  formTweaksUseLibrary.dispatchEvent(new Event("change"));
  updateSavedTweaksSummary();
  navMain.show();
}

function setSubmitting(submitting: boolean) {
  formSubmit.disabled = submitting;
  btnModalClose.disabled = submitting;
  formFileSpinner.style.display = submitting ? "inline-block" : "none";
  showUploadProgress(submitting);
}

btnUploadApp.addEventListener("click", () => {
  formFileId.value = "";
  formFileText.hidden = true;
  formFileText.value = "";
  formFileSelect.hidden = false;
  formFileSelect.disabled = false;
  formFileUrl.hidden = true;
  formFileUrl.disabled = true;
  formFileUseUrl.disabled = false;
  formFileUseUrl.checked = false;
  modal.show();
});
btnModalClose.addEventListener("click", () => modal.hide());
formSubmit.addEventListener("click", () => {
  if (!uploadForm.checkValidity()) {
    // only first tab has required inputs
    navMain.show();
    uploadForm.reportValidity();
    return;
  }
  setSubmitting(true);

  uppy
    .upload()
    .then(async (result) => {
      if (!formFileId.value) formFileId.value = uploadIds(result, "ipa")[0] ?? "";
      if (!formTweakIds.value) formTweakIds.value = uploadIds(result, "tweak").join(",");
      await request(uploadForm.action, { method: "POST", body: new FormData(uploadForm) });
      modal.hide();
      resetUploadForm();
    })
    .catch((err) => {
      console.error(err);
      resetUploads();
      alert(`${errorMessage(err)}\n\nPlease select your files again.`);
    })
    .finally(() => setSubmitting(false));
});

formTweaksUseLibrary.addEventListener("change", () => {
  const upload = !formTweaksUseLibrary.checked;
  savedTweaksContainer.hidden = upload;
  savedTweaksContainer.disabled = upload;
  formTweaksSelect.hidden = !upload;
  formTweaksSelect.disabled = !upload;
  if (!upload) {
    removeUploads("tweak");
    formTweaksSelect.value = "";
  }
});
formFileUseUrl.addEventListener("change", () => {
  formFileSelect.hidden = !formFileSelect.hidden;
  formFileSelect.disabled = !formFileSelect.disabled;
  formFileUrl.hidden = !formFileUrl.hidden;
  formFileUrl.disabled = !formFileUrl.disabled;
});
formProfile.addEventListener("change", () => {
  const selectedOption = formProfile.options[formProfile.selectedIndex];
  if (selectedOption?.getAttribute("account") === "true") {
    if (formIdProv.checked) {
      formIdOriginal.checked = true;
    }
    formIdProv.disabled = true;
    formIdEncode.checked = true;
    formIdEncode.disabled = false;
  } else {
    formIdProv.disabled = false;
    formIdEncode.checked = false;
    formIdEncode.disabled = true;
  }
});
for (const radio of document.getElementsByName(formIdOriginal.name)) {
  radio.addEventListener("change", () => {
    formIdCustomText.disabled = !formIdCustom.checked;
  });
}

// delegated, so cards inserted by live updates work too
document.addEventListener("click", (e) => {
  const target = eventElement(e);
  if (!target) return;
  const createFrom = target.closest(".dropdownCreateFrom");
  if (createFrom) {
    formFileId.value = createFrom.getAttribute("x-app-id") ?? "";
    formFileText.hidden = false;
    formFileText.value = createFrom.getAttribute("x-app-name") ?? "";
    formFileSelect.hidden = true;
    formFileSelect.disabled = true;
    formFileUrl.hidden = true;
    formFileUrl.disabled = true;
    formFileUseUrl.disabled = true;
    formFileUseUrl.checked = false;
    modal.show();
    return;
  }
  if (target.closest("#linkTweaksView")) {
    modal.hide();
    return;
  }
  const action = target.closest("a[data-action]");
  if (action instanceof HTMLAnchorElement) {
    e.preventDefault();
    if (action.dataset.confirm && !confirm(action.dataset.confirm)) return;
    request(action.href).catch(reportError);
  }
});

function applySearchFilter() {
  const searchText = inputSearchFilter.value.toLowerCase();
  for (const item of document.querySelectorAll<HTMLElement>(".appItem, .tweakItem")) {
    const title = item.querySelector(".card-title")?.textContent?.toLowerCase() ?? "";
    item.hidden = searchText !== "" && !title.includes(searchText);
  }
  layoutGrids();
}
inputSearchFilter.addEventListener("input", () => applySearchFilter());

function clearSearch() {
  inputSearchFilter.value = "";
  inputSearchFilter.dispatchEvent(new Event("input"));
}
inputSearchFilter.addEventListener("keydown", (e) => {
  if (e.key === "Escape") clearSearch();
});
btnSearchClear.addEventListener("click", clearSearch);

// enable all tooltips
for (const el of document.querySelectorAll('[data-bs-toggle="tooltip"]')) {
  new Tooltip(el);
}

// created lazily since live updates replace cards; focus is what makes taps work on touch screens
function showTweakNames(e: Event) {
  const el = eventElement(e)?.closest(".tweak-count");
  if (!el || Tooltip.getInstance(el)) return;
  Tooltip.getOrCreateInstance(el, { trigger: "hover focus", customClass: "tweak-tooltip" }).show();
}
document.addEventListener("mouseover", showTweakNames);
document.addEventListener("focusin", showTweakNames);

function releaseCard(card: Element) {
  for (const el of card.querySelectorAll(".tweak-count")) {
    Tooltip.getInstance(el)?.dispose();
  }
  cardResizeObserver.unobserve(card);
}

interface LiveState {
  apps: string;
  tweaks: string;
  savedTweaks: string;
}

// unchanged cards are left alone, so e.g. an open menu stays open
function patchGrid(row: HTMLElement, html: string, itemClass: string, m: Masonry) {
  const tpl = document.createElement("template");
  tpl.innerHTML = html;
  const incoming = [...tpl.content.children].filter(
    (el): el is HTMLElement => el instanceof HTMLElement && el.classList.contains(itemClass),
  );
  const current = new Map(
    [...row.getElementsByClassName(itemClass)]
      .filter((el): el is HTMLElement => el instanceof HTMLElement)
      .map((el) => [el.dataset.id, el]),
  );
  let changed = false;
  // the masonry size item stays first
  let prev = row.firstElementChild;
  if (!prev) throw new Error("grid has no size item");
  for (const next of incoming) {
    const nextHtml = next.outerHTML;
    let el = current.get(next.dataset.id);
    current.delete(next.dataset.id);
    if (el && cardHtml.get(el) !== nextHtml) {
      releaseCard(el);
      el.replaceWith(next);
      el = undefined;
    }
    if (!el) {
      el = next;
      cardHtml.set(el, nextHtml);
      cardResizeObserver.observe(el);
      changed = true;
    }
    if (prev.nextElementSibling !== el) {
      prev.after(el);
      changed = true;
    }
    prev = el;
  }
  for (const el of current.values()) {
    releaseCard(el);
    el.remove();
    changed = true;
  }
  if (changed) m.reloadItems();
  return changed;
}

// deferred while the picker is open, so it doesn't close under the cursor
let savedTweaksHtml: string | null = null;
let pendingSavedTweaksHtml: string | null = null;
function setSavedTweaks(html: string) {
  if (html === savedTweaksHtml) return;
  if (savedTweaksContainer.querySelector(".dropdown-menu.show")) {
    pendingSavedTweaksHtml = html;
    return;
  }
  const checkboxes = () => savedTweaksContainer.querySelectorAll<HTMLInputElement>("input[type=checkbox]");
  const checked = new Set([...checkboxes()].filter((i) => i.checked).map((i) => i.value));
  savedTweaksContainer.innerHTML = html;
  savedTweaksHtml = html;
  for (const input of checkboxes()) {
    input.checked = checked.has(input.value);
  }
  initSavedTweaks();
}
savedTweaksContainer.addEventListener("hidden.bs.dropdown", () => {
  if (pendingSavedTweaksHtml === null) return;
  const html = pendingSavedTweaksHtml;
  pendingSavedTweaksHtml = null;
  setSavedTweaks(html);
});

function applyLiveState(state: LiveState) {
  let changed = patchGrid(masonryRow, state.apps, "appItem", masonry);
  changed = patchGrid(masonryTweaksRow, state.tweaks, "tweakItem", masonryTweaks) || changed;
  tweaksEmpty.hidden = masonryTweaksRow.getElementsByClassName("tweakItem").length > 0;
  setSavedTweaks(state.savedTweaks);
  if (changed) applySearchFilter();
}

function setLiveStatus(state: "live" | "offline", text: string, title: string) {
  liveStatus.dataset.state = state;
  liveStatusText.textContent = text;
  liveStatus.title = title;
  fitNavbar();
}

function connectLive() {
  const events = new EventSource("/events");
  events.addEventListener("open", () => setLiveStatus("live", "Live", "Receiving live updates"));
  events.addEventListener("state", (e) => {
    try {
      applyLiveState(JSON.parse(e.data) as LiveState);
    } catch (err) {
      console.error(err);
    }
  });
  events.addEventListener("error", () => {
    setLiveStatus("offline", "Reconnecting", "Lost connection to server, reconnecting...");
    // the browser retries on its own, except when the server rejected the request
    if (events.readyState === EventSource.CLOSED) {
      events.close();
      setTimeout(connectLive, 5_000);
    }
  });
}

const btnTheme = byId("btnTheme", HTMLButtonElement);
const themeIcon = byId("themeIcon", HTMLElement);
function updateThemeButton() {
  const dark = window.theme.current() === "dark";
  themeIcon.className = dark ? "bi bi-moon-stars-fill" : "bi bi-sun-fill";
  const label = `${dark ? "Dark" : "Light"} theme (${window.theme.isAuto() ? "matching system" : "manual"})`;
  btnTheme.title = `${label}, click to switch`;
  btnTheme.setAttribute("aria-label", btnTheme.title);
}
btnTheme.addEventListener("click", () => window.theme.toggle());
document.addEventListener("themechange", updateThemeButton);
updateThemeButton();

setView(location.hash.slice(1));
connectLive();
