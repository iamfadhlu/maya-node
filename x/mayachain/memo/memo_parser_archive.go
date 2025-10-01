package mayachain

import (
	"fmt"
	"strings"

	"gitlab.com/mayachain/mayanode/common/cosmos"
)

func (p *parser) getUintArrayBySeparatorV1(idx int, required bool, separator string, def, max cosmos.Uint) []cosmos.Uint {
	p.incRequired(required)
	value := p.get(idx)
	if value == "" {
		return []cosmos.Uint{}
	}
	strArray := strings.Split(value, separator)
	result := make([]cosmos.Uint, 0, len(strArray))
	for _, str := range strArray {
		u, err := cosmos.ParseUint(str)
		if err != nil {
			if required {
				p.addErr(fmt.Errorf("cannot parse '%s' as an uint: %w", str, err))
				return []cosmos.Uint{}
			} else if str == "" {
				u = def
			}
		}
		if !u.Equal(def) && !max.IsZero() && u.GT(max) {
			p.addErr(fmt.Errorf("uint value %s is greater than max value %s", u, max))
		}
		result = append(result, u)
	}
	return result
}
