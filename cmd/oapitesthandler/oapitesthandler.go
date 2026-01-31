package main

import (
	"github.com/alecthomas/kong"
	"github.com/willabides/oapitesthandler/internal/handlergen"
)

var version = "unknown"

const description = `` // add description here

type cmdRoot struct {
	Config  string           `kong:"short='c',required,help='Path to oapi-codegen config YAML file'"`
	Out     string           `kong:"short='o',required,help='Directory to write the generated test handler to'"`
	Spec    string           `kong:"arg,help='Path to OpenAPI spec YAML file'"`
	Models  string           `kong:"help='Path to a package containing the OpenAPI models. If not specified, models will be generated into the output directory.'"`
	Version kong.VersionFlag `kong:"help=${VersionHelp}"`
}

var kongVars = kong.Vars{
	"version":     version,
	"VersionHelp": `Output the oapitesthandler version and exit.`,
}

func main() {
	var cli cmdRoot
	k := kong.Parse(&cli,
		kongVars,
		kong.Description(description),
	)

	err := handlergen.Run(cli.Spec, cli.Config, cli.Out, cli.Models)
	k.FatalIfErrorf(err)
}
