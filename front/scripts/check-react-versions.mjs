// React refuses to render when react and react-dom differ, even by a patch
// version: every page is blank (minified error #527). Fail the build instead.
import { readFileSync } from 'node:fs';

const version = (pkg) =>
  JSON.parse(readFileSync(new URL(`../node_modules/${pkg}/package.json`, import.meta.url), 'utf8')).version;

const react = version('react');
const reactDom = version('react-dom');
if (react !== reactDom) {
  console.error(`react ${react} and react-dom ${reactDom} must have the same version: upgrade them together.`);
  process.exit(1);
}
