package main

import (
	"os"
	"path/filepath"
	"runtime"
)

// bundleLayout resolves the runtime inside the app's resources directory:
//
//	<res>/node/bin/node            (per-arch, merged by mygo for darwin/universal)
//	<res>/server/server.js …       (Next standalone + static + public + sources)
//
// AI_RADAR_NODE / AI_RADAR_SERVER override them (development, tests).
func bundleLayout(res string) (node, app string) {
	exe := "node"
	if runtime.GOOS == "windows" {
		exe = "node.exe"
	}
	node = filepath.Join(res, "node", "bin", exe)
	app = filepath.Join(res, "server")
	if v := os.Getenv("AI_RADAR_NODE"); v != "" {
		node = v
	}
	if v := os.Getenv("AI_RADAR_SERVER"); v != "" {
		app = v
	}
	return
}
