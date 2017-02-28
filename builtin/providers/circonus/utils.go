package circonus

import (
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/circonus-labs/circonus-gometrics/api"
	"github.com/circonus-labs/circonus-gometrics/api/config"
	"github.com/hashicorp/errwrap"
	"github.com/hashicorp/terraform/helper/schema"
)

func castSchemaToTF(in map[schemaAttr]*schema.Schema, descrs attrDescrs) map[string]*schema.Schema {
	out := make(map[string]*schema.Schema, len(in))
	for k, v := range in {
		if descr, ok := descrs[k]; ok {
			// NOTE(sean@): At some point this check needs to be uncommented and all
			// missing descriptions need to be populated.
			//
			// if len(descr) == 0 {
			// 	panic(fmt.Sprintf("PROVIDER BUG: Description of attribute %s empty", k))
			// }

			v.Description = string(descr)
		} else {
			panic(fmt.Sprintf("PROVIDER BUG: Unable to find description for attr %q", k))
		}

		out[string(k)] = v
	}

	return out
}

func failoverGroupIDToCID(groupID int) string {
	if groupID == 0 {
		return ""
	}

	return fmt.Sprintf("%s/%d", config.ContactGroupPrefix, groupID)
}

func failoverGroupCIDToID(cid api.CIDType) (int, error) {
	re := regexp.MustCompile("^" + config.ContactGroupPrefix + "/(" + config.DefaultCIDRegex + ")$")
	matches := re.FindStringSubmatch(string(*cid))
	if matches == nil || len(matches) < 2 {
		return -1, fmt.Errorf("Did not find a valid contact_group ID in the CID %q", string(*cid))
	}

	contactGroupID, err := strconv.Atoi(matches[1])
	if err != nil {
		return -1, errwrap.Wrapf(fmt.Sprintf("invalid contact_group ID: unable to find an ID in %q: {{error}}", string(*cid)), err)
	}

	return contactGroupID, nil
}

// flattenList returns a list of all string values to a []*string.
func flattenList(l []interface{}) []*string {
	vals := make([]*string, 0, len(l))
	for _, v := range l {
		val, ok := v.(string)
		if ok && val != "" {
			vals = append(vals, &val)
		}
	}
	return vals
}

// flattenSet flattens the values in a schema.Set and returns a []*string
func flattenSet(s *schema.Set) []*string {
	return flattenList(s.List())
}

// listToSet returns a TypeSet from the given list.
func stringListToSet(stringList []string, keyName schemaAttr) []interface{} {
	m := make([]interface{}, 0, len(stringList))
	for _, v := range stringList {
		s := make(map[string]interface{}, 1)
		s[string(keyName)] = v
		m = append(m, s)
	}

	return m
}

func normalizeTimeDurationStringToSeconds(v interface{}) string {
	switch v.(type) {
	case string:
		d, err := time.ParseDuration(v.(string))
		if err != nil {
			return fmt.Sprintf("<unable to normalize time duration %s: %v>", v.(string), err)
		}

		return fmt.Sprintf("%ds", int(d.Seconds()))
	default:
		return fmt.Sprintf("<unable to normalize duration on %#v>", v)
	}
}

// configGetBool returns the boolean value if found.
func configGetBool(d *schema.ResourceData, attrName schemaAttr) bool {
	return d.Get(string(attrName)).(bool)
}

// configGetBoolOk returns the boolean value if found and true as the second
// argument, otherwise returns false if the value was not found.
func configGetBoolOK(d *schema.ResourceData, attrName schemaAttr) (b, found bool) {
	if v, ok := d.GetOk(string(attrName)); ok {
		return v.(bool), true
	}

	return false, false
}

func configGetDurationOK(d *schema.ResourceData, attrName schemaAttr) (time.Duration, bool) {
	if v, ok := d.GetOk(string(attrName)); ok {
		d, err := time.ParseDuration(v.(string))
		if err != nil {
			return time.Duration(0), false
		}

		return d, true
	}

	return time.Duration(0), false
}

func schemaGetSetAsListOk(d *schema.ResourceData, attrName schemaAttr) (interfaceList, bool) {
	if listRaw, ok := d.GetOk(string(attrName)); ok {
		return listRaw.(*schema.Set).List(), true
	}
	return nil, false
}

// configGetString returns an attribute as a string.  If the attribute is not
// found, return an empty string.
func configGetString(d *schema.ResourceData, attrName schemaAttr) string {
	if s, ok := schemaGetStringOK(d, attrName); ok {
		return s
	}

	return ""
}

// schemaGetStringOK returns an attribute as a string and true if the attribute
// was found.  If the attribute is not found, return an empty string.
func schemaGetStringOK(d *schema.ResourceData, attrName schemaAttr) (string, bool) {
	if v, ok := d.GetOk(string(attrName)); ok {
		return v.(string), ok
	}

	return "", false
}

// configGetStringPtr returns an attribute as a *string.  If the attribute is
// not found, return a nil pointer.
func configGetStringPtr(d *schema.ResourceData, attrName schemaAttr) *string {
	if s, ok := schemaGetStringOK(d, attrName); ok {
		return &s
	}

	return nil
}

func indirect(v interface{}) interface{} {
	switch v.(type) {
	case string:
		return v
	case *string:
		p := v.(*string)
		if p == nil {
			return nil
		}
		return *p
	default:
		return v
	}
}

func suppressEquivalentTimeDurations(k, old, new string, d *schema.ResourceData) bool {
	d1, err := time.ParseDuration(old)
	if err != nil {
		return false
	}

	d2, err := time.ParseDuration(new)
	if err != nil {
		return false
	}

	return d1 == d2
}
