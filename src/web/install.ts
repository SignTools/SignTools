import { byId } from "./dom";

const href = byId("manifest", HTMLDivElement).getAttribute("href");
if (href) window.location.href = href;
