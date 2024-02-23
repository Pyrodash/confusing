package confusing

import (
	"fmt"
	"os"
	"strings"
)

const (
	SnakeCaseConvention      = "snake"
	CamelCaseConvention      = "camel"
	UpperSnakeCaseConvention = "upper_snake"
)

var normalizers = map[string]KeyNormalizer{
	SnakeCaseConvention:      &SnakeCaseNormalizer{},
	CamelCaseConvention:      &CamelCaseNormalizer{},
	UpperSnakeCaseConvention: &UpperSnakeCaseNormalizer{},
}

var sourceConventions = map[string]string{
	EnvSourceType:  UpperSnakeCaseConvention,
	YAMLSourceType: SnakeCaseConvention,
	JSONSourceType: CamelCaseConvention,
}

type UnknownConventionError struct {
	Convention string
}

func (u UnknownConventionError) Error() string {
	return fmt.Sprintf("unknown convention: %s", u.Convention)
}

type KeyNormalizer interface {
	Normalize(key string) string
}

type SnakeCaseNormalizer struct{}

func (n *SnakeCaseNormalizer) Normalize(key string) string {
	return camelToSnake(key, true)
}

type UpperSnakeCaseNormalizer struct{}

// Normalize todo: Optimize this process
func (n *UpperSnakeCaseNormalizer) Normalize(key string) string {
	parts := strings.Split(key, ".")

	for i, part := range parts {
		parts[i] = strings.ToUpper(camelToSnake(part, false))
	}

	key = strings.Join(parts, "_")

	return key
}

type CamelCaseNormalizer struct{}

func (n *CamelCaseNormalizer) Normalize(key string) string {
	parts := strings.Split(key, ".")

	for i, part := range parts {
		parts[i] = lcfirst(part)
	}

	return strings.Join(parts, ".")
}

func SetConventionForSourceType(sourceType SourceType, convention string) {
	sourceConventions[sourceType] = convention
}

func GetNormalizer(convention string) KeyNormalizer {
	return normalizers[convention]
}

func GetConventionForSourceType(sourceType SourceType) string {
	convention := sourceConventions[sourceType]
	conventionEnvVar := fmt.Sprintf("%s_CONVENTION", strings.ToUpper(sourceType))

	return stringOrDefault(os.Getenv(conventionEnvVar), convention)
}

func NormalizerForSourceType(convention string, sourceType SourceType) (KeyNormalizer, error) {
	var normalizer KeyNormalizer

	if len(convention) > 0 {
		normalizer = GetNormalizer(convention)
	} else {
		convention = GetConventionForSourceType(sourceType)
		normalizer = GetNormalizer(convention)
	}

	if normalizer == nil {
		return nil, UnknownConventionError{Convention: convention}
	}

	return normalizer, nil
}
