package main

import (
	"log"
	"os"

	cmdplugin "github.com/gitrgoliveira/vault-plugin-database-cmd"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/vault/api"
	dbplugin "github.com/hashicorp/vault/sdk/database/dbplugin/v5"
)

func main() {
	apiClientMeta := &api.PluginAPIClientMeta{}
	flags := apiClientMeta.FlagSet()
	flags.Parse(os.Args[1:])

	err := Run()
	if err != nil {
		log.Println(err)
		logger := hclog.New(&hclog.LoggerOptions{})

		logger.Error("plugin shutting down", "error", err)
		os.Exit(1)
	}
}

func Run() error {
	dbplugin.ServeMultiplex(cmdplugin.New)

	return nil
}
