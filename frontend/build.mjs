import { copyFile, mkdir, readdir, stat } from 'node:fs/promises';
import { dirname, join } from 'node:path';

const root = process.cwd();
const dist = join(root, 'dist');
const ignore = new Set(['dist', 'node_modules']);

async function exists(file) {
  try {
    await stat(file);
    return true;
  } catch {
    return false;
  }
}

async function copyTree(source, target) {
  await mkdir(target, { recursive: true });
  const entries = await readdir(source, { withFileTypes: true });
  for (const entry of entries) {
    if (ignore.has(entry.name)) continue;
    const from = join(source, entry.name);
    const to = join(target, entry.name);
    if (entry.isDirectory()) {
      await copyTree(from, to);
    } else if (entry.isFile()) {
      await copyFile(from, to);
    }
  }
}

await mkdir(dist, { recursive: true });

const items = await readdir(root, { withFileTypes: true });
for (const item of items) {
  if (ignore.has(item.name)) continue;
  const from = join(root, item.name);
  const to = join(dist, item.name);
  if (item.isDirectory()) {
    await copyTree(from, to);
  } else if (item.isFile()) {
    await copyFile(from, to);
  }
}

if (!(await exists(join(dist, 'index.html')))) {
  throw new Error('dist/index.html missing after build');
}
