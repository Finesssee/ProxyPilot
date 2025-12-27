package assets

import "embed"

//go:embed index.html vite.svg logo.png assets/*
var FS embed.FS
