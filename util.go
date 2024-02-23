package confusing

import (
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"unicode"
)

var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

// converts camelCase to dot.notation
func camelToSnake(input string, ensureLowercase bool) string {
	input = matchFirstCap.ReplaceAllString(input, "${1}_${2}")
	input = matchAllCap.ReplaceAllString(input, "${1}_${2}")

	if ensureLowercase {
		input = strings.ToLower(input)
	}

	return input
}

func ucfirst(s string) string {
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])

	return string(r)
}

func lcfirst(s string) string {
	r := []rune(s)
	r[0] = unicode.ToLower(r[0])

	return string(r)
}

func concatenateKeys(keys ...string) string {
	return strings.Join(keys, ".")
}

func parseBool(val string) (bool, error) {
	switch strings.ToLower(val) {
	case "1", "yes", "on":
		return true, nil
	case "0", "no", "off":
		return false, nil
	}

	return false, InvalidBooleanError
}

func parseBoolOrDefault(val string, defaultValue bool) bool {
	boolValue, err := parseBool(val)

	if err != nil {
		return defaultValue
	}

	return boolValue
}

func processStructField(field reflect.StructField) string {
	key := field.Tag.Get("config")

	if key == "-" {
		return ""
	}

	if field.Anonymous {
		t := field.Type

		if t.Kind() == reflect.Ptr {
			t = t.Elem()
		}

		// How can we read an anonymous embedded primitive?
		// Maybe we can use the name of the type as the field name in the future
		if t.Kind() != reflect.Struct {
			return ""
		}
	} else if !field.IsExported() {
		return ""
	}

	if key == "" {
		key = field.Name
	}

	return key
}

func stringOrDefault(key string, defaultValue string) string {
	if key == "" {
		return defaultValue
	}

	return key
}

func inferSourceTypeFromFilePath(filePath string) SourceType {
	var typ SourceType
	var ok bool

	if strings.HasPrefix(filePath, ".env") {
		typ = sourceTypeByExt[".env"]
	} else {
		ext := filepath.Ext(filePath)
		typ, ok = sourceTypeByExt[ext]

		if !ok {
			typ = ext
		}
	}

	_, ok = sources[typ]

	if !ok {
		typ = ""
	}

	return typ
}
