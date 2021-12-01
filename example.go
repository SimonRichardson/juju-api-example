package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/SimonRichardson/juju-api-example/api"
	"github.com/SimonRichardson/juju-api-example/client"
	"github.com/juju/charm/v8"
)

func main() {
	client, err := client.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	applicationsAPI := api.NewApplicationsAPI(client)
	if err := applicationsAPI.Deploy("default", "ubuntu", api.DeployArgs{
		Channel:         charm.MakePermissiveChannel("latest", "stable", ""),
		Revision:        -1,
		NumUnits:        1,
		ApplicationName: "ubuntu",
	}); err != nil {
		log.Fatalf("%+v\n", err)
	}

	statusAPI := api.NewStatusAPI(client)

	ticker := time.NewTicker(time.Second)
	for range ticker.C {
		status, err := statusAPI.FullStatus(nil)
		if err != nil {
			log.Fatal(err)
		}
		dump("Status", status)
	}
}

func dump(name string, value interface{}) {
	output, _ := json.MarshalIndent(value, "", "	")
	fmt.Println("=== ", name)
	fmt.Println(string(output))
	fmt.Println()
}
