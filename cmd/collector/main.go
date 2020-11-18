package main

import (
	"fmt"
	"log"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/service"
	"go.opentelemetry.io/collector/service/defaultcomponents"
)

func main() {
	factories, err := components()
	if err != nil {
		log.Fatalf("failed to build default components: %v", err)
	}

	info := component.ApplicationStartInfo{
		ExeName:  "collector",
		LongName: "Hypertrace Collector",
		Version:  Version,
		GitHash:  GitHash,
	}

	if err := run(service.Parameters{ApplicationStartInfo: info, Factories: factories}); err != nil {
		log.Fatal(err)
	}
}

func components() (component.Factories, error) {
	var errs []error
	factories, err := defaultcomponents.Components()
	if err != nil {
		return component.Factories{}, err
	}

	return factories, componenterror.CombineErrors(errs)
}

func run(params service.Parameters) error {
	app, err := service.New(params)
	if err != nil {
		return fmt.Errorf("failed to construct the application: %w", err)
	}

	err = app.Run()
	if err != nil {
		return fmt.Errorf("application run finished with error: %w", err)
	}

	return nil
}
