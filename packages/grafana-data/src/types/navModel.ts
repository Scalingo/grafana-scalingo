import { ComponentType } from 'react';

export interface NavLinkDTO {
  id?: string;
  text: string;
  description?: string;
  section?: NavSection;
  subTitle?: string;
  icon?: string;
  img?: string;
  url?: string;
  target?: string;
  sortWeight?: number;
  divider?: boolean;
  hideFromMenu?: boolean;
  hideFromTabs?: boolean;
  children?: NavLinkDTO[];
  highlightText?: string;
}

export interface NavModelItem extends NavLinkDTO {
  children?: NavModelItem[];
  active?: boolean;
  breadcrumbs?: NavModelBreadcrumb[];
  parentItem?: NavModelItem;
  showOrgSwitcher?: boolean;
  onClick?: () => void;
  menuItemType?: NavMenuItemType;
  highlightText?: string;
  highlightId?: string;
  tabSuffix?: ComponentType<{ className?: string }>;
  showIconInNavbar?: boolean;
}

export enum NavSection {
  Core = 'core',
  Plugin = 'plugin',
  Config = 'config',
}

export enum NavMenuItemType {
  Section = 'section',
  Item = 'item',
}

/**
 *  Interface used to describe  different kinds of page titles and page navigation. Navmodels are usually generated in the backend and stored in Redux.
 */
export interface NavModel {
  /**
   *  Main page. that wraps the navigation. Generate the `children` property generate tabs when used with the Page component.
   */
  main: NavModelItem;
  /**
   *   This is the current active tab/navigation.
   */
  node: NavModelItem;
  /**
   *  Describes breadcrumbs that are used in places such as data source settings., folder page and plugins page.
   */
  breadcrumbs?: NavModelItem[];
}

export interface NavModelBreadcrumb {
  title: string;
  url?: string;
}

export type NavIndex = { [s: string]: NavModelItem };
