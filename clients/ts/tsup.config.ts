import { defineConfig } from "tsup";

export default defineConfig({
  entry: ["src/index.ts"],
  format: ["esm"],
  dts: true,
  outDir: "dist",
  external: ["@bufbuild/protobuf/codegenv2"], // ← treat as external
  bundle: true, // includes all imports, even rpc_pb.js
});
