package main

import (
	"log"

	client "github.com/sjm1327605995/goygopro/client/tenon"
)

func main() {
	if err := client.Run(); err != nil {
		log.Fatal(err)
	}
}
