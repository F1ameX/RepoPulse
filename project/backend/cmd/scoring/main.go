package main

import (
	"os"

	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/command"
	"github.com/F1ameX/RepoPulse/project/backend/internal/scoring/app"
)

func main() {
	os.Exit(command.Execute(app.Name, app.LoadConfig, app.Run))
}
