import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";
import nextTs from "eslint-config-next/typescript";

const eslintConfig = defineConfig([
  ...nextVitals,
  ...nextTs,
  {
    rules: {
      // This dashboard is a plain client-rendered SPA: every section fetches
      // its own data on mount (and on an interval, for Overview) with a
      // standard `useEffect(() => { load(); }, [load])` — the textbook
      // data-fetching-on-mount pattern, not an accidental derived-state bug.
      // The new set-state-in-effect rule flags that pattern unconditionally;
      // rewriting it around use()/Suspense would be a bigger architectural
      // change than this dashboard's scope calls for.
      "react-hooks/set-state-in-effect": "off",
    },
  },
  // Override default ignores of eslint-config-next.
  globalIgnores([
    // Default ignores of eslint-config-next:
    ".next/**",
    "out/**",
    "build/**",
    "next-env.d.ts",
  ]),
]);

export default eslintConfig;
