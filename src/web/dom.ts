type ElementClass<T extends Element> = { new (): T; prototype: T };

// fails loudly if a template's id was renamed, instead of with a null error later
export function byId<T extends HTMLElement>(id: string, type: ElementClass<T>): T {
  const el = document.getElementById(id);
  if (!(el instanceof type)) {
    throw new Error(`#${id}: expected ${type.name}, found ${el?.constructor.name ?? "nothing"}`);
  }
  return el;
}

export function query<T extends Element>(root: ParentNode, selector: string, type: ElementClass<T>): T {
  const el = root.querySelector(selector);
  if (!(el instanceof type)) {
    throw new Error(`${selector}: expected ${type.name}, found ${el?.constructor.name ?? "nothing"}`);
  }
  return el;
}

export function eventElement(e: Event): Element | null {
  return e.target instanceof Element ? e.target : null;
}

export function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}
