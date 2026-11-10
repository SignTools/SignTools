import { Modal } from "bootstrap";
import { byId } from "./dom";

const formToken = byId("formToken", HTMLInputElement);

byId("uploadForm", HTMLFormElement).addEventListener("submit", (event) => {
  formToken.classList.remove("is-invalid", "is-valid");
  if (formToken.value.length !== 6) {
    event.preventDefault();
    event.stopPropagation();
    formToken.classList.add("is-invalid");
  }
});

new Modal(byId("uploadModal", HTMLDivElement), { backdrop: "static", keyboard: false }).show();
formToken.focus();
