import { openPackage } from '../lib/package';
import { buildGraph, compareIds, computeDigests, GraphObject } from '../lib/integrity';

/**
 * Print the integrity graph roots of a package as JSON, in the shape of the
 * manifest's digests block, so they can be cross-checked against another
 * implementation. --objects adds every object's self and closure hash
 * (narrative sections under "narrative"). When the graph is incomplete
 * (unresolved refs, ambiguous ids, cycles) the roots are undefined: the
 * problems are printed instead and the exit code is 1.
 */
export function digestsCommand(input: string, options: { objects?: boolean }): number {
  const pkg = openPackage(input);
  const graph = buildGraph(pkg);
  const out: Record<string, unknown> = {};
  let code = 0;
  try {
    Object.assign(out, computeDigests(graph));
  } catch (err) {
    out.error = (err as Error).message;
    code = 1;
  }
  if (options.objects) {
    const describe = (node: GraphObject) => ({
      kind: node.kind,
      selfHash: node.selfHash,
      ...(node.tainted ? { tainted: true } : { closureHash: node.closureHash }),
    });
    const objects: Record<string, ReturnType<typeof describe>> = {};
    for (const id of [...graph.objects.keys()].sort(compareIds)) objects[id] = describe(graph.objects.get(id)!);
    out.objects = objects;
    if (graph.byKind.narrative.length > 0) {
      out.narrative = Object.fromEntries(graph.byKind.narrative.map((s) => [s.id, describe(s)]));
    }
  }
  if (graph.problems.length > 0) {
    out.problems = graph.problems.map((p) => ({ kind: p.kind, from: p.from, id: p.id, message: p.message }));
    code = 1;
  }
  console.log(JSON.stringify(out, null, 2));
  return code;
}
