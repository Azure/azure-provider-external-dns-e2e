package main

import (
	"azure-provider-external-dns-e2e/cmd"
	"fmt"
)

func main() {
	fmt.Println("In main.go ------------- for new cmd")
	if err := cmd.Execute(); err != nil {
		panic(err)
	}
}
