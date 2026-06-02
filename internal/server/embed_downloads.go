package server

import "embed"

//go:embed downloads/*
var DownloadsFS embed.FS
