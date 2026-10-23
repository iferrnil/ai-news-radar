package main

import (
	"runtime"

	"github.com/egoist/mygo"
)

// windowOptions for the native Content window: vibrancy sidebar material, hidden title bar,
// remembered size. No Page / preload — MyGo draws the UI itself.
func windowOptions(content mygo.Content) mygo.WindowOptions {
	o := mygo.WindowOptions{
		Title:           appName,
		Width:           1280,
		Height:          840,
		MinWidth:        880,
		MinHeight:       560,
		StateKey:        "native-main",
		BackgroundColor: "#0b0b10",
		Vibrancy:        mygo.VibrancySidebar,
		Content:         content,
	}
	if runtime.GOOS == "darwin" {
		o.TitleBarStyle = mygo.TitleBarHiddenInset
		o.TrafficLightPosition = &mygo.Point{X: 16, Y: 14}
	}
	return o
}
