import AdmZip from 'adm-zip';
import * as fs from 'fs';
import * as path from 'path';

/**
 * A readable VWP package, backed by either an unpacked directory or a .vwp
 * ZIP archive. All paths are forward-slash relative paths within the package.
 */
export interface PackageSource {
  /** Where the package was loaded from (for display). */
  origin: string;
  /** Relative paths of all safe files in the package. */
  list(): string[];
  has(relPath: string): boolean;
  read(relPath: string): Buffer;
  /** Entry names that were rejected as unsafe (path traversal, absolute paths). */
  unsafeEntries: string[];
}

/**
 * A package path is unsafe if it is absolute, contains a drive letter or
 * backslash, or has any '..' segment. Verifiers must reject such paths and
 * unpack must never write them to disk.
 */
export function isUnsafePath(relPath: string): boolean {
  if (relPath.length === 0) return true;
  if (relPath.includes('\\') || relPath.includes('\0')) return true;
  if (relPath.startsWith('/') || /^[A-Za-z]:/.test(relPath)) return true;
  return relPath.split('/').some((seg) => seg === '..');
}

class DirSource implements PackageSource {
  unsafeEntries: string[] = [];
  private files: string[];

  constructor(public origin: string, private rootDir: string) {
    this.files = [];
    const walk = (dir: string) => {
      for (const name of fs.readdirSync(dir)) {
        if (name === '.DS_Store') continue;
        const full = path.join(dir, name);
        const stat = fs.statSync(full);
        if (stat.isDirectory()) {
          walk(full);
        } else if (stat.isFile()) {
          this.files.push(path.relative(rootDir, full).split(path.sep).join('/'));
        }
      }
    };
    walk(rootDir);
    this.files.sort();
  }

  list(): string[] {
    return [...this.files];
  }

  has(relPath: string): boolean {
    return this.files.includes(relPath);
  }

  read(relPath: string): Buffer {
    if (isUnsafePath(relPath)) throw new Error(`unsafe package path: ${relPath}`);
    return fs.readFileSync(path.join(this.rootDir, ...relPath.split('/')));
  }
}

class ZipSource implements PackageSource {
  unsafeEntries: string[] = [];
  private entries = new Map<string, Buffer>();

  constructor(public origin: string, zipPath: string) {
    const zip = new AdmZip(zipPath);
    for (const entry of zip.getEntries()) {
      if (entry.isDirectory) continue;
      const name = entry.entryName;
      if (isUnsafePath(name)) {
        this.unsafeEntries.push(name);
        continue;
      }
      this.entries.set(name, entry.getData());
    }
  }

  list(): string[] {
    return [...this.entries.keys()].sort();
  }

  has(relPath: string): boolean {
    return this.entries.has(relPath);
  }

  read(relPath: string): Buffer {
    const data = this.entries.get(relPath);
    if (!data) throw new Error(`no such file in package: ${relPath}`);
    return data;
  }
}

/** Open a VWP package from a .vwp file or an unpacked package directory. */
export function openPackage(input: string): PackageSource {
  const stat = fs.statSync(input);
  if (stat.isDirectory()) return new DirSource(input, input);
  return new ZipSource(input, input);
}

/** Read and parse a JSON file from the package, or return undefined. */
export function readJson(pkg: PackageSource, relPath: string): unknown {
  if (!pkg.has(relPath)) return undefined;
  return JSON.parse(pkg.read(relPath).toString('utf8'));
}
