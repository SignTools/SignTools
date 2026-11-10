// @types/masonry-layout marks every method optional, lacks `options` and pulls in jQuery
declare module "masonry-layout" {
  namespace Masonry {
    interface Options {
      itemSelector?: string;
      columnWidth?: string | number | Element;
      percentPosition?: boolean;
      horizontalOrder?: boolean;
      // undocumented: skips the transition
      layoutInstant?: boolean;
    }
  }
  class Masonry {
    constructor(element: Element | string, options?: Masonry.Options);
    options: Masonry.Options;
    layout(): void;
    reloadItems(): void;
  }
  export = Masonry;
}
