import { routerOptions } from "./index";
import { NavigationItem } from "./navigation";

const routePathToTitle = (path: string): string => {
  try {
    return path
      .replace(/^\/+/, "") // Remove leading slashes
      .replace(/-/g, " ") // Replace hyphens with spaces
      .trim() // Trim whitespace from both ends
      .replace(/^\w/, (c) => c.toUpperCase()); // Capitalize the first character
  } catch (e) {
    return path;
  }
};

export const processNavigationItems = (items: NavigationItem[]): NavigationItem[] => {
  return items.filter((item) => !item.hide).map((item) => {
    const newItem: NavigationItem = { ...item };

    if (newItem.value && !newItem.label) {
      newItem.label = routerOptions[newItem.value]?.title || routePathToTitle(newItem.value);
    }

    if (newItem.submenu && newItem.submenu.length > 0) {
      newItem.submenu = processNavigationItems(newItem.submenu);
    }

    return newItem;
  });
};
