import { mkdir, rm, copyFile, stat, readdir } from 'node:fs/promises';
import { join, dirname } from 'node:path';

const root = process.cwd();
const dist = join(root, 'dist');
const files = ['index.html', 'styles.css', 'app.js'];

async function exists(path) {
	try {
		await stat(path);
		return true;
	} catch {
		return false;
	}
}

async function copyTree(source, target) {
	await mkdir(target, { recursive: true });
	const entries = await readdir(source, { withFileTypes: true });
	for (const entry of entries) {
		const from = join(source, entry.name);
		const to = join(target, entry.name);
		if (entry.isDirectory()) {
			await copyTree(from, to);
		} else if (entry.isFile()) {
			await copyFile(from, to);
		}
	}
}

await rm(dist, { recursive: true, force: true });
await mkdir(dist, { recursive: true });

for (const file of files) {
	await copyFile(join(root, file), join(dist, file));
}

const assetsDir = join(root, 'assets');
if (await exists(assetsDir)) {
	await copyTree(assetsDir, join(dist, 'assets'));
}
