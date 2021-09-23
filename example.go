package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/SimonRichardson/juju-api-example/api"
	"github.com/SimonRichardson/juju-api-example/client"
)

func main() {
	client, err := client.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	statusAPI := api.NewStatusAPI(client)
	status, err := statusAPI.FullStatus(nil)
	if err != nil {
		log.Fatal(err)
	}
	dump("Status", status)

	modelsAPI := api.NewModelsAPI(client)
	models, err := modelsAPI.Models()
	if err != nil {
		log.Fatal(err)
	}
	dump("Models", models)
}

func dump(name string, value interface{}) {
	output, _ := json.MarshalIndent(value, "", "	")
	fmt.Println("=== ", name)
	fmt.Println(string(output))
	fmt.Println()
}
