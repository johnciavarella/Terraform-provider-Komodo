package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/moghtech/terraform-provider-komodo/internal/provider"
)

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"komodo": providerserver.NewProtocol6WithError(provider.New("test")()),
}

func TestAccServerResource_basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-test")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and read.
			{
				Config: testAccServerResourceConfig(rName, "http://localhost:8120"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("komodo_server.test", "id"),
					resource.TestCheckResourceAttr("komodo_server.test", "name", rName),
					resource.TestCheckResourceAttr("komodo_server.test", "address", "http://localhost:8120"),
					resource.TestCheckResourceAttr("komodo_server.test", "enabled", "true"),
				),
			},
			// Import.
			{
				ResourceName:      "komodo_server.test",
				ImportState:       true,
				ImportStateVerify: true,
				// Passkey is sensitive and won't be returned on read from some APIs.
				ImportStateVerifyIgnore: []string{"passkey"},
			},
			// Update address.
			{
				Config: testAccServerResourceConfig(rName, "http://192.168.1.100:8120"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("komodo_server.test", "address", "http://192.168.1.100:8120"),
				),
			},
		},
	})
}

func testAccServerResourceConfig(name, address string) string {
	return fmt.Sprintf(`
resource "komodo_server" "test" {
  name    = %[1]q
  address = %[2]q
  enabled = true
}
`, name, address)
}

func TestAccStackResource_basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-test")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccStackResourceConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("komodo_stack.test", "id"),
					resource.TestCheckResourceAttr("komodo_stack.test", "name", rName),
				),
			},
			{
				ResourceName:      "komodo_stack.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccStackResourceConfig(name string) string {
	return fmt.Sprintf(`
resource "komodo_stack" "test" {
  name = %[1]q
}
`, name)
}

func TestAccDeploymentResource_basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-test")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDeploymentResourceConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("komodo_deployment.test", "id"),
					resource.TestCheckResourceAttr("komodo_deployment.test", "name", rName),
					resource.TestCheckResourceAttr("komodo_deployment.test", "image", "nginx"),
				),
			},
			{
				ResourceName:      "komodo_deployment.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccDeploymentResourceConfig(name string) string {
	return fmt.Sprintf(`
resource "komodo_deployment" "test" {
  name  = %[1]q
  image = "nginx"
}
`, name)
}

func TestAccTagResource_basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-test")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTagResourceConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("komodo_tag.test", "id"),
					resource.TestCheckResourceAttr("komodo_tag.test", "name", rName),
				),
			},
			{
				ResourceName:      "komodo_tag.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccTagResourceConfig(name string) string {
	return fmt.Sprintf(`
resource "komodo_tag" "test" {
  name = %[1]q
}
`, name)
}

func TestAccBuildResource_basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-test")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBuildResourceConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("komodo_build.test", "id"),
					resource.TestCheckResourceAttr("komodo_build.test", "name", rName),
				),
			},
			{
				ResourceName:      "komodo_build.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccBuildResourceConfig(name string) string {
	return fmt.Sprintf(`
resource "komodo_build" "test" {
  name = %[1]q
  repo = "myorg/test-repo"
}
`, name)
}

func TestAccRepoResource_basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-test")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRepoResourceConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("komodo_repo.test", "id"),
					resource.TestCheckResourceAttr("komodo_repo.test", "name", rName),
				),
			},
			{
				ResourceName:      "komodo_repo.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccRepoResourceConfig(name string) string {
	return fmt.Sprintf(`
resource "komodo_repo" "test" {
  name   = %[1]q
  repo   = "myorg/test-repo"
  branch = "main"
}
`, name)
}
