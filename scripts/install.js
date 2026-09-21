#!/usr/bin/env node

/**
 * Fling CLI - Post-install Script
 * Downloads the pre-built native binary for the target platform or builds from Go source.
 */

const { ensureBinary, getPlatformInfo, getPackageVersion } = require('./utils');

async function install() {
  const { goos, goarch } = getPlatformInfo();
  const version = getPackageVersion();

  console.log(`\n======================================================`);
  console.log(`  Fling CLI v${version} Post-Install Setup`);
  console.log(`  Target Architecture: ${goos}/${goarch}`);
  console.log(`======================================================\n`);

  try {
    const binPath = await ensureBinary(true);
    console.log(`\n[fling-cli] Ready! Native binary configured at: ${binPath}\n`);
  } catch (err) {
    console.warn(`\n[fling-cli] Note: Post-install pre-fetching could not complete:`);
    console.warn(`  ${err.message}`);
    console.warn(`[fling-cli] Fling will attempt to resolve/compile the binary when first executed.\n`);
  }
}

install();
