import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import fs from "fs";
import path from "path";
import { defineConfig } from "vite";

const embedGoContent = `package assets

import "embed"

//go:embed index.html vite.svg logo.png assets/*
var FS embed.FS
`;

// https://vite.dev/config/
export default defineConfig({
	plugins: [
		react(),
		tailwindcss(),
		{
			name: "generate-embed-go",
			closeBundle() {
				const outDir = path.resolve(__dirname, "../cmd/proxypilotui/assets");
				fs.writeFileSync(path.join(outDir, "embed.go"), embedGoContent);
				console.log("Generated embed.go");
			},
		},
	],
	resolve: {
		alias: {
			"@": path.resolve(__dirname, "./src"),
		},
	},
	server: {
		proxy: {
			"/v0": {
				target: "http://localhost:8318",
				changeOrigin: true,
			},
			"/v1": {
				target: "http://localhost:8318",
				changeOrigin: true,
			},
		},
	},
	build: {
		outDir: "../cmd/proxypilotui/assets",
		emptyOutDir: true,
		rollupOptions: {
			output: {
				entryFileNames: "assets/[name].js",
				chunkFileNames: "assets/[name].js",
				assetFileNames: "assets/[name].[ext]",
			},
		},
	},
});
