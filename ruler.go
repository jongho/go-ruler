package ruler

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// we'll use these values
// to avoid passing strings to our
// special comparison func for these comparators
const (
	eq        = iota
	neq       = iota
	gt        = iota
	gte       = iota
	lt        = iota
	lte       = iota
	exists    = iota
	nexists   = iota
	regex     = iota
	matches   = iota
	contains  = iota
	ncontains = iota
)

// Ruler holds an array of Rules
type Ruler struct {
	rules []*Rule
}

// NewRuler creates a new Ruler for you
// optionally accepts a pointer to a slice of filters
// if you have filters that you want to start with
func NewRuler(rules []*Rule) *Ruler {
	if rules != nil {
		return &Ruler{
			rules,
		}
	}

	return &Ruler{}
}

// NewRulerWithJSON returns a new ruler with filters parsed from JSON data
// expects JSON as a slice of bytes and will parse your JSON for you!
func NewRulerWithJSON(jsonstr []byte) (*Ruler, error) {
	var rules []*Rule

	err := json.Unmarshal(jsonstr, &rules)
	if err != nil {
		return nil, err
	}

	return NewRuler(rules), nil
}

// Rule adds a new rule for the property at `path`
// returns a RulerFilter that you can use to add conditions
// and more filters
func (r *Ruler) Rule(path string) *RulerRule {
	rule := &Rule{
		"",
		path,
		nil,
	}

	r.rules = append(r.rules, rule)

	return &RulerRule{
		r,
		rule,
	}
}

// Test tests all the rules (i.e. filters) in your set of rules,
// given a map that looks like a JSON object
// (map[string]interface{})
func (r *Ruler) Test(o map[string]interface{}) (bool, error) {
	for _, f := range r.rules {
		val := pluck(o, f.Path)

		if val != nil {
			// both the actual and expected value must be comparable
			a := reflect.TypeOf(val)
			e := reflect.TypeOf(f.Value)

			if !a.Comparable() || !e.Comparable() {
				return false, nil
			}

			result, err := r.compare(f, val)
			if err != nil {
				return false, err
			}
			if !result {
				return false, nil
			}
		} else if val == nil && (f.Comparator == "exists" || f.Comparator == "nexists") {
			// either one of these can be done
			return r.compare(f, val)
		} else {
			// if we couldn't find the value on the map
			// and the comparator isn't exists/nexists, this fails
			return false, fmt.Errorf("did not find property (%s) on map", f.Path)
		}

	}

	return true, nil
}

// compares real v. actual values
func (r *Ruler) compare(f *Rule, actual interface{}) (bool, error) {
	expected := f.Value
	switch f.Comparator {
	case "eq":
		return actual == expected, nil

	case "neq":
		return actual != expected, nil

	case "gt":
		return r.inequality(gt, actual, expected)

	case "gte":
		return r.inequality(gte, actual, expected)

	case "lt":
		return r.inequality(lt, actual, expected)

	case "lte":
		return r.inequality(lte, actual, expected)

	case "exists":
		// not sure this makes complete sense
		if actual != nil {
			return true, nil
		}
		return false, nil
	case "nexists":
		if actual == nil {
			return true, nil
		}
		return false, nil

	case "regex":
		fallthrough
	case "contains":
		fallthrough
	case "matches":
		return r.regexp(actual, expected)

	case "ncontains":
		result, err := r.regexp(actual, expected)
		if err != nil {
			return false, err
		}
		return !result, err

	default:
		//should probably return an error or something
		//but this is good for now
		//if comparator is not implemented, return false
		return false, errors.New("unknown comparator %s")
	}
}

// runs equality comparison
// separated in a different function because
// we need to do another type assertion here
// and some other acrobatics
func (r *Ruler) inequality(op int, actual, expected interface{}) (bool, error) {

	if reflect.TypeOf(actual) != reflect.TypeOf(expected) {
		return false, errors.New("Value types are mismatched, cannot compare values")
	}

	t := reflect.TypeOf(actual).String()
	switch t {
	case "uint8":
		return compareUint(op, actual, expected), nil
	case "uint16":
		return compareUint(op, actual, expected), nil
	case "uint32":
		return compareUint(op, actual, expected), nil
	case "uint64":
		return compareUint(op, actual, expected), nil
	case "uint":
		return compareUint(op, actual, expected), nil
	case "int8":
		return compareInt(op, actual, expected), nil
	case "int16":
		return compareInt(op, actual, expected), nil
	case "int32":
		return compareInt(op, actual, expected), nil
	case "int64":
		return compareInt(op, actual, expected), nil
	case "int":
		return compareInt(op, actual, expected), nil
	case "float32":
		return compareFloat(op, actual, expected), nil
	case "float64":
		return compareFloat(op, actual, expected), nil
	case "string":
		return compareStr(op, actual, expected), nil
	default:
		return false, errors.New("Invalid type for inequality comparison")
	}

}

func (r *Ruler) regexp(actual, expected interface{}) (bool, error) {
	// regexps must be strings
	var streg string
	var ok bool
	if streg, ok = expected.(string); !ok {
		return false, errors.New("expected value not actually a string, bailing")
	}

	var astring string
	if astring, ok = actual.(string); !ok {
		return false, errors.New("actual value not actually a string, bailing")
	}

	reg, err := regexp.Compile(streg)
	if err != nil {
		return false, errors.New("regexp is bad, bailing")
	}

	return reg.MatchString(astring), nil
}

// given a map, pull a property from it at some deeply nested depth
// this re-implements (most of) JS `pluck` in go: https://github.com/gjohnson/pluck
func pluck(o map[string]interface{}, path string) interface{} {
	// support dots for now because thats all we need
	parts := strings.Split(path, ".")

	if len(parts) == 1 && o[parts[0]] != nil {
		// if there is only one part, just return that property value
		return o[parts[0]]
	} else if len(parts) > 1 && o[parts[0]] != nil {
		var prev map[string]interface{}
		var ok bool
		if prev, ok = o[parts[0]].(map[string]interface{}); !ok {
			// not an object type! ...or a map, yeah, that.
			return nil
		}

		for i := 1; i < len(parts)-1; i++ {
			// we need to check the existence of another
			// map[string]interface for every property along the way
			cp := parts[i]

			if prev[cp] == nil {
				// didn't find the property, it's missing
				return nil
			}
			var ok bool
			if prev, ok = prev[cp].(map[string]interface{}); !ok {
				return nil
			}
		}

		if prev[parts[len(parts)-1]] != nil {
			return prev[parts[len(parts)-1]]
		} else {
			return nil
		}
	}

	return nil
}

func compareUint(op int, actual, expected interface{}) bool {

	var cmpUint [2]uint64
	cmpUint[0] = actual.(uint64)
	cmpUint[1] = expected.(uint64)

	switch op {
	case gt:
		return cmpUint[0] > cmpUint[1]
	case gte:
		return cmpUint[0] >= cmpUint[1]
	case lt:
		return cmpUint[0] < cmpUint[1]
	case lte:
		return cmpUint[0] <= cmpUint[1]
	}

	return false
}

func compareInt(op int, actual, expected interface{}) bool {

	var cmpInt [2]int64
	cmpInt[0] = actual.(int64)
	cmpInt[1] = expected.(int64)

	switch op {
	case gt:
		return cmpInt[0] > cmpInt[1]
	case gte:
		return cmpInt[0] >= cmpInt[1]
	case lt:
		return cmpInt[0] < cmpInt[1]
	case lte:
		return cmpInt[0] <= cmpInt[1]
	}

	return false
}

func compareFloat(op int, actual, expected interface{}) bool {

	var cmpFloat [2]float64
	cmpFloat[0] = actual.(float64)
	cmpFloat[1] = expected.(float64)

	switch op {
	case gt:
		return cmpFloat[0] > cmpFloat[1]
	case gte:
		return cmpFloat[0] >= cmpFloat[1]
	case lt:
		return cmpFloat[0] < cmpFloat[1]
	case lte:
		return cmpFloat[0] <= cmpFloat[1]
	}

	return false
}

func compareStr(op int, actual, expected interface{}) bool {

	var cmpStr [2]string
	cmpStr[0] = actual.(string)
	cmpStr[1] = expected.(string)

	actualFloat, err := strconv.ParseFloat(cmpStr[0], 64)
	if err != nil {
		return false
	}
	expectedFloat, err := strconv.ParseFloat(cmpStr[1], 64)
	if err != nil {
		return false
	}

	switch op {
	case gt:
		return actualFloat > expectedFloat
	case gte:
		return actualFloat >= expectedFloat
	case lt:
		return actualFloat < expectedFloat
	case lte:
		return actualFloat <= expectedFloat
	}

	return false
}
