import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const bin = path.join(root, "dist", "index.js");

if (process.platform !== "win32" && fs.existsSync(bin)) {
  fs.chmodSync(bin, 0o755);
}
