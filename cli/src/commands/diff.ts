import { openPackage } from '../lib/package';
import { diffPackages } from '../lib/diff';

export function diffCommand(oldInput: string, newInput: string): number {
  const oldPkg = openPackage(oldInput);
  const newPkg = openPackage(newInput);
  const lines = diffPackages(oldPkg, newPkg);

  if (lines.length === 0) {
    console.log('no structural differences');
    return 0;
  }
  for (const line of lines) {
    const marker = line.change === 'added' ? '+' : line.change === 'removed' ? '-' : '~';
    console.log(`${marker} ${line.kind} ${line.id}${line.detail ? `: ${line.detail}` : ''}`);
  }
  console.log('');
  console.log(`${lines.length} difference${lines.length === 1 ? '' : 's'}`);
  return 0;
}
