import AdmZip from 'adm-zip';
import * as fs from 'fs';
import * as path from 'path';
import { sha256Hex } from '../lib/hashes';

function walkFiles(rootDir: string): string[] {
  const files: string[] = [];
  const walk = (dir: string) => {
    for (const name of fs.readdirSync(dir)) {
      if (name === '.DS_Store') continue;
      const full = path.join(dir, name);
      const stat = fs.statSync(full);
      if (stat.isDirectory()) walk(full);
      else if (stat.isFile()) files.push(path.relative(rootDir, full).split(path.sep).join('/'));
    }
  };
  walk(rootDir);
  return files.sort();
}

/**
 * Compute SHA-256 digests for every part declared in the manifest and write
 * the updated manifest back to the package directory.
 */
function writeHashes(dir: string): void {
  const manifestPath = path.join(dir, 'manifest.json');
  const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'));
  const refs = [
    manifest.workproduct,
    manifest.provenance,
    ...(manifest.parts ?? []),
    ...(manifest.signatures ?? []),
  ].filter((ref) => ref?.path);
  for (const ref of refs) {
    const filePath = path.join(dir, ...ref.path.split('/'));
    if (!fs.existsSync(filePath)) {
      throw new Error(`manifest declares part that does not exist: ${ref.path}`);
    }
    ref.sha256 = sha256Hex(fs.readFileSync(filePath));
  }
  fs.writeFileSync(manifestPath, JSON.stringify(manifest, null, 2) + '\n');
}

export function packCommand(dir: string, output: string, options: { writeHashes?: boolean }): number {
  if (!fs.existsSync(path.join(dir, 'manifest.json'))) {
    console.error(`error: ${dir} does not contain manifest.json`);
    return 2;
  }
  if (options.writeHashes) writeHashes(dir);

  const zip = new AdmZip();
  for (const relPath of walkFiles(dir)) {
    zip.addFile(relPath, fs.readFileSync(path.join(dir, ...relPath.split('/'))));
  }
  fs.mkdirSync(path.dirname(path.resolve(output)), { recursive: true });
  zip.writeZip(output);
  console.log(`packed ${dir} -> ${output}`);
  return 0;
}
