#!/usr/bin/env node

import { existsSync, readdirSync, readFileSync } from "node:fs";
import { dirname, extname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const repositoryRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const docsPath = resolve(
  process.argv[2] ?? join(repositoryRoot, "..", "askXuan-docs", "API-REFERENCE.md"),
);

function walk(directory, fileName) {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const path = join(directory, entry.name);
    if (entry.isDirectory()) return walk(path, fileName);
    return entry.name === fileName ? [path] : [];
  });
}

function contract(method, path) {
  return `${method.toUpperCase()} ${path}`;
}

if (!existsSync(docsPath) || extname(docsPath) !== ".md") {
  console.error(`API Reference 不存在: ${docsPath}`);
  process.exit(2);
}

const runtimeContracts = new Map();
const routePattern =
  /Method:\s*http\.Method(Get|Post|Put|Delete|Patch)[\s\S]*?Path:\s*"([^"]+)"/g;

for (const file of walk(join(repositoryRoot, "services"), "routes.go")) {
  const source = readFileSync(file, "utf8");
  const blocks = source.includes("rest.WithPrefix(")
    ? [...source.matchAll(/server\.AddRoutes\(([\s\S]*?)\n\t\)/g)].map(
        (match) => match[1],
      )
    : [source];

  for (const block of blocks) {
    const prefix = block.match(/rest\.WithPrefix\("([^"]+)"\)/)?.[1] ?? "";
    for (const match of block.matchAll(routePattern)) {
      const path =
        prefix && match[2] === "/"
          ? prefix
          : `${prefix}${match[2]}`;
      runtimeContracts.set(
        contract(match[1], path),
        relative(repositoryRoot, file),
      );
    }
  }
}

const documentedContracts = new Set();
const docsSource = readFileSync(docsPath, "utf8");
const docsPattern = /^\|\s*(GET|POST|PUT|DELETE|PATCH)\s*\|\s*`([^`]+)`\s*\|/gim;
for (const match of docsSource.matchAll(docsPattern)) {
  documentedContracts.add(contract(match[1], match[2]));
}

const missing = [...runtimeContracts.keys()]
  .filter((item) => !documentedContracts.has(item))
  .sort();
const stale = [...documentedContracts]
  .filter((item) => !runtimeContracts.has(item))
  .sort();

console.log(`运行时 HTTP 契约: ${runtimeContracts.size}`);
console.log(`API Reference 唯一契约: ${documentedContracts.size}`);
console.log(`文档缺失: ${missing.length}`);
for (const item of missing) {
  console.log(`- ${item} (${runtimeContracts.get(item)})`);
}
console.log(`文档过期: ${stale.length}`);
for (const item of stale) {
  console.log(`- ${item}`);
}

process.exitCode = missing.length === 0 && stale.length === 0 ? 0 : 1;
