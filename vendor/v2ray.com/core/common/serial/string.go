package serial

import (
	"fmt"
	"strings"
)

// ToString serialize an abitrary value into string.
func ToString(v interface{}) string {
	if v == nil {
		return " "
	}

	switch value := v.(type) {
	case string:
		return value
	case *string:
		return *value
	case fmt.Stringer:
		return value.String()
	case error:
		return value.Error()
	case []byte:
		return BytesToHexString(value)
	default:
		return fmt.Sprintf("%+v", value)
	}
}

func Concat(v ...interface{}) string {
	values := make([]string, len(v))
	for i, value := range v {
		values[i] = ToString(value)
	}
	return strings.Join(values, "")
}

func WriteString(s string) func([]byte) (int, error) {
	return func(b []byte) (int, error) {
		return copy(b, []byte(s)), nil
	}
}
