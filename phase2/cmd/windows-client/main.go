// Phase 2 Windows Client
//
// Windows client with Cloud Files API integration:
// - CfAPI via C++ shim (CGO)
// - True placeholder behavior
// - Explorer integration
//
// Build requirements:
// - Windows 10 1809+
// - CGO enabled
// - Windows SDK
package main

import (
	"fmt"
	"log"
	"runtime"
)

func main() {
	fmt.Println("FruitSalade Phase 2 Windows Client")
	fmt.Printf("  Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Println()

	if runtime.GOOS != "windows" {
		log.Fatal("This binary is for Windows only")
	}

	// TODO: Initialize CfAPI shim
	// TODO: Register sync root
	// TODO: Create placeholders from metadata
	// TODO: Handle hydration callbacks

	log.Fatal("Windows client not yet implemented")
}
