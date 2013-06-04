package main

import (
	"fmt"
	"os"
)

func main() {
	var trntSessionInfo TrntSessionInfo

	if len(os.Args) < 2 {
		fmt.Println(DebugGetFuncName(), "Usage:gotrnt file.torrent")
		return
	}

	// Start listener
	StartGoTrntListener()

	// Read torrent file and init session
	if trntSessionInfo.Init(os.Args[1]) {

		// Connect to peers
		trntSessionInfo.Start()

		// Wait for listener
		WaitForGoTrntListener()
	}

	// Close all peer connections
	trntSessionInfo.Stop()
}
