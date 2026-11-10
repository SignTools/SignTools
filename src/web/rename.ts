import { Modal } from "bootstrap";
import { byId } from "./dom";

new Modal(byId("uploadModal", HTMLDivElement), { backdrop: "static", keyboard: false }).show();
byId("formName", HTMLInputElement).focus();
