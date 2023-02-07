package main

import (
	"context"
	"terraform-provider-axiom-provider/axiom"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

func main() {
	providerserver.Serve(context.Background(), axiom.New, providerserver.ServeOpts{
		Address: "registry.terraform.io/axiom/axiom-provider",
	})
}
