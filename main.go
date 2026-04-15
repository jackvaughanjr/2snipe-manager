package main

import "github.com/jackvaughanjr/2snipe-manager/cmd"

var version = "dev" // overridden by -ldflags "-X main.version=vX.Y.Z"

func main() {
	cmd.SetVersion(version)
	cmd.Execute()
}
