package main

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"terraform-provider-axiom-provider/axiom"
)

func main() {
	providerserver.Serve(context.Background(), axiom.New, providerserver.ServeOpts{
		Address: "registry.terraform.io/axiom/axiom-provider",
	})
}
