import { openPackage } from '../lib/package';
import { collectRetrievalOrigins } from '../lib/validations';

/**
 * List the distinct network origins the package's external references want to
 * reach, and the auth each needs — WITHOUT touching the network or reading any
 * secret. This is the "gather all the base URLs, then request credentials for
 * them" step a reviewer runs before `verify --online`.
 */
export function originsCommand(input: string): number {
  const pkg = openPackage(input);
  const origins = collectRetrievalOrigins(pkg);

  if (origins.length === 0) {
    console.log('No retrievable external references declared.');
    return 0;
  }

  console.log(`Retrieval origins (${origins.length}):`);
  console.log('');
  for (const o of origins) {
    console.log(`  ${o.origin}`);
    console.log(`    auth required: ${o.requiresAuth ? 'yes' : 'no'}`);
    console.log(`    schemes:       ${o.schemes.join(', ')}`);
    if (o.tokenHints.length) console.log(`    token source:  ${o.tokenHints.join(', ')}`);
    if (o.scopes.length) console.log(`    scopes:        ${o.scopes.join(', ')}`);
    if (o.discovery) console.log(`    discovery:     ${o.discovery}`);
    console.log('');
  }
  console.log('Supply one credential per origin over the top (env/prompt/vault/OAuth),');
  console.log('then run `vwp verify <package> --online`. Secrets are never stored in the package.');
  return 0;
}
