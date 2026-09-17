// VWP-Bench rubric math — the open reference implementation of the scoring
// rules in benchmark/rubric/README.md. The score-rubric.mjs CLI is a thin
// wrapper over this. Pure functions; no I/O.
//
// Rules (proposal §4): yes/no items earn full or zero credit; count items earn
// proportional credit (6 of 8 → 15/20); 0–5 ratings convert to points by a
// fixed multiplier; any flagged cap condition holds the signability rating at
// capMaxRating or below regardless of total. Two blind graders score every
// work product; a third decides only the items they disagree on.

const round2 = (n) => Math.round(n * 100) / 100;

/** Score one grade sheet against a rubric.
 * @returns {{areaPoints: Record<string, number>, total: number, signabilityRating: number|null,
 *            effectiveSignability: number|null, capApplied: boolean, complete: boolean}} */
export function computeGradeScore(rubric, sheet) {
  const capAreaKeys = rubric.capAreaKeys ?? ['signability'];
  const capMaxRating = rubric.capMaxRating ?? 3;
  const capFlags = sheet.capFlags ?? {};
  const capApplied = (rubric.capConditions ?? []).some((c) => capFlags[c.key] === true);
  const areaPoints = {};
  let complete = true;
  let signabilityRating = null;
  let effectiveSignability = null;

  for (const area of rubric.areas) {
    const entry = sheet.areas?.[area.key];
    let pts = 0;
    if (area.kind === 'yesNoItems') {
      const answers = entry?.kind === 'yesNoItems' ? entry.answers ?? {} : {};
      for (const item of area.items ?? []) {
        const a = answers[item.key];
        if (a === true) pts += item.points;
        if (a === null || a === undefined) complete = false;
      }
    } else if (area.kind === 'proportional') {
      const e = entry?.kind === 'proportional' ? entry : null;
      if (e?.captured == null || e?.total == null) complete = false;
      else pts = area.points * Math.min(1, Math.max(0, e.captured / e.total));
    } else if (area.kind === 'rating') {
      const e = entry?.kind === 'rating' ? entry : null;
      if (e?.rating == null) complete = false;
      else {
        let rating = e.rating;
        if (capAreaKeys.includes(area.key)) {
          signabilityRating = rating;
          if (capApplied) rating = Math.min(rating, capMaxRating);
          effectiveSignability = rating;
        }
        pts = rating * (area.ratingMultiplier ?? 1);
      }
    } else {
      throw new Error(`unknown area kind '${area.kind}' in area '${area.key}'`);
    }
    areaPoints[area.key] = round2(pts);
  }
  const total = round2(Object.values(areaPoints).reduce((s, p) => s + p, 0));
  return { areaPoints, total, signabilityRating, effectiveSignability, capApplied, complete };
}

/** Item-level diff of two sheets. Cap flags count as items — they gate the cap.
 * @returns {{areaKey: string, itemKey: string|null, field: 'answer'|'captured'|'rating'|'capFlag'}[]} */
export function diffGradeSheets(rubric, a, b) {
  const out = [];
  for (const area of rubric.areas) {
    const ea = a.areas?.[area.key];
    const eb = b.areas?.[area.key];
    if (area.kind === 'yesNoItems') {
      const aa = ea?.kind === 'yesNoItems' ? ea.answers ?? {} : {};
      const ab = eb?.kind === 'yesNoItems' ? eb.answers ?? {} : {};
      for (const item of area.items ?? []) {
        if ((aa[item.key] ?? null) !== (ab[item.key] ?? null)) out.push({ areaKey: area.key, itemKey: item.key, field: 'answer' });
      }
    } else if (area.kind === 'proportional') {
      const ca = ea?.kind === 'proportional' ? ea.captured ?? null : null;
      const cb = eb?.kind === 'proportional' ? eb.captured ?? null : null;
      const ta = ea?.kind === 'proportional' ? ea.total ?? null : null;
      const tb = eb?.kind === 'proportional' ? eb.total ?? null : null;
      if (ca !== cb || ta !== tb) out.push({ areaKey: area.key, itemKey: null, field: 'captured' });
    } else {
      const ra = ea?.kind === 'rating' ? ea.rating ?? null : null;
      const rb = eb?.kind === 'rating' ? eb.rating ?? null : null;
      if (ra !== rb) out.push({ areaKey: area.key, itemKey: null, field: 'rating' });
    }
  }
  for (const cap of rubric.capConditions ?? []) {
    if ((a.capFlags?.[cap.key] ?? false) !== (b.capFlags?.[cap.key] ?? false)) {
      out.push({ areaKey: '', itemKey: cap.key, field: 'capFlag' });
    }
  }
  return out;
}

/** Merge two primary sheets into the consensus sheet: agreed entries carry
 * over; disputed entries resolve from the tiebreak sheet when it answers them.
 * The consensus is scoreable only when `unresolved` is empty. */
export function mergeGradeSheets(rubric, a, b, tiebreak = null) {
  const disagreements = diffGradeSheets(rubric, a, b);
  const unresolved = [];
  const merged = JSON.parse(JSON.stringify(a));
  merged.areas ??= {};
  merged.capFlags ??= {};
  for (const d of disagreements) {
    if (d.field === 'capFlag') {
      const t = tiebreak?.capFlags?.[d.itemKey];
      if (t === undefined) unresolved.push(d);
      else merged.capFlags[d.itemKey] = t;
      continue;
    }
    const te = tiebreak?.areas?.[d.areaKey];
    const me = merged.areas[d.areaKey];
    if (d.field === 'answer') {
      const t = te?.kind === 'yesNoItems' ? te.answers?.[d.itemKey] ?? null : null;
      if (t === null) unresolved.push(d);
      else if (me?.kind === 'yesNoItems') (me.answers ??= {})[d.itemKey] = t;
    } else if (d.field === 'captured') {
      const ok = te?.kind === 'proportional' && te.captured != null && te.total != null;
      if (!ok) unresolved.push(d);
      else if (me?.kind === 'proportional') {
        me.captured = te.captured;
        me.total = te.total;
      }
    } else {
      const t = te?.kind === 'rating' ? te.rating ?? null : null;
      if (t === null) unresolved.push(d);
      else if (me?.kind === 'rating') me.rating = t;
    }
  }
  return { sheet: merged, unresolved };
}

/** Structural lint of a rubric beyond the JSON schema: area points must match
 * their items / multiplier, keys must be unique. Returns a list of problems. */
export function lintRubric(rubric) {
  const problems = [];
  const seen = new Set();
  for (const area of rubric.areas ?? []) {
    if (seen.has(area.key)) problems.push(`duplicate area key '${area.key}'`);
    seen.add(area.key);
    if (area.kind === 'yesNoItems') {
      const sum = (area.items ?? []).reduce((s, i) => s + i.points, 0);
      if (round2(sum) !== round2(area.points)) problems.push(`area '${area.key}': items sum to ${sum}, points is ${area.points}`);
      const keys = new Set();
      for (const i of area.items ?? []) {
        if (keys.has(i.key)) problems.push(`area '${area.key}': duplicate item key '${i.key}'`);
        keys.add(i.key);
      }
    } else if (area.kind === 'rating') {
      const max = 5 * (area.ratingMultiplier ?? 1);
      if (round2(max) !== round2(area.points)) problems.push(`area '${area.key}': 5 × ${area.ratingMultiplier ?? 1} = ${max}, points is ${area.points}`);
    }
  }
  for (const k of rubric.capAreaKeys ?? ['signability']) {
    const a = (rubric.areas ?? []).find((x) => x.key === k);
    if (!a) problems.push(`capAreaKeys names unknown area '${k}'`);
    else if (a.kind !== 'rating') problems.push(`capAreaKeys area '${k}' is not a rating area`);
  }
  return problems;
}
