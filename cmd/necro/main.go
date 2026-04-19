package main

import (
	"log"

	"github.com/repo-necromancer/necro/internal/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		log.Fatal(err)
	}
}
