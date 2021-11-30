package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/SimonRichardson/juju-api-example/api"
	"github.com/SimonRichardson/juju-api-example/client"
	"github.com/juju/charm/v8"
)

func main() {
	client, err := client.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	// statusAPI := api.NewStatusAPI(client)
	// status, err := statusAPI.FullStatus(nil)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// dump("Status", status)

	// modelsAPI := api.NewModelsAPI(client)
	// models, err := modelsAPI.Models()
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// dump("Models", models)

	applicationsAPI := api.NewApplicationsAPI(client)
	if err := applicationsAPI.Deploy("default", "ubuntu", api.DeployArgs{
		Channel: charm.MakePermissiveChannel("latest", "stable", ""),
	}); err != nil {
		log.Fatal(err)
	}
}

func dump(name string, value interface{}) {
	output, _ := json.MarshalIndent(value, "", "	")
	fmt.Println("=== ", name)
	fmt.Println(string(output))
	fmt.Println()
}
