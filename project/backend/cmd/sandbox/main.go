package main

import (
	"os"

	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/command"
	"github.com/F1ameX/RepoPulse/project/backend/internal/sandbox/app"
)

func main() {
	os.Exit(command.Execute(app.Name, app.Run))
}
