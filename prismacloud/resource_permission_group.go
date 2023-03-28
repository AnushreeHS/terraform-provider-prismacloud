package prismacloud

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/mitchellh/mapstructure"
	pc "github.com/paloaltonetworks/prisma-cloud-go"
	"github.com/paloaltonetworks/prisma-cloud-go/permission_group"
	"golang.org/x/net/context"
)

func resourcePermissionGroup() *schema.Resource {
	return &schema.Resource{
		CreateContext: createPermissionGroup,
		ReadContext:   readPermissionGroup,
		UpdateContext: updatePermissionGroup,
		DeleteContext: deletePermissionGroup,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the permission group",
			},
			"description": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Description",
			},
			"permission_group_type": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Permission group type",
				ValidateFunc: validation.StringInSlice(
					[]string{
						permission_group.TypeDefault,
						permission_group.TypeCustom,
						permission_group.TypeInternal,
					},
					false,
				),
			},
			"last_modified_by": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Last modified by",
			},
			"last_modified_ts": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Last modified timestamp",
			},
			"associated_roles": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Associated permission roles",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"role_id": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The role ID",
						},
						"name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Role name",
						},
					},
				},
			},
			"features": {
				Type:        schema.TypeList,
				Required:    true,
				Description: "Features",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"feature_name": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Feature name",
						},
						"operations": {
							Type:        schema.TypeList,
							Required:    true,
							Description: "Operations",
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									//A mapping of operations and a boolean value representing whether the privilege to perform the operation needs to be granted.
									"create": {
										Type:        schema.TypeBool,
										Optional:    true,
										Description: "Create operation",
									},
									"read": {
										Type:        schema.TypeBool,
										Optional:    true,
										Description: "Read operation",
									},
									"update": {
										Type:        schema.TypeBool,
										Optional:    true,
										Description: "Update operation",
									},
									"delete": {
										Type:        schema.TypeBool,
										Optional:    true,
										Description: "Delete operation",
									},
								},
							},
						},
					},
				},
			},
			"accept_account_groups": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Accept account groups",
			},
			"accept_resource_lists": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Accept resource lists",
			},
			"accept_code_repositories": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Accept code repositories",
			},
			"custom": { //Boolean value signifying whether this is a custom (i.e. user-defined) permission group. Is set to true if the attribute value of permissionGroupType is set to CUSTOM
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Custom",
			},
			"id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Permission group id",
			},
		},
	}
}

func parsePermissionGroup(d *schema.ResourceData) permission_group.PermissionGroup {
	ans := permission_group.PermissionGroup{
		Name:        d.Get("name").(string),
		Description: d.Get("description").(string),
	}

	cld := d.Get("features").([]interface{})

	ans.Features = make([]permission_group.Features, 0, len(cld))
	for _, fe := range cld {
		cl := fe.(map[string]interface{})
		var md permission_group.Operations
		err := mapstructure.Decode(cl["operations"].([]interface{})[0], &md)
		if err != nil {
			panic(err)
		}
		ans.Features = append(ans.Features, permission_group.Features{
			FeatureName: cl["feature_name"].(string),
			Operations:  md,
		})
	}

	return ans
}

func savePermissionGroup(d *schema.ResourceData, obj permission_group.PermissionGroup) {

	d.Set("name", obj.Name)
	d.Set("description", obj.Description)
	d.Set("last_modified_by", obj.LastModifiedBy)
	d.Set("last_modified_ts", obj.LastModifiedTs)
	d.Set("permission_group_type", obj.Type)
	d.Set("accept_account_groups", obj.AcceptAccountGroups)
	d.Set("accept_resource_lists", obj.AcceptResourceLists)
	d.Set("associated_roles", obj.AssociatedRoles)
	d.Set("accept_code_repositories", obj.AcceptCodeRepositories)
	d.Set("custom", obj.Custom)

	feat := make([]interface{}, 0, len(obj.Features))
	for _, fe := range obj.Features {
		feat = append(feat, map[string]interface{}{
			"feature_name": fe.FeatureName,
			"operations":   map[string]bool{"create": fe.Operations.CREATE, "read": fe.Operations.READ, "update": fe.Operations.UPDATE, "delete": fe.Operations.DELETE},
		})
	}
}
func createPermissionGroup(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*pc.Client)
	obj := parsePermissionGroup(d)

	if err := permission_group.Create(client, obj); err != nil {
		return diag.FromErr(err)
	}

	PollApiUntilSuccess(func() error {
		_, err := permission_group.Identify(client, obj.Name)
		return err
	})

	id, err := permission_group.Identify(client, obj.Name)
	if err != nil {
		return diag.FromErr(err)
	}

	PollApiUntilSuccess(func() error {
		_, err := permission_group.Get(client, id)
		return err
	})

	d.SetId(id)
	return readPermissionGroup(ctx, d, meta)
}

func readPermissionGroup(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*pc.Client)
	id := d.Id()

	obj, err := permission_group.Get(client, id)
	if err != nil {
		if err == pc.InvalidPermissionGroupIdError {
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}

	savePermissionGroup(d, obj)

	return nil
}

func updatePermissionGroup(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*pc.Client)
	obj := parsePermissionGroup(d)
	obj.Id = d.Id()

	if err := permission_group.Update(client, obj); err != nil {
		return diag.FromErr(err)
	}

	return readPermissionGroup(ctx, d, meta)
}

func deletePermissionGroup(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*pc.Client)
	id := d.Id()

	err := permission_group.Delete(client, id)
	if err != nil {
		if err != pc.ObjectNotFoundError {
			return diag.FromErr(err)
		}
	}

	d.SetId("")
	return nil
}