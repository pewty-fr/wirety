import js from '@eslint/js'
import globals from 'globals'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import tseslint from 'typescript-eslint'
import { defineConfig, globalIgnores } from 'eslint/config'

export default defineConfig([
  globalIgnores(['dist']),
  {
    files: ['**/*.{ts,tsx}'],
    extends: [
      js.configs.recommended,
      tseslint.configs.recommended,
      reactHooks.configs.flat.recommended,
      reactRefresh.configs.vite,
    ],
    languageOptions: {
      ecmaVersion: 2020,
      globals: globals.browser,
    },
    rules: {
      // React Compiler readiness rules introduced in eslint-plugin-react-hooks@7.1.
      // They flag legitimate patterns (loading a prop into form state via useEffect,
      // syncing selectedNetworkId, async helpers referenced before declaration) that
      // the React Compiler can't auto-optimize. Properly fixing each violation means
      // moving to `key`-based state resets, render-time `useMemo` derivation, etc. —
      // a deliberate migration, not a lint cleanup. Disabled until that migration
      // happens; re-enable to re-surface them.
      'react-hooks/set-state-in-effect': 'off',
      'react-hooks/immutability': 'off',
    },
  },
  {
    files: ['src/contexts/*.tsx', 'src/providers/*.tsx'],
    rules: {
      'react-refresh/only-export-components': 'off',
    },
  },
])
