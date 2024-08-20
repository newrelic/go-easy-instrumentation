package main

import (
	"log"
	"os"

	"github.com/dave/dst/decorator"
	"golang.org/x/tools/go/packages"
)

const loadMode = packages.LoadSyntax

func createDiffFile(path string) {
	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
}

func main() {
	log.Default().SetFlags(0)
	cfg := NewCLIConfig()

	createDiffFile(cfg.DiffFile)

	pkgs, err := decorator.Load(&packages.Config{Dir: cfg.PackagePath, Mode: loadMode}, cfg.PackageName)
	if err != nil {
		log.Fatal(err)
	}

	manager := NewInstrumentationManager(pkgs, cfg.AppName, cfg.AgentVariableName, cfg.DiffFile, cfg.PackagePath)
	err = manager.InstrumentPackages(InstrumentMain, InstrumentHandleFunction, InstrumentHttpClient, CannotInstrumentHttpMethod)
	if err != nil {
		log.Fatal(err)
	}

	manager.AddRequiredModules()
	manager.WriteDiff()
}
