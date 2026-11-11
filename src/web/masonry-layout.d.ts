// @types/masonry-layout marks every method optional and pulls in jQuery
declare module "masonry-layout" {
  namespace Masonry {
    interface Options {
      itemSelector?: string;
      columnWidth?: string | number | Element;
      percentPosition?: boolean;
      horizontalOrder?: boolean;
      transitionDuration?: number | string;
    }
  }
  class Masonry {
    constructor(element: Element | string, options?: Masonry.Options);
    layout(): void;
    reloadItems(): void;
  }
  export = Masonry;
}
