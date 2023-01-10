package differ

import (
	"encoding/json"
	"reflect"

	"github.com/hashicorp/terraform/internal/command/jsonformat/computed"
	"github.com/hashicorp/terraform/internal/command/jsonformat/differ/replace"
	"github.com/hashicorp/terraform/internal/command/jsonplan"
	"github.com/hashicorp/terraform/internal/plans"
)

// Change contains the unmarshalled generic interface{} types that are output by
// the JSON functions in the various json packages (such as jsonplan and
// jsonprovider).
//
// A Change can be converted into a computed.Diff, ready for rendering, with the
// ComputeDiffForAttribute, ComputeDiffForOutput, and ComputeDiffForBlock
// functions.
//
// The Before and After fields are actually go-cty values, but we cannot convert
// them directly because of the Terraform Cloud redacted endpoint. The redacted
// endpoint turns sensitive values into strings regardless of their types.
// Because of this, we cannot just do a direct conversion using the ctyjson
// package. We would have to iterate through the schema first, find the
// sensitive values and their mapped types, update the types inside the schema
// to strings, and then go back and do the overall conversion. This isn't
// including any of the more complicated parts around what happens if something
// was sensitive before and isn't sensitive after or vice versa. This would mean
// the type would need to change between the before and after value. It is in
// fact just easier to iterate through the values as generic JSON interfaces.
type Change struct {

	// BeforeExplicit matches AfterExplicit except references the Before value.
	BeforeExplicit bool

	// AfterExplicit refers to whether the After value is explicit or
	// implicit. It is explicit if it has been specified by the user, and
	// implicit if it has been set as a consequence of other changes.
	//
	// For example, explicitly setting a value to null in a list should result
	// in After being null and AfterExplicit being true. In comparison,
	// removing an element from a list should also result in After being null
	// and AfterExplicit being false. Without the explicit information our
	// functions would not be able to tell the difference between these two
	// cases.
	AfterExplicit bool

	// Before contains the value before the proposed change.
	//
	// The type of the value should be informed by the schema and cast
	// appropriately when needed.
	Before interface{}

	// After contains the value after the proposed change.
	//
	// The type of the value should be informed by the schema and cast
	// appropriately when needed.
	After interface{}

	// Unknown describes whether the After value is known or unknown at the time
	// of the plan. In practice, this means the after value should be rendered
	// simply as `(known after apply)`.
	//
	// The concrete value could be a boolean describing whether the entirety of
	// the After value is unknown, or it could be a list or a map depending on
	// the schema describing whether specific elements or attributes within the
	// value are unknown.
	Unknown interface{}

	// BeforeSensitive matches Unknown, but references whether the Before value
	// is sensitive.
	BeforeSensitive interface{}

	// AfterSensitive matches Unknown, but references whether the After value is
	// sensitive.
	AfterSensitive interface{}

	// ReplacePaths generally contains nested slices that describe paths to
	// elements or attributes that are causing the overall resource to be
	// replaced.
	ReplacePaths replace.ForcesReplacement
}

// ValueFromJsonChange unmarshals the raw []byte values in the jsonplan.Change
// structs into generic interface{} types that can be reasoned about.
func ValueFromJsonChange(change jsonplan.Change) Change {
	return Change{
		Before:          unmarshalGeneric(change.Before),
		After:           unmarshalGeneric(change.After),
		Unknown:         unmarshalGeneric(change.AfterUnknown),
		BeforeSensitive: unmarshalGeneric(change.BeforeSensitive),
		AfterSensitive:  unmarshalGeneric(change.AfterSensitive),
		ReplacePaths:    replace.Parse(change.ReplacePaths),
	}
}

func (change Change) asDiff(renderer computed.DiffRenderer) computed.Diff {
	return computed.NewDiff(renderer, change.calculateChange(), change.ReplacePaths.ForcesReplacement())
}

func (change Change) calculateChange() plans.Action {
	if (change.Before == nil && !change.BeforeExplicit) && (change.After != nil || change.AfterExplicit) {
		return plans.Create
	}
	if (change.After == nil && !change.AfterExplicit) && (change.Before != nil || change.BeforeExplicit) {
		return plans.Delete
	}

	if reflect.DeepEqual(change.Before, change.After) && change.AfterExplicit == change.BeforeExplicit && change.isAfterSensitive() == change.isBeforeSensitive() {
		return plans.NoOp
	}

	return plans.Update
}

// getDefaultActionForIteration is used to guess what the change could be for
// complex attributes (collections and objects) and blocks.
//
// You can't really tell the difference between a NoOp and an Update just by
// looking at the attribute itself as you need to inspect the children.
//
// This function returns a Delete or a Create action if the before or after
// values were null, and returns a NoOp for all other cases. It should be used
// in conjunction with compareActions to calculate the actual action based on
// the actions of the children.
func (change Change) getDefaultActionForIteration() plans.Action {
	if change.Before == nil && change.After == nil {
		return plans.NoOp
	}

	if change.Before == nil {
		return plans.Create
	}
	if change.After == nil {
		return plans.Delete
	}
	return plans.NoOp
}

// compareActions will compare current and next, and return plans.Update if they
// are different, and current if they are the same.
//
// This function should be used in conjunction with getDefaultActionForIteration
// to convert a NoOp default action into an Update based on the actions of a
// values children.
func compareActions(current, next plans.Action) plans.Action {
	if next == plans.NoOp {
		return current
	}

	if current != next {
		return plans.Update
	}
	return current
}

func unmarshalGeneric(raw json.RawMessage) interface{} {
	if raw == nil {
		return nil
	}

	var out interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		panic("unrecognized json type: " + err.Error())
	}
	return out
}
