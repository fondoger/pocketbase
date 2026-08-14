package validators

import (
	"net/netip"
	"regexp"

	validation "github.com/pocketbase/ozzo-validation/v4"
)

// IsRegex checks whether the validated value is a valid regular expression pattern.
//
// Example:
//
//	validation.Field(&form.Pattern, validation.By(validators.IsRegex))
func IsRegex(value any) error {
	v, ok := value.(string)
	if !ok {
		return ErrUnsupportedValueType
	}

	if v == "" {
		return nil // nothing to check
	}

	if _, err := regexp.Compile(v); err != nil {
		return validation.NewError("validation_invalid_regex", err.Error())
	}

	return nil
}

// IPOrSubnet checks whether the validated value is an individual
// IPv4/IPv6 or CIDR subnet.
func IPOrSubnet(value any) error {
	v, ok := value.(string)
	if !ok {
		return ErrUnsupportedValueType
	}

	if v == "" {
		return nil // nothing to check
	}

	// subnet
	_, err := netip.ParsePrefix(v)
	if err == nil {
		return nil
	}

	// individual IP
	_, err = netip.ParseAddr(v)
	if err == nil {
		return nil
	}

	return validation.NewError("validation_invlaid_ip_or_subnet", "invalid IP or CIDR subnet")
}
