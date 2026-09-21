#!/usr/bin/env node

/**
 * Fling CLI - Cross-Platform Release Builder
 * Compiles binaries for all major platforms (macOS, Linux, Windows across ARM64, AMD64, 386).
 */

const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

const targets = [
  { os: 'darwin', arch: 'arm64', ext: '' },
  { os: 'darwin', arch: 'amd64', ext: '' },
  { os: 'linux', arch: 'amd64', ext: '' },
  { os: 'linux', arch: 'arm64', ext: '' },
  { os: 'linux', arch: '386', ext: '' },
  { os: 'windows', arch: 'amd64', ext: '.exe' },
  { os: 'windows', arch: 'arm64', ext: '.exe' },
  { os: 'windows', arch: '386', ext: '.exe' }
];

const rootDir = path.join(__dirname, '..');
const distDir = path.join(rootDir, 'dist');

if (!fs.existsSync(distDir)) {
  fs.mkdirSync(distDir, { recursive: true });
}

console.log(`==> Compiling Fling CLI for ${targets.length} cross-platform targets...`);

for (const target of targets) {
  const outputName = `fling-${target.os}-${target.arch}${target.ext}`;
  const outputPath = path.join(distDir, outputName);

  console.log(`  → Building ${target.os}/${target.arch} -> ${outputName}`);

  try {
    execSync(`go build -ldflags="-s -w" -o "${outputPath}" ./cmd/fling`, {
      cwd: rootDir,
      env: {
        ...process.env,
        GOOS: target.os,
        GOARCH: target.arch,
        CGO_ENABLED: '0'
      },
      stdio: 'pipe'
    });

    const stats = fs.statSync(outputPath);
    const sizeMb = (stats.size / (1024 * 1024)).toFixed(2);
    console.log(`    ✓ Done (${sizeMb} MB)`);
  } catch (err) {
    console.error(`    ✗ Failed to build for ${target.os}/${target.arch}:`, err.message);
    process.exit(1);
  }
}

console.log(`\n==> All ${targets.length} binaries built successfully in ./dist/`);
