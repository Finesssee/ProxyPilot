#!/usr/bin/env bun

import { chmodSync } from "fs";
import { join } from "path";
import pkg from "./package.json";

const VERSION = pkg.version;
const REPO = "Finesssee/ProxyPilot";

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
  const binaryName = `proxypilot-${platform}-${arch}${ext}`;
  const url = `https://github.com/${REPO}/releases/download/v${VERSION}/${binaryName}`;

  const binDir = join(import.meta.dir, "bin");
  const binPath = join(binDir, platform === "windows" ? "proxypilot.exe" : "proxypilot");

  console.log(`Downloading proxypilot v${VERSION} for ${platform}-${arch}...`);

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

    console.log("proxypilot installed successfully!");
    console.log('Run "proxypilot --help" to get started.');
  } catch (err: any) {
    console.error("Failed to download proxypilot:", err.message);
    console.error(`\nYou can manually download from:\nhttps://github.com/${REPO}/releases`);
    process.exit(1);
  }
}

install();
