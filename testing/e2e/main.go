package main

import (
	"github.com/Azure/azure-provider-external-dns-e2e/testing/e2e/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		panic(err)
	}
}
