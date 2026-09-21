/**
 * Fling CLI - Node.js Package Utilities
 * Handles platform detection, binary resolution, downloads, and fallback compilation.
 */

const fs = require('fs');
const path = require('path');
const os = require('os');
const https = require('https');
const http = require('http');
const { execSync, spawnSync } = require('child_process');

const REPO_OWNER = 'SatyamKumarCS';
const REPO_NAME = 'Fling-CLI';

/**
 * Maps Node platform and architecture to Go equivalents.
 */
function getPlatformInfo() {
  const platform = process.platform;
  const arch = process.arch;

  let goos = '';
  switch (platform) {
    case 'darwin':
      goos = 'darwin';
      break;
    case 'linux':
      goos = 'linux';
      break;
    case 'win32':
      goos = 'windows';
      break;
    case 'freebsd':
      goos = 'freebsd';
      break;
    default:
      goos = platform;
  }

  let goarch = '';
  switch (arch) {
    case 'x64':
      goarch = 'amd64';
      break;
    case 'arm64':
      goarch = 'arm64';
      break;
    case 'arm':
      goarch = 'arm';
      break;
    case 'ia32':
      goarch = '386';
      break;
    default:
      goarch = arch;
  }

  const isWindows = goos === 'windows';
  const ext = isWindows ? '.exe' : '';
  const binaryName = `fling-${goos}-${goarch}${ext}`;
  const localBinaryName = isWindows ? 'fling-native.exe' : 'fling-native';

  return {
    goos,
    goarch,
    isWindows,
    ext,
    binaryName,
    localBinaryName,
    platform,
    arch
  };
}

/**
 * Returns package version from package.json.
 */
function getPackageVersion() {
  try {
    const pkgJsonPath = path.join(__dirname, '..', 'package.json');
    const pkg = JSON.parse(fs.readFileSync(pkgJsonPath, 'utf8'));
    return pkg.version || '1.0.0';
  } catch (err) {
    return '1.0.0';
  }
}

/**
 * Returns user cache directory for Fling binaries (~/.fling/bin).
 */
function getUserCacheDir() {
  const home = os.homedir() || os.tmpdir();
  const cacheDir = path.join(home, '.fling', 'bin');
  if (!fs.existsSync(cacheDir)) {
    fs.mkdirSync(cacheDir, { recursive: true });
  }
  return cacheDir;
}

/**
 * Download a file with automatic redirect handling (HTTPS/HTTP).
 */
function downloadFile(url, destPath) {
  return new Promise((resolve, reject) => {
    const file = fs.createWriteStream(destPath);

    function requestUrl(currentUrl, maxRedirects = 5) {
      if (maxRedirects <= 0) {
        file.close();
        fs.unlink(destPath, () => {});
        return reject(new Error('Too many redirects while downloading binary.'));
      }

      const client = currentUrl.startsWith('https') ? https : http;
      const headers = {
        'User-Agent': `fling-cli-npm-installer/${getPackageVersion()}`
      };

      const req = client.get(currentUrl, { headers }, (res) => {
        // Handle HTTP redirects (301, 302, 303, 307, 308)
        if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
          let nextUrl = res.headers.location;
          if (!nextUrl.startsWith('http')) {
            const urlObj = new URL(currentUrl);
            nextUrl = new URL(nextUrl, urlObj.origin).href;
          }
          return requestUrl(nextUrl, maxRedirects - 1);
        }

        if (res.statusCode !== 200) {
          file.close();
          fs.unlink(destPath, () => {});
          return reject(new Error(`Failed to download binary: HTTP ${res.statusCode} (${res.statusMessage})`));
        }

        res.pipe(file);

        file.on('finish', () => {
          file.close(() => {
            // Verify file size is non-empty
            try {
              const stats = fs.statSync(destPath);
              if (stats.size < 1024) {
                fs.unlinkSync(destPath);
                return reject(new Error('Downloaded binary is too small or corrupted.'));
              }
              resolve(destPath);
            } catch (err) {
              reject(err);
            }
          });
        });
      });

      req.on('error', (err) => {
        file.close();
        fs.unlink(destPath, () => {});
        reject(err);
      });

      req.setTimeout(30000, () => {
        req.destroy();
        file.close();
        fs.unlink(destPath, () => {});
        reject(new Error('Download request timed out after 30s.'));
      });
    }

    requestUrl(url);
  });
}

/**
 * Checks if Go compiler is available in the current environment.
 */
function hasGoCompiler() {
  try {
    const result = spawnSync('go', ['version'], { stdio: 'ignore' });
    return result.status === 0;
  } catch (e) {
    return false;
  }
}

/**
 * Compiles Fling from local Go source files if available.
 */
function compileLocalBinary(destPath) {
  const rootDir = path.join(__dirname, '..');
  const goModPath = path.join(rootDir, 'go.mod');
  const mainGoPath = path.join(rootDir, 'cmd', 'fling', 'main.go');

  if (!fs.existsSync(goModPath) || !fs.existsSync(mainGoPath)) {
    throw new Error('Local Go source files not found for compilation.');
  }

  if (!hasGoCompiler()) {
    throw new Error('Go compiler is not installed.');
  }

  const destDir = path.dirname(destPath);
  if (!fs.existsSync(destDir)) {
    fs.mkdirSync(destDir, { recursive: true });
  }

  execSync(`go build -ldflags="-s -w" -o "${destPath}" ./cmd/fling`, {
    cwd: rootDir,
    stdio: 'inherit'
  });

  if (!fs.existsSync(destPath)) {
    throw new Error('Compilation finished but binary was not created.');
  }

  if (process.platform !== 'win32') {
    fs.chmodSync(destPath, 0o755);
  }

  return destPath;
}

/**
 * Finds or downloads/builds the executable native binary.
 */
async function ensureBinary(verbose = false) {
  const { isWindows, binaryName, localBinaryName, goos, goarch } = getPlatformInfo();
  const version = getPackageVersion();

  // Locations to check in order of priority:
  const localPkgBin = path.join(__dirname, '..', 'bin', localBinaryName);
  const repoRootBin = path.join(__dirname, '..', isWindows ? 'fling.exe' : 'fling');
  const cachedBin = path.join(getUserCacheDir(), `fling-v${version}-${goos}-${goarch}${isWindows ? '.exe' : ''}`);

  // 1. Check local package bin directory
  if (fs.existsSync(localPkgBin)) {
    try {
      if (!isWindows) fs.chmodSync(localPkgBin, 0o755);
      return localPkgBin;
    } catch (_) {}
  }

  // 2. Check repo root binary (useful for dev checkouts)
  if (fs.existsSync(repoRootBin)) {
    try {
      if (!isWindows) fs.chmodSync(repoRootBin, 0o755);
      return repoRootBin;
    } catch (_) {}
  }

  // 3. Check user cache directory (~/.fling/bin/...)
  if (fs.existsSync(cachedBin)) {
    try {
      if (!isWindows) fs.chmodSync(cachedBin, 0o755);
      return cachedBin;
    } catch (_) {}
  }

  // 4. Target location for download or build: prefer localPkgBin if writable, else cachedBin
  let targetPath = localPkgBin;
  try {
    const binDir = path.dirname(localPkgBin);
    if (!fs.existsSync(binDir)) fs.mkdirSync(binDir, { recursive: true });
    fs.accessSync(binDir, fs.constants.W_OK);
  } catch (_) {
    targetPath = cachedBin;
  }

  const releaseUrl = `https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/download/v${version}/${binaryName}`;

  if (verbose) {
    console.log(`[fling-cli] Native binary not found. Preparing for ${goos}/${goarch}...`);
  }

  // Try downloading from GitHub Releases
  try {
    if (verbose) console.log(`[fling-cli] Downloading pre-built binary from:\n  ${releaseUrl}`);
    await downloadFile(releaseUrl, targetPath);
    if (!isWindows) fs.chmodSync(targetPath, 0o755);
    if (verbose) console.log(`[fling-cli] Successfully installed pre-built binary.`);
    return targetPath;
  } catch (downloadErr) {
    if (verbose) {
      console.warn(`[fling-cli] Could not download pre-built release: ${downloadErr.message}`);
    }

    // Try building with Go if source is present
    if (hasGoCompiler()) {
      try {
        if (verbose) console.log(`[fling-cli] Compiling Fling from Go source...`);
        compileLocalBinary(targetPath);
        if (verbose) console.log(`[fling-cli] Build succeeded!`);
        return targetPath;
      } catch (compileErr) {
        if (verbose) {
          console.warn(`[fling-cli] Compilation failed: ${compileErr.message}`);
        }
      }
    }

    // If both failed, throw error
    throw new Error(
      `Failed to obtain Fling binary for ${goos}/${goarch}.\n` +
      `  - Release download failed: ${downloadErr.message}\n` +
      `  - Please ensure internet connectivity or install Go (https://go.dev) to compile from source.`
    );
  }
}

module.exports = {
  getPlatformInfo,
  getPackageVersion,
  getUserCacheDir,
  downloadFile,
  hasGoCompiler,
  compileLocalBinary,
  ensureBinary
};
