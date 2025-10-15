package main

import (
	"os"

	"github.com/YangchenYe323/boxtroll/internal/command"
)

func main() {
	if err := command.BoxtrollCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
