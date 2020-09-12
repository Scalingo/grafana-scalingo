export * from './colors';
export * from './validate';
export * from './slate';
export * from './dataLinks';
export * from './tags';
export * from './measureText';
export { default as ansicolor } from './ansicolor';

import * as DOMUtil from './dom'; // includes Element.closest polyfil
export { DOMUtil };
