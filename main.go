package main

import (
	"log"

	"github.com/dave/dst/decorator"
	"golang.org/x/tools/go/packages"

	"github.com/newrelic/go-easy-instrumentation/cli"
	"github.com/newrelic/go-easy-instrumentation/parser"
)

func main() {
	log.Default().SetFlags(0)
	cfg := cli.NewCLIConfig()

	pkgs, err := decorator.Load(&packages.Config{Dir: cfg.PackagePath, Mode: packages.LoadSyntax}, cfg.PackageName)
	if err != nil {
		log.Fatal(err)
	}

	manager := parser.NewInstrumentationManager(pkgs, cfg.AppName, cfg.AgentVariableName, cfg.DiffFile, cfg.PackagePath)
	err = manager.CreateDiffFile()
	if err != nil {
		log.Fatal(err)
	}

	err = manager.DetectDependencyIntegrations()
	if err != nil {
		log.Fatal(err)
	}

	err = manager.InstrumentApplication()
	if err != nil {
		log.Fatal(err)
	}

	err = manager.AddRequiredModules()
	if err != nil {
		log.Fatal(err)
	}

	err = manager.WriteDiff()
	if err != nil {
		log.Fatal(err)
	}
}
