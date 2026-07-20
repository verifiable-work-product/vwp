import { coverageConfigDefaults, defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    coverage: {
      provider: 'v8',
      // The benchmark tool libraries live outside the CLI package but are
      // tested here; allowExternal lets coverage track them.
      allowExternal: true,
      exclude: [
        ...coverageConfigDefaults.exclude,
        // Commander argv wiring, exercised via the dist smoke test (child
        // process — invisible to v8 coverage).
        'src/index.ts',
        // Thin argv/fs wrappers over the tested lib/ modules.
        '../benchmark/tools/*.mjs',
        'vitest.config.ts',
      ],
      thresholds: {
        lines: 80,
        statements: 80,
        functions: 80,
        branches: 80,
      },
    },
  },
});
