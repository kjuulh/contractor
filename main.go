package main

import (
	"log"

	"git.front.kjuulh.io/kjuulh/contractor/cmd/contractor"
)

func main() {
	if err := contractor.RootCmd().Execute(); err != nil {
		log.Fatal(err)
	}
}
