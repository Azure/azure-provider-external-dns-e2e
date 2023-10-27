package main

import (
	"fmt"

	"github.com/Azure/azure-provider-external-dns-e2e/cmd"
)

func main() {
	fmt.Println("In main.go ------------- for new cmd")
	if err := cmd.Execute(); err != nil {
		panic(err)
	}
}
