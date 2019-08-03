package log

import (
	"log"
	"strings"
)

func init() {
	log.SetFlags(0)
	log.SetPrefix("")
}

type Level int

const (
	Debug Level = iota
	Info
	None Level = 99
)

var level = None

func SetLevel(l Level) {
	level = l
}

var (
	debugAll  = false
	debugMods map[string]bool
)

func SetDebugMods(mods string) {
	if mods == "none" {
		return
	}
	level = Debug

	if mods == "all" {
		debugAll = true
		return
	}

	debugMods = make(map[string]bool)
	for _, m := range strings.Split(mods, ",") {
		debugMods[m] = true
	}
}

func Debugf(mod, format string, v ...interface{}) {
	if level > Debug {
		return
	} else if !debugAll && !debugMods[mod] {
		return
	}

	log.Printf(format, v...)
}

func Debugln(mod string, v ...interface{}) {
	if level > Debug {
		return
	} else if !debugAll && !debugMods[mod] {
		return
	}

	log.Println(v...)
}

func Infof(format string, v ...interface{}) {
	if level > Info {
		return
	}

	log.Printf(format, v...)
}

func Infoln(v ...interface{}) {
	if level > Info {
		return
	}

	log.Println(v...)
}
