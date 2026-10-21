/**
 * Library entry point for @vwp/cli.
 *
 * The deterministic verifier is importable so that other tools (hosted
 * verify pages, package builders, CI checks) can run exactly the same
 * verification as the CLI without shelling out to it.
 */
export {
  openPackage,
  openPackageBuffer,
  readJson,
  isUnsafePath,
  type PackageSource,
} from './package';
export { verifyPackage, type Outcome, type OutcomeStatus, type VerifyReport } from './validations';
export { diffPackages, type DiffLine } from './diff';
export { sha256Hex } from './hashes';
export { schemaErrors, extensionSchemaErrors, findSchemasDir, SCHEMA_NAMES, type SchemaName } from './schema';
export { canonicalJson, stripReviewFields, type ObjectKind } from './canonical';
export {
  buildGraph,
  computeDigests,
  selfHash,
  closureHash,
  collectionRoot,
  analysisRoot,
  contentRoot,
  reviewRoot,
  dependencyIds,
  evidenceBytesSha256,
  evidenceClosureHash,
  DIGEST_ALGORITHM,
  COLLECTION_KINDS,
  type CollectionKind,
  type Digests,
  type GraphObject,
  type GraphProblem,
  type IntegrityGraph,
} from './integrity';
