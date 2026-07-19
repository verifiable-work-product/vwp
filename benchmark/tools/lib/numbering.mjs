// The shared pagination convention for numbered evidence copies.
//
// A "numbered copy" is the rendering of a text evidence file that CPAs read
// and cite: fixed lines per page, page-relative line numbers in the margin,
// like a deposition transcript. number-evidence.mjs produces it and
// transcribe-worksheet.mjs resolves pin cites against it, so both must use
// this module — the convention IS the anchor semantics for human citations.

export const LINES_PER_PAGE = 50;

/** Split evidence text into its physical lines (the unit of citation). */
export function evidenceLines(text) {
  const lines = text.split(/\r?\n/);
  // A trailing newline produces one empty final element; it is not a citable line.
  if (lines.length > 0 && lines[lines.length - 1] === '') lines.pop();
  return lines;
}

/** Render the numbered copy: page headers and page-relative line numbers. */
export function renderNumberedCopy(text, title) {
  const lines = evidenceLines(text);
  const out = [];
  for (let i = 0; i < lines.length; i++) {
    const page = Math.floor(i / LINES_PER_PAGE) + 1;
    const lineOnPage = (i % LINES_PER_PAGE) + 1;
    if (lineOnPage === 1) {
      if (page > 1) out.push('');
      out.push(`==== ${title ?? 'EVIDENCE'} — PAGE ${page} ====`);
    }
    out.push(`${String(lineOnPage).padStart(3)} | ${lines[i]}`);
  }
  return out.join('\n') + '\n';
}

/**
 * Resolve a pin cite (page, from..to page-relative lines) to the cited text.
 * Returns undefined when the cite falls outside the document.
 */
export function resolvePinCite(text, page, from, to) {
  const lines = evidenceLines(text);
  const start = (page - 1) * LINES_PER_PAGE + (from - 1);
  const end = (page - 1) * LINES_PER_PAGE + (to ?? from);
  if (start < 0 || start >= lines.length || end > lines.length || end <= start) return undefined;
  return lines.slice(start, end).join('\n');
}
