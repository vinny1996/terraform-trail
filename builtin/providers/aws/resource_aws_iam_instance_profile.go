package aws

import (
	"fmt"

	"github.com/hashicorp/aws-sdk-go/aws"
	"github.com/hashicorp/aws-sdk-go/gen/iam"

	"github.com/hashicorp/terraform/helper/schema"
)

func resourceAwsIamInstanceProfile() *schema.Resource {
	return &schema.Resource{
		Create: resourceAwsIamInstanceProfileCreate,
		Read:   resourceAwsIamInstanceProfileRead,
		Update: resourceAwsIamInstanceProfileUpdate,
		Delete: resourceAwsIamInstanceProfileDelete,

		Schema: map[string]*schema.Schema{
			"arn": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},
			"create_date": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},
			"unique_id": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},
			"name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"path": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Default:  "/",
				ForceNew: true,
			},
			"roles": &schema.Schema{
				Type:     schema.TypeSet,
				Required: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set:      schema.HashString,
			},
		},
	}
}

func resourceAwsIamInstanceProfileCreate(d *schema.ResourceData, meta interface{}) error {
	iamconn := meta.(*AWSClient).iamconn
	name := d.Get("name").(string)

	request := &iam.CreateInstanceProfileRequest{
		InstanceProfileName: aws.String(name),
		Path:                aws.String(d.Get("path").(string)),
	}

	response, err := iamconn.CreateInstanceProfile(request)
	if err == nil {
		err = instanceProfileReadResult(d, response.InstanceProfile)
	}
	if err != nil {
		return fmt.Errorf("Error creating IAM instance profile %s: %s", name, err)
	}

	return instanceProfileSetRoles(d, iamconn)
}

func instanceProfileAddRole(iamconn *iam.IAM, profileName, roleName string) error {
	request := &iam.AddRoleToInstanceProfileRequest{
		InstanceProfileName: aws.String(profileName),
		RoleName:            aws.String(roleName),
	}

	return iamconn.AddRoleToInstanceProfile(request)
}

func instanceProfileRemoveRole(iamconn *iam.IAM, profileName, roleName string) error {
	request := &iam.RemoveRoleFromInstanceProfileRequest{
		InstanceProfileName: aws.String(profileName),
		RoleName:            aws.String(roleName),
	}

	return iamconn.RemoveRoleFromInstanceProfile(request)
}

func instanceProfileSetRoles(d *schema.ResourceData, iamconn *iam.IAM) error {
	oldInterface, newInterface := d.GetChange("roles")
	oldRoles := oldInterface.(*schema.Set)
	newRoles := newInterface.(*schema.Set)

	currentRoles := schema.CopySet(oldRoles)

	d.Partial(true)

	for _, role := range oldRoles.Difference(newRoles).List() {
		err := instanceProfileRemoveRole(iamconn, d.Id(), role.(string))
		if err != nil {
			return fmt.Errorf("Error removing role %s from IAM instance profile %s: %s", role, d.Id(), err)
		}
		currentRoles.Remove(role)
		d.Set("roles", currentRoles)
		d.SetPartial("roles")
	}

	for _, role := range newRoles.Difference(oldRoles).List() {
		err := instanceProfileAddRole(iamconn, d.Id(), role.(string))
		if err != nil {
			return fmt.Errorf("Error adding role %s to IAM instance profile %s: %s", role, d.Id(), err)
		}
		currentRoles.Add(role)
		d.Set("roles", currentRoles)
		d.SetPartial("roles")
	}

	d.Partial(false)

	return nil
}

func resourceAwsIamInstanceProfileUpdate(d *schema.ResourceData, meta interface{}) error {
	iamconn := meta.(*AWSClient).iamconn

	if !d.HasChange("roles") {
		return nil
	}

	return instanceProfileSetRoles(d, iamconn)
}

func resourceAwsIamInstanceProfileRead(d *schema.ResourceData, meta interface{}) error {
	iamconn := meta.(*AWSClient).iamconn

	request := &iam.GetInstanceProfileRequest{
		InstanceProfileName: aws.String(d.Id()),
	}

	result, err := iamconn.GetInstanceProfile(request)
	if err != nil {
		if iamerr, ok := err.(aws.APIError); ok && iamerr.Code == "NoSuchEntity" {
			d.SetId("")
			return nil
		}
		return fmt.Errorf("Error reading IAM instance profile %s: %s", d.Id(), err)
	}

	return instanceProfileReadResult(d, result.InstanceProfile)
}

func resourceAwsIamInstanceProfileDelete(d *schema.ResourceData, meta interface{}) error {
	iamconn := meta.(*AWSClient).iamconn

	request := &iam.DeleteInstanceProfileRequest{
		InstanceProfileName: aws.String(d.Id()),
	}
	err := iamconn.DeleteInstanceProfile(request)
	if err != nil {
		return fmt.Errorf("Error deleting IAM instance profile %s: %s", d.Id(), err)
	}
	d.SetId("")
	return nil
}

func instanceProfileReadResult(d *schema.ResourceData, result *iam.InstanceProfile) error {
	d.SetId(*result.InstanceProfileName)
	if err := d.Set("name", result.InstanceProfileName); err != nil {
		return err
	}
	if err := d.Set("path", result.Path); err != nil {
		return err
	}

	roles := &schema.Set{F: schema.HashString}
	for _, role := range result.Roles {
		roles.Add(*role.RoleName)
	}
	if err := d.Set("roles", roles); err != nil {
		return err
	}

	return nil
}
