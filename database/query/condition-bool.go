package query

import (
	"errors"
	"fmt"
	"strconv"
)

type boolCondition struct {
	key      string
	operator uint8
	value    bool
}

func newBoolCondition(key string, operator uint8, value interface{}) *boolCondition {

	var parsedValue bool

	switch v := value.(type) {
	case bool:
		parsedValue = v
	case string:
		var err error
		parsedValue, err = strconv.ParseBool(v)
		if err != nil {
			return &boolCondition{
				key:      fmt.Sprintf("could not parse \"%s\" to bool: %s", v, err),
				operator: errorPresent,
			}
		}
	default:
		return &boolCondition{
			key:      fmt.Sprintf("incompatible value %v for int64", value),
			operator: errorPresent,
		}
	}

	return &boolCondition{
		key:      key,
		operator: operator,
		value:    parsedValue,
	}
}

func (c *boolCondition) complies(f Fetcher) bool {
	comp, ok := f.GetBool(c.key)
	if !ok {
		return false
	}

	switch c.operator {
	case Is:
		return comp == c.value
	default:
		return false
	}
}

func (c *boolCondition) check() error {
	if c.operator == errorPresent {
		return errors.New(c.key)
	}
	return nil
}
