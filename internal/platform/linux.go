package platform

import (
	"fmt"
	"os"
)

func getLinuxExcludes() []string {
	username := os.Getenv("USER")
	if username == "" {
		username = os.Getenv("LOGNAME")
	}

	return []string{
		"./**/*.sock",
		"./.gnupg/S.*",
		fmt.Sprintf("./%s/Sources/github.com/NixOS/nixpkgs", username),
		fmt.Sprintf("./%s/local/share/nvim", username),
	}
}
