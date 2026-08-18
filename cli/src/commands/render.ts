import { openPackage } from '../lib/package';

const RENDERING_CANDIDATES = ['renderings/memo.md', 'renderings/workpaper.md'];

/**
 * Print the package's Markdown readable if present. The preferred v0.2 role
 * is `readable`; v0.1 `rendering` roles and path conventions remain supported.
 * Renderings are derived views; the structured package is the source of truth.
 */
export function renderCommand(input: string): number {
  const pkg = openPackage(input);
  let parts: Array<{ path?: string; mediaType?: string; role?: string }> = [];
  if (pkg.has('manifest.json')) {
    try {
      const manifest = JSON.parse(pkg.read('manifest.json').toString('utf8'));
      if (Array.isArray(manifest.parts)) parts = manifest.parts;
    } catch {
      // Preserve the path-convention fallback for malformed/early manifests.
    }
  }
  const declaredMarkdown = (role: string) =>
    parts
      .filter(
        (part) =>
          part.role === role &&
          part.mediaType === 'text/markdown' &&
          typeof part.path === 'string' &&
          /\.md$/i.test(part.path) &&
          pkg.has(part.path)
      )
      .map((part) => part.path as string)
      .sort()[0];
  const found =
    declaredMarkdown('readable') ??
    declaredMarkdown('rendering') ??
    RENDERING_CANDIDATES.find((p) => pkg.has(p)) ??
    pkg.list().find((p) => p.startsWith('renderings/') && /\.md$/i.test(p));
  if (!found) {
    console.error('error: package contains no Markdown readable');
    return 2;
  }
  process.stdout.write(pkg.read(found).toString('utf8'));
  return 0;
}
