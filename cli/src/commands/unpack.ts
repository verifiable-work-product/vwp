import AdmZip from 'adm-zip';
import * as fs from 'fs';
import * as path from 'path';
import { isUnsafePath } from '../lib/package';

/**
 * Extract a .vwp package. Entries with unsafe paths (absolute, drive-letter,
 * backslash, or '..' segments) are refused and abort the extraction.
 */
export function unpackCommand(input: string, outDir: string): number {
  const zip = new AdmZip(input);
  const entries = zip.getEntries().filter((e) => !e.isDirectory);

  const unsafe = entries.map((e) => e.entryName).filter(isUnsafePath);
  if (unsafe.length > 0) {
    console.error(`error: refusing to unpack; unsafe entry paths: ${unsafe.join(', ')}`);
    return 2;
  }

  const resolvedOut = path.resolve(outDir);
  for (const entry of entries) {
    const target = path.resolve(resolvedOut, ...entry.entryName.split('/'));
    // Defense in depth: never write outside the output directory.
    if (target !== resolvedOut && !target.startsWith(resolvedOut + path.sep)) {
      console.error(`error: refusing to write outside output directory: ${entry.entryName}`);
      return 2;
    }
    fs.mkdirSync(path.dirname(target), { recursive: true });
    fs.writeFileSync(target, entry.getData());
  }
  console.log(`unpacked ${input} -> ${outDir} (${entries.length} files)`);
  return 0;
}
