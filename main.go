package main

import (
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6/tf6server"
	"terraform-provider-axiom-provider/axiom"
)

//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs
func main() {
	err := tf6server.Serve(
		"registry.terraform.io/axiom/axiom-provider",
		providerserver.NewProtocol6(axiom.NewAxiomProvider()),
	)
	if err != nil {
		log.Fatal(err)
	}
}
