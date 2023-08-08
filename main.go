package main

import (
	"log"

	"git.front.kjuulh.io/kjuulh/contractor/cmd/contractor"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("DEBUG: no .env file found")
	}

	if err := contractor.RootCmd().Execute(); err != nil {
		log.Fatal(err)
	}
}
