package platform

func getLinuxExcludes() []string {
	return []string{
		"./**/*.sock",
		"./**/.build",
		"./**/.venv",
		"./**/__worktrees",
		"./**/node_modules",
		"./**/target",
		"./.Trash",
		"./.cache",
		"./.cargo",
		"./.local/share/Trash",
		"./.npm",
		"./.rustup",
		"./.vscode/extensions",
		"./Downloads",
		"./snap",
		"./go",
	}
}
