package triton

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"time"

	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/joyent/triton-go"
)

var (
	machineStateRunning = "running"
	machineStateDeleted = "deleted"

	machineStateChangeTimeout = 10 * time.Minute

	nicStateDeleted = "deleted"
	nicStateRunning = "running"

	resourceMachineMetadataKeys = map[string]string{
		// semantics: "schema_name": "metadata_name"
		"root_authorized_keys": "root_authorized_keys",
		"user_script":          "user-script",
		"user_data":            "user-data",
		"administrator_pw":     "administrator-pw",
		"cloud_config":         "cloud-init:user-data",
	}
)

func resourceMachine() *schema.Resource {
	return &schema.Resource{
		Create:   resourceMachineCreate,
		Exists:   resourceMachineExists,
		Read:     resourceMachineRead,
		Update:   resourceMachineUpdate,
		Delete:   resourceMachineDelete,
		Timeouts: slowResourceTimeout,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Description:  "Friendly name for machine",
				Type:         schema.TypeString,
				Optional:     true,
				Computed:     true,
				ValidateFunc: resourceMachineValidateName,
			},
			"type": {
				Description: "Machine type (smartmachine or virtualmachine)",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"dataset": {
				Description: "Dataset URN with which the machine was provisioned",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"memory": {
				Description: "Amount of memory allocated to the machine (in Mb)",
				Type:        schema.TypeInt,
				Computed:    true,
			},
			"disk": {
				Description: "Amount of disk allocated to the machine (in Gb)",
				Type:        schema.TypeInt,
				Computed:    true,
			},
			"ips": {
				Description: "IP addresses assigned to the machine",
				Type:        schema.TypeList,
				Computed:    true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"tags": {
				Description: "Machine tags",
				Type:        schema.TypeMap,
				Optional:    true,
			},
			"created": {
				Description: "When the machine was created",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"updated": {
				Description: "When the machine was updated",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"package": {
				Description: "The package for use for provisioning",
				Type:        schema.TypeString,
				Required:    true,
			},
			"image": {
				Description: "UUID of the image",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			"primaryip": {
				Description: "Primary (public) IP address for the machine",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"nic": {
				Description: "Network interface",
				Type:        schema.TypeList,
				Computed:    true,
				Optional:    true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"ip": {
							Description: "NIC's IPv4 address",
							Computed:    true,
							Type:        schema.TypeString,
						},
						"mac": {
							Description: "NIC's MAC address",
							Computed:    true,
							Type:        schema.TypeString,
						},
						"primary": {
							Description: "Whether this is the machine's primary NIC",
							Computed:    true,
							Type:        schema.TypeBool,
						},
						"netmask": {
							Description: "IPv4 netmask",
							Computed:    true,
							Type:        schema.TypeString,
						},
						"gateway": {
							Description: "IPv4 gateway",
							Computed:    true,
							Type:        schema.TypeString,
						},
						"network": {
							Description: "ID of the network to which the NIC is attached",
							Required:    true,
							Type:        schema.TypeString,
						},
						"state": {
							Description: "Provisioning state of the NIC",
							Computed:    true,
							Type:        schema.TypeString,
						},
					},
				},
			},
			"firewall_enabled": {
				Description: "Whether to enable the firewall for this machine",
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
			},
			"domain_names": {
				Description: "List of domain names from Triton CNS",
				Type:        schema.TypeList,
				Computed:    true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},

			// computed resources from metadata
			"root_authorized_keys": {
				Description: "Authorized keys for the root user on this machine",
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
			},
			"user_script": {
				Description: "User script to run on boot (every boot on SmartMachines)",
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
			},
			"cloud_config": {
				Description: "copied to machine on boot",
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
			},
			"user_data": {
				Description: "Data copied to machine on boot",
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
			},
			"administrator_pw": {
				Description: "Administrator's initial password (Windows only)",
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
			},

			// deprecated fields
			"networks": {
				Description: "Desired network IDs",
				Type:        schema.TypeList,
				Optional:    true,
				Computed:    true,
				Deprecated:  "Networks is deprecated, please use `nic`",
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
	}
}

func resourceMachineCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*triton.Client)

	var networks []string
	for _, network := range d.Get("networks").([]interface{}) {
		networks = append(networks, network.(string))
	}
	if nicsRaw, found := d.GetOk("nic"); found {
		for _, nicI := range nicsRaw.([]interface{}) {
			nic := nicI.(map[string]interface{})
			networks = append(networks, nic["network"].(string))
		}
	}

	metadata := map[string]string{}
	for schemaName, metadataKey := range resourceMachineMetadataKeys {
		if v, ok := d.GetOk(schemaName); ok {
			metadata[metadataKey] = v.(string)
		}
	}

	tags := map[string]string{}
	for k, v := range d.Get("tags").(map[string]interface{}) {
		tags[k] = v.(string)
	}

	machine, err := client.Machines().CreateMachine(context.Background(), &triton.CreateMachineInput{
		Name:            d.Get("name").(string),
		Package:         d.Get("package").(string),
		Image:           d.Get("image").(string),
		Networks:        networks,
		Metadata:        metadata,
		Tags:            tags,
		FirewallEnabled: d.Get("firewall_enabled").(bool),
	})
	if err != nil {
		return err
	}

	d.SetId(machine.ID)
	stateConf := &resource.StateChangeConf{
		Target: []string{fmt.Sprintf(machineStateRunning)},
		Refresh: func() (interface{}, string, error) {
			getResp, err := client.Machines().GetMachine(context.Background(), &triton.GetMachineInput{
				ID: d.Id(),
			})
			if err != nil {
				return nil, "", err
			}

			return getResp, getResp.State, nil
		},
		Timeout:    machineStateChangeTimeout,
		MinTimeout: 3 * time.Second,
	}
	_, err = stateConf.WaitForState()
	if err != nil {
		return err
	}

	// refresh state after it provisions
	return resourceMachineRead(d, meta)
}

func resourceMachineExists(d *schema.ResourceData, meta interface{}) (bool, error) {
	client := meta.(*triton.Client)

	return resourceExists(client.Machines().GetMachine(context.Background(), &triton.GetMachineInput{
		ID: d.Id(),
	}))
}

func resourceMachineRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*triton.Client)

	machine, err := client.Machines().GetMachine(context.Background(), &triton.GetMachineInput{
		ID: d.Id(),
	})
	if err != nil {
		return err
	}

	nics, err := client.Machines().ListNICs(context.Background(), &triton.ListNICsInput{
		MachineID: d.Id(),
	})
	if err != nil {
		return err
	}
	// If/when CloudAPI exposes the Interface name of each nic, we could use that
	// to sort the list:
	//
	//sort.Slice(nics, func(i, j int) bool { return nics[i].Interface < nics[j].Interface })

	d.Set("name", machine.Name)
	d.Set("type", machine.Type)
	d.Set("state", machine.State)
	d.Set("dataset", machine.Image)
	d.Set("image", machine.Image)
	d.Set("memory", machine.Memory)
	d.Set("disk", machine.Disk)
	d.Set("ips", machine.IPs)
	d.Set("tags", machine.Tags)
	d.Set("created", machine.Created)
	d.Set("updated", machine.Updated)
	d.Set("package", machine.Package)
	d.Set("image", machine.Image)
	d.Set("primaryip", machine.PrimaryIP)
	d.Set("firewall_enabled", machine.FirewallEnabled)
	d.Set("domain_names", machine.DomainNames)

	// create and update NICs
	var (
		machineNICs []map[string]interface{}
		networks    []string
	)
	for _, nic := range nics {
		machineNICs = append(
			machineNICs,
			map[string]interface{}{
				"ip":      nic.IP,
				"mac":     nic.MAC,
				"primary": nic.Primary,
				"netmask": nic.Netmask,
				"gateway": nic.Gateway,
				"state":   nic.State,
				"network": nic.Network,
			},
		)
		networks = append(networks, nic.Network)
	}
	d.Set("nic", machineNICs)
	d.Set("networks", networks)

	// computed attributes from metadata
	for schemaName, metadataKey := range resourceMachineMetadataKeys {
		d.Set(schemaName, machine.Metadata[metadataKey])
	}

	return nil
}

func resourceMachineUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*triton.Client)

	d.Partial(true)

	if d.HasChange("name") {
		oldNameInterface, newNameInterface := d.GetChange("name")
		oldName := oldNameInterface.(string)
		newName := newNameInterface.(string)

		err := client.Machines().RenameMachine(context.Background(), &triton.RenameMachineInput{
			ID:   d.Id(),
			Name: newName,
		})
		if err != nil {
			return err
		}

		stateConf := &resource.StateChangeConf{
			Pending: []string{oldName},
			Target:  []string{newName},
			Refresh: func() (interface{}, string, error) {
				getResp, err := client.Machines().GetMachine(context.Background(), &triton.GetMachineInput{
					ID: d.Id(),
				})
				if err != nil {
					return nil, "", err
				}

				return getResp, getResp.Name, nil
			},
			Timeout:    machineStateChangeTimeout,
			MinTimeout: 3 * time.Second,
		}
		_, err = stateConf.WaitForState()
		if err != nil {
			return err
		}

		d.SetPartial("name")
	}

	if d.HasChange("tags") {
		tags := map[string]string{}
		for k, v := range d.Get("tags").(map[string]interface{}) {
			tags[k] = v.(string)
		}

		var err error
		if len(tags) == 0 {
			err = client.Machines().DeleteMachineTags(context.Background(), &triton.DeleteMachineTagsInput{
				ID: d.Id(),
			})
		} else {
			err = client.Machines().ReplaceMachineTags(context.Background(), &triton.ReplaceMachineTagsInput{
				ID:   d.Id(),
				Tags: tags,
			})
		}
		if err != nil {
			return err
		}

		expectedTagsMD5 := stableMapHash(tags)
		stateConf := &resource.StateChangeConf{
			Target: []string{expectedTagsMD5},
			Refresh: func() (interface{}, string, error) {
				getResp, err := client.Machines().GetMachine(context.Background(), &triton.GetMachineInput{
					ID: d.Id(),
				})
				if err != nil {
					return nil, "", err
				}

				return getResp, stableMapHash(getResp.Tags), nil
			},
			Timeout:    machineStateChangeTimeout,
			MinTimeout: 3 * time.Second,
		}
		_, err = stateConf.WaitForState()
		if err != nil {
			return err
		}

		d.SetPartial("tags")
	}

	if d.HasChange("package") {
		newPackage := d.Get("package").(string)

		err := client.Machines().ResizeMachine(context.Background(), &triton.ResizeMachineInput{
			ID:      d.Id(),
			Package: newPackage,
		})
		if err != nil {
			return err
		}

		stateConf := &resource.StateChangeConf{
			Target: []string{fmt.Sprintf("%s@%s", newPackage, "running")},
			Refresh: func() (interface{}, string, error) {
				getResp, err := client.Machines().GetMachine(context.Background(), &triton.GetMachineInput{
					ID: d.Id(),
				})
				if err != nil {
					return nil, "", err
				}

				return getResp, fmt.Sprintf("%s@%s", getResp.Package, getResp.State), nil
			},
			Timeout:    machineStateChangeTimeout,
			MinTimeout: 3 * time.Second,
		}
		_, err = stateConf.WaitForState()
		if err != nil {
			return err
		}

		d.SetPartial("package")
	}

	if d.HasChange("nic") {
		// NOTE (seanc@): Due to the way CloudAPI handles NICs and NIC ordering, the
		// order of NICs in this list is significant.  A change to the order of a
		// `nic` implies that all existing NICs must be removed and re-added to the
		// machine.  This is fairly disruptive because a NIC change involves a
		// reboot of the target machine.  For now, favor correctness (i.e. ordering)
		// over availability provisioning time.  The availability of a target
		// machine is being addressed independently.  Where possible, skip the
		// add/remove if the nic at a given index hasn't changed.

		o, n := d.GetChange("nic")
		if o == nil {
			o = []map[string]interface{}{}
		}
		if n == nil {
			n = []map[string]interface{}{}
		}

		// Create a lower-bound index of NICs that haven't changed.  After this
		// index has been crossed, all NICs must be removed/re-added.  A value of -1
		// means all NICs are removed and re-added.
		unchangedLowerBound := -1
		for i, newNICRaw := range n.([]interface{}) {
			if i > len(o.([]interface{}))-1 {
				break
			}

			newNIC := newNICRaw.(map[string]interface{})
			oldNicListRaw := o.([]interface{})
			oldNIC := oldNicListRaw[i].(map[string]interface{})

			identical := true
			if len(newNIC) != len(oldNIC) {
				identical = false
			}
			if identical {
				for k, vNew := range newNIC {
					if vOld, found := oldNIC[k]; found {
						if !reflect.DeepEqual(vNew, vOld) {
							identical = false
							break
						}
					} else {
						identical = false
						break
					}
				}
			}
			if identical {
				unchangedLowerBound = i
			} else {
				break
			}
		}

		for i, nicRaw := range o.([]interface{}) {
			if unchangedLowerBound >= i {
				continue
			}
			nic := nicRaw.(map[string]interface{})
			mac := nic["mac"].(string)
			if err := client.Machines().RemoveNIC(context.Background(), &triton.RemoveNICInput{
				MachineID: d.Id(),
				MAC:       mac,
			}); err != nil {
				return err
			}

			stateConf := &resource.StateChangeConf{
				Target: []string{nicStateDeleted},
				Refresh: func() (interface{}, string, error) {
					getResp, err := client.Machines().GetNIC(context.Background(), &triton.GetNICInput{
						MachineID: d.Id(),
						MAC:       mac,
					})
					if err != nil {
						if triton.IsResourceNotFound(err) {
							return getResp, nicStateDeleted, nil
						}
						return nil, "", err
					}

					return getResp, getResp.State, nil
				},
				Timeout:    machineStateChangeTimeout,
				MinTimeout: 3 * time.Second,
			}
			_, err := stateConf.WaitForState()
			if err != nil {
				return err
			}
		}

		for i, nicRaw := range n.([]interface{}) {
			if unchangedLowerBound >= i {
				continue
			}
			nicConfig := nicRaw.(map[string]interface{})
			network := nicConfig["network"].(string)
			newNic, err := client.Machines().AddNIC(context.Background(), &triton.AddNICInput{
				MachineID: d.Id(),
				Network:   network,
				Primary:   i == 0,
			})
			if err != nil {
				return err
			}

			stateConf := &resource.StateChangeConf{
				Target: []string{nicStateRunning},
				Refresh: func() (interface{}, string, error) {
					getResp, err := client.Machines().GetNIC(context.Background(), &triton.GetNICInput{
						MachineID: d.Id(),
						MAC:       newNic.MAC,
					})
					if err != nil {
						return nil, "", err
					}

					return getResp, getResp.State, nil
				},
				Timeout:    machineStateChangeTimeout,
				MinTimeout: 3 * time.Second,
			}
			_, err = stateConf.WaitForState()
			if err != nil {
				return err
			}
		}
	}

	if d.HasChange("firewall_enabled") {
		enable := d.Get("firewall_enabled").(bool)

		var err error
		if enable {
			err = client.Machines().EnableMachineFirewall(context.Background(), &triton.EnableMachineFirewallInput{
				ID: d.Id(),
			})
		} else {
			err = client.Machines().DisableMachineFirewall(context.Background(), &triton.DisableMachineFirewallInput{
				ID: d.Id(),
			})
		}
		if err != nil {
			return err
		}

		stateConf := &resource.StateChangeConf{
			Target: []string{fmt.Sprintf("%t", enable)},
			Refresh: func() (interface{}, string, error) {
				getResp, err := client.Machines().GetMachine(context.Background(), &triton.GetMachineInput{
					ID: d.Id(),
				})
				if err != nil {
					return nil, "", err
				}

				return getResp, fmt.Sprintf("%t", getResp.FirewallEnabled), nil
			},
			Timeout:    machineStateChangeTimeout,
			MinTimeout: 3 * time.Second,
		}
		_, err = stateConf.WaitForState()
		if err != nil {
			return err
		}

		d.SetPartial("firewall_enabled")
	}

	metadata := map[string]string{}
	for schemaName, metadataKey := range resourceMachineMetadataKeys {
		if d.HasChange(schemaName) {
			metadata[metadataKey] = d.Get(schemaName).(string)
		}
	}
	if len(metadata) > 0 {
		if _, err := client.Machines().UpdateMachineMetadata(context.Background(), &triton.UpdateMachineMetadataInput{
			ID:       d.Id(),
			Metadata: metadata,
		}); err != nil {
			return err
		}

		stateConf := &resource.StateChangeConf{
			Target: []string{"converged"},
			Refresh: func() (interface{}, string, error) {
				getResp, err := client.Machines().GetMachine(context.Background(), &triton.GetMachineInput{
					ID: d.Id(),
				})
				if err != nil {
					return nil, "", err
				}

				for k, v := range metadata {
					if upstream, ok := getResp.Metadata[k]; !ok || v != upstream {
						return getResp, "converging", nil
					}
				}

				return getResp, "converged", nil
			},
			Timeout:    machineStateChangeTimeout,
			MinTimeout: 3 * time.Second,
		}
		_, err := stateConf.WaitForState()
		if err != nil {
			return err
		}

		for schemaName := range resourceMachineMetadataKeys {
			if d.HasChange(schemaName) {
				d.SetPartial(schemaName)
			}
		}
	}

	d.Partial(false)

	return resourceMachineRead(d, meta)
}

func resourceMachineDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*triton.Client)

	err := client.Machines().DeleteMachine(context.Background(), &triton.DeleteMachineInput{
		ID: d.Id(),
	})
	if err != nil {
		return err
	}

	stateConf := &resource.StateChangeConf{
		Target: []string{machineStateDeleted},
		Refresh: func() (interface{}, string, error) {
			getResp, err := client.Machines().GetMachine(context.Background(), &triton.GetMachineInput{
				ID: d.Id(),
			})
			if err != nil {
				if triton.IsResourceNotFound(err) {
					return getResp, "deleted", nil
				}
				return nil, "", err
			}

			return getResp, getResp.State, nil
		},
		Timeout:    machineStateChangeTimeout,
		MinTimeout: 3 * time.Second,
	}
	_, err = stateConf.WaitForState()
	if err != nil {
		return err
	}

	return nil
}

func resourceMachineValidateName(value interface{}, name string) (warnings []string, errors []error) {
	warnings = []string{}
	errors = []error{}

	r := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9\_\.\-]*$`)
	if !r.Match([]byte(value.(string))) {
		errors = append(errors, fmt.Errorf(`"%s" is not a valid %s`, value.(string), name))
	}

	return warnings, errors
}
