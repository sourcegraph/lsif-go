package main

import (
	"log"
	"os"
)

func init() {
	log.SetFlags(0)
	log.SetPrefix("")
	log.SetOutput(os.Stdout)
}
