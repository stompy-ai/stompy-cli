package repomarker

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const addressQuoteHint = `quote addresses that start with @, e.g. address: "@handle/name"`

var (
	invalidTokenLine = regexp.MustCompile(`^yaml: line ([1-9][0-9]*): found character that cannot start any token$`)
	unquotedAddress  = regexp.MustCompile(`(?m)^ *(?:- +)?(?:address|"address"|'address') *:[\t ]+@`)
)

func markerDecodeError(data []byte, err error) error {
	message := fmt.Sprintf("%s is not valid: %v (allowed keys: version, environment, projects[address, role, default, purpose])", FileName, err)
	if hasUnquotedAddressAtError(data, err) {
		message += "\nHint: " + addressQuoteHint
	}
	return fmt.Errorf("%s", message)
}

// Bind the hint to the scanner's offending line, never a later address or a
// quoted value/comment containing address-like text. Valid YAML is untouched.
func hasUnquotedAddressAtError(data []byte, err error) bool {
	match := invalidTokenLine.FindStringSubmatch(err.Error())
	if match == nil {
		return false
	}
	line, conversionErr := strconv.Atoi(match[1])
	lines := strings.Split(string(data), "\n")
	if conversionErr != nil || line > len(lines) {
		return false
	}
	return unquotedAddress.MatchString(lines[line-1])
}
