#!/usr/bin/env bun

import { $ } from "bun";
import { existsSync, chmodSync } from "fs";
import { join } from "path";

const VERSION = "0.1.0";
const REPO = "router-for-me/CLIProxyAPI";

const PLATFORM_MAP: Record<string, string> = {
  darwin: "darwin",
  linux: "linux",
  win32: "windows",
};

const ARCH_MAP: Record<string, string> = {
  x64: "amd64",
  arm64: "arm64",
};

async function install() {
  const platform = PLATFORM_MAP[process.platform];
  const arch = ARCH_MAP[process.arch];

  if (!platform || !arch) {
    console.error(`Unsupported platform: ${process.platform}-${process.arch}`);
    process.exit(1);
  }

  const ext = platform === "windows" ? ".exe" : "";
  const binaryName = `ppswitch-${platform}-${arch}${ext}`;
  const url = `https://github.com/${REPO}/releases/download/ppswitch-v${VERSION}/${binaryName}`;

  const binDir = join(import.meta.dir, "bin");
  const binPath = join(binDir, platform === "windows" ? "ppswitch.exe" : "ppswitch");

  console.log(`Downloading ppswitch v${VERSION} for ${platform}-${arch}...`);

  try {
    const response = await fetch(url, { redirect: "follow" });

    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }

    await Bun.write(binPath, response);

    // Make executable on Unix
    if (platform !== "windows") {
      chmodSync(binPath, 0o755);
    }

    console.log("ppswitch installed successfully!");
    console.log('Run "ppswitch --help" to get started.');
  } catch (err: any) {
    console.error("Failed to download ppswitch:", err.message);
    console.error(`\nYou can manually download from:\nhttps://github.com/${REPO}/releases`);
    process.exit(1);
  }
}

install();
