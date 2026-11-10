// Runs in <head> before the page renders, so there's no flash of the wrong theme. An override is
// only stored while it differs from the system theme; matching it again returns to auto.
(() => {
  const key = "theme";
  const media = matchMedia("(prefers-color-scheme: dark)");
  const system = () => (media.matches ? "dark" : "light");
  const stored = () => {
    const value = localStorage.getItem(key);
    return value === "dark" || value === "light" ? value : null;
  };
  const current = () => stored() ?? system();

  function apply() {
    document.documentElement.dataset.bsTheme = current();
    document.dispatchEvent(new Event("themechange"));
  }

  window.theme = {
    current,
    isAuto: () => stored() === null,
    toggle() {
      const next = current() === "dark" ? "light" : "dark";
      if (next === system()) localStorage.removeItem(key);
      else localStorage.setItem(key, next);
      apply();
    },
  };

  media.addEventListener("change", () => {
    if (stored() === system()) localStorage.removeItem(key);
    apply();
  });
  // keep other open tabs in sync
  window.addEventListener("storage", (e) => {
    if (e.key === key) apply();
  });
  apply();
})();
