const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

test('package.json defines the Electron entrypoints and scripts', () => {
  const packagePath = path.join(__dirname, '..', 'package.json');
  const pkg = JSON.parse(fs.readFileSync(packagePath, 'utf8'));

  assert.equal(pkg.main, 'main.js');
  assert.equal(pkg.scripts.start, 'electron .');
  assert.match(pkg.scripts.test, /node --test/);

  const electronVersion = (pkg.dependencies && pkg.dependencies.electron)
    || (pkg.devDependencies && pkg.devDependencies.electron);

  assert.equal(typeof electronVersion, 'string');
  assert.notEqual(electronVersion.trim(), '');
});
