package postgresql_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stackitcloud/stackit-sdk-go/core/config"
	"github.com/stackitcloud/stackit-sdk-go/core/utils"
	"github.com/stackitcloud/stackit-sdk-go/services/postgresql"
	"github.com/stackitcloud/terraform-provider-stackit/stackit/core"
	"github.com/stackitcloud/terraform-provider-stackit/stackit/testutil"
)

// Instance resource data
var instanceResource = map[string]string{
	"project_id":        testutil.ProjectId,
	"name":              testutil.ResourceNameWithDateTime("postgresql"),
	"plan_id":           "57d40175-0f4c-4bcc-b52d-cf5d2ee9f5a7",
	"sgw_acl":           "192.168.0.0/16",
	"metrics_frequency": "34",
	"plugins":           "foo-bar",
}

func resourceConfig(acls, frequency, plugins string) string {
	return fmt.Sprintf(`
				%s

				resource "stackit_postgresql_instance" "instance" {
					project_id = "%s"
					name    = "%s"
					plan_id = "%s"
					parameters = {
						sgw_acl = "%s"
						plugins = ["%s"] 
						# metrics_frequency = %s
						# metrics_prefix = "pre"
						# enable_monitoring = true
						# monitoring_instance_id = "b9e38481-4f3d-4a28-8ed0-43fd32c024c7"
					}
				}

				resource "stackit_postgresql_credentials" "credentials" {
					project_id = stackit_postgresql_instance.instance.project_id
					instance_id = stackit_postgresql_instance.instance.instance_id
				}
				`,
		testutil.PostgreSQLProviderConfig(),
		instanceResource["project_id"],
		instanceResource["name"],
		instanceResource["plan_id"],
		acls,
		plugins,
		frequency,
	)
}
func TestAccPostgreSQLResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.TestAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPostgreSQLDestroy,
		Steps: []resource.TestStep{

			// Creation
			{
				Config: resourceConfig(instanceResource["sgw_acl"], instanceResource["metrics_frequency"], instanceResource["plugins"]),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Instance data
					resource.TestCheckResourceAttr("stackit_postgresql_instance.instance", "project_id", instanceResource["project_id"]),
					resource.TestCheckResourceAttrSet("stackit_postgresql_instance.instance", "instance_id"),
					resource.TestCheckResourceAttr("stackit_postgresql_instance.instance", "plan_id", instanceResource["plan_id"]),
					resource.TestCheckResourceAttr("stackit_postgresql_instance.instance", "name", instanceResource["name"]),
					resource.TestCheckResourceAttr("stackit_postgresql_instance.instance", "parameters.sgw_acl", instanceResource["sgw_acl"]),

					// Credentials data
					resource.TestCheckResourceAttrPair(
						"stackit_postgresql_credentials.credentials", "project_id",
						"stackit_postgresql_instance.instance", "project_id",
					),
					resource.TestCheckResourceAttrPair(
						"stackit_postgresql_credentials.credentials", "instance_id",
						"stackit_postgresql_instance.instance", "instance_id",
					),
					resource.TestCheckResourceAttrSet("stackit_postgresql_credentials.credentials", "credentials_id"),
					resource.TestCheckResourceAttrSet("stackit_postgresql_credentials.credentials", "host"),
				),
			},
			{ // Data source
				Config: fmt.Sprintf(`
					%s

					data "stackit_postgresql_instance" "instance" {
						project_id  = stackit_postgresql_instance.instance.project_id
						instance_id = stackit_postgresql_instance.instance.instance_id
					}

					data "stackit_postgresql_credentials" "credentials" {
						project_id     = stackit_postgresql_credentials.credentials.project_id
						instance_id    = stackit_postgresql_credentials.credentials.instance_id
					    credentials_id = stackit_postgresql_credentials.credentials.credentials_id
					}`,
					resourceConfig(instanceResource["sgw_acl"], instanceResource["metrics_frequency"], instanceResource["plugins"]),
				),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Instance data
					resource.TestCheckResourceAttr("data.stackit_postgresql_instance.instance", "project_id", instanceResource["project_id"]),

					resource.TestCheckResourceAttrPair("stackit_postgresql_instance.instance", "instance_id",
						"data.stackit_postgresql_instance.instance", "instance_id"),

					resource.TestCheckResourceAttrPair("stackit_postgresql_credentials.credentials", "credentials_id",
						"data.stackit_postgresql_credentials.credentials", "credentials_id"),

					resource.TestCheckResourceAttr("data.stackit_postgresql_instance.instance", "plan_id", instanceResource["plan_id"]),

					resource.TestCheckResourceAttr("data.stackit_postgresql_instance.instance", "name", instanceResource["name"]),
					resource.TestCheckResourceAttr("data.stackit_postgresql_instance.instance", "parameters.sgw_acl", instanceResource["sgw_acl"]),
					resource.TestCheckResourceAttr("data.stackit_postgresql_instance.instance", "parameters.plugins.#", "1"),
					resource.TestCheckResourceAttr("data.stackit_postgresql_instance.instance", "parameters.plugins.0", instanceResource["plugins"]),

					// Credentials data
					resource.TestCheckResourceAttr("data.stackit_postgresql_credentials.credentials", "project_id", instanceResource["project_id"]),
					resource.TestCheckResourceAttrSet("data.stackit_postgresql_credentials.credentials", "credentials_id"),
					resource.TestCheckResourceAttrSet("data.stackit_postgresql_credentials.credentials", "host"),
					resource.TestCheckResourceAttrSet("data.stackit_postgresql_credentials.credentials", "port"),
					resource.TestCheckResourceAttrSet("data.stackit_postgresql_credentials.credentials", "uri"),
				),
			},
			// Import
			{
				ResourceName: "stackit_postgresql_instance.instance",
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					r, ok := s.RootModule().Resources["stackit_postgresql_instance.instance"]
					if !ok {
						return "", fmt.Errorf("couldn't find resource stackit_postgresql_instance.instance")
					}
					instanceId, ok := r.Primary.Attributes["instance_id"]
					if !ok {
						return "", fmt.Errorf("couldn't find attribute instance_id")
					}

					return fmt.Sprintf("%s,%s", testutil.ProjectId, instanceId), nil
				},
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName: "stackit_postgresql_credentials.credentials",
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					r, ok := s.RootModule().Resources["stackit_postgresql_credentials.credentials"]
					if !ok {
						return "", fmt.Errorf("couldn't find resource stackit_postgresql_credentials.credentials")
					}
					instanceId, ok := r.Primary.Attributes["instance_id"]
					if !ok {
						return "", fmt.Errorf("couldn't find attribute instance_id")
					}
					credentialsId, ok := r.Primary.Attributes["credentials_id"]
					if !ok {
						return "", fmt.Errorf("couldn't find attribute credentials_id")
					}

					return fmt.Sprintf("%s,%s,%s", testutil.ProjectId, instanceId, credentialsId), nil
				},
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update
			{
				Config: resourceConfig(instanceResource["sgw_acl"], fmt.Sprintf("%s0", instanceResource["metrics_frequency"]), fmt.Sprintf("%s-baz", instanceResource["plugins"])),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Instance data
					resource.TestCheckResourceAttr("stackit_postgresql_instance.instance", "project_id", instanceResource["project_id"]),
					resource.TestCheckResourceAttrSet("stackit_postgresql_instance.instance", "instance_id"),
					resource.TestCheckResourceAttr("stackit_postgresql_instance.instance", "plan_id", instanceResource["plan_id"]),
					resource.TestCheckResourceAttr("stackit_postgresql_instance.instance", "name", instanceResource["name"]),
					resource.TestCheckResourceAttr("stackit_postgresql_instance.instance", "parameters.sgw_acl", instanceResource["sgw_acl"]),
					resource.TestCheckResourceAttr("stackit_postgresql_instance.instance", "parameters.plugins.0", fmt.Sprintf("%s-baz", instanceResource["plugins"])),
				),
			},
			// Deletion is done by the framework implicitly
		},
	})
}

func testAccCheckPostgreSQLDestroy(s *terraform.State) error {
	ctx := context.Background()
	var client *postgresql.APIClient
	var err error
	if testutil.PostgreSQLCustomEndpoint == "" {
		client, err = postgresql.NewAPIClient()
	} else {
		client, err = postgresql.NewAPIClient(
			config.WithEndpoint(testutil.PostgreSQLCustomEndpoint),
		)
	}
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}

	instancesToDestroy := []string{}
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "stackit_postgresql_instance" {
			continue
		}
		// instance terraform ID: "[project_id],[instance_id]"
		instanceId := strings.Split(rs.Primary.ID, core.Separator)[1]
		instancesToDestroy = append(instancesToDestroy, instanceId)
	}

	instancesResp, err := client.GetInstances(ctx, testutil.ProjectId).Execute()
	if err != nil {
		return fmt.Errorf("getting instancesResp: %w", err)
	}

	instances := *instancesResp.Instances
	for i := range instances {
		if instances[i].InstanceId == nil {
			continue
		}
		if utils.Contains(instancesToDestroy, *instances[i].InstanceId) {
			if !checkInstanceDeleteSuccess(&instances[i]) {
				err := client.DeleteInstanceExecute(ctx, testutil.ProjectId, *instances[i].InstanceId)
				if err != nil {
					return fmt.Errorf("destroying instance %s during CheckDestroy: %w", *instances[i].InstanceId, err)
				}
				_, err = postgresql.DeleteInstanceWaitHandler(ctx, client, testutil.ProjectId, *instances[i].InstanceId).WaitWithContext(ctx)
				if err != nil {
					return fmt.Errorf("destroying instance %s during CheckDestroy: waiting for deletion %w", *instances[i].InstanceId, err)
				}
			}
		}
	}
	return nil
}

func checkInstanceDeleteSuccess(i *postgresql.Instance) bool {
	if *i.LastOperation.Type != postgresql.InstanceTypeDelete {
		return false
	}

	if *i.LastOperation.Type == postgresql.InstanceTypeDelete {
		if *i.LastOperation.State != postgresql.InstanceStateSuccess {
			return false
		} else if strings.Contains(*i.LastOperation.Description, "DeleteFailed") || strings.Contains(*i.LastOperation.Description, "failed") {
			return false
		}
	}
	return true
}
