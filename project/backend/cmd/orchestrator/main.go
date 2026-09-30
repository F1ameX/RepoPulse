package main

import (
	"os"

	"github.com/F1ameX/RepoPulse/project/backend/internal/orchestrator/app"
	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/command"
)

func main() {
	os.Exit(command.Execute(app.Name, app.LoadConfig, app.Run))
}
