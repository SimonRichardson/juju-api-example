package main

import (
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
	fmt.Println(statusAPI.FullStatus(nil))
}
