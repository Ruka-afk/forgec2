import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";
import nextTs from "eslint-config-next/typescript";

const eslintConfig = defineConfig([
  ...nextVitals,
  ...nextTs,
  // Override default ignores of eslint-config-next.
  globalIgnores([
    // Default ignores of eslint-config-next:
    ".next/**",
    "out/**",
    "build/**",
    "next-env.d.ts",
    // Vendor/minified bundles — do not lint
    "public/js/**",
    "node_modules/**",
  ]),
  {
    rules: {
      // React 19 strictness: setState in useEffect is standard for data fetching
      "react-hooks/set-state-in-effect": "off",
      // React 19 strictness: accessing hook-returned refs during render is intentional in our hooks
      "react-hooks/refs": "off",
      // Dynamic agent screenshots/avatars can't use next/image (unknown dimensions, dynamic URLs)
      "@next/next/no-img-element": "off",
      // Project uses Google Fonts CDN by design (AGENTS.md), not _document.js
      "@next/next/no-page-custom-font": "off",
      // Enforce type safety: ban `as any` casts
      "@typescript-eslint/no-explicit-any": "error",
    },
  },
]);

export default eslintConfig;
