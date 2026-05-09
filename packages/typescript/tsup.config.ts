import { defineConfig } from "tsup";

export default defineConfig([
  {
    entry: ["src/index.ts"],
    format: ["esm", "cjs"],
    dts: true,
    sourcemap: true,
    clean: true,
    target: "node20",
  },
  {
    entry: ["src/conformance/bin.ts"],
    format: ["cjs"],
    sourcemap: true,
    clean: false,
    target: "node20",
    outDir: "dist/conformance",
    banner: { js: "#!/usr/bin/env node" },
  },
]);
