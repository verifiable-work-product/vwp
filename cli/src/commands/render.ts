import { openPackage } from '../lib/package';

const RENDERING_CANDIDATES = ['renderings/memo.md', 'renderings/workpaper.md'];

/**
 * v0.1 rendering support: print the package's markdown rendering if present.
 * Renderings are derived views; the structured package is the source of truth.
 */
export function renderCommand(input: string): number {
  const pkg = openPackage(input);
  const found =
    RENDERING_CANDIDATES.find((p) => pkg.has(p)) ??
    pkg.list().find((p) => p.startsWith('renderings/') && p.endsWith('.md'));
  if (!found) {
    console.error('error: package contains no markdown rendering under renderings/');
    return 2;
  }
  process.stdout.write(pkg.read(found).toString('utf8'));
  return 0;
}
