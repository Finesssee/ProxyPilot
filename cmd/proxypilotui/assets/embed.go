package assets

import "embed"

//go:embed index.html vite.svg assets/*
var FS embed.FS
