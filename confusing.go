package confusing

import (
	"errors"
	"os"
	"reflect"
	"strings"
)

var (
	InvalidBooleanError = errors.New("invalid boolean value")
	readerType          = reflect.TypeOf((*Reader)(nil)).Elem()
)

var sources = map[SourceType]SourceBuilder{
	EnvSourceType:  BuildEnvSource,
	YAMLSourceType: BuildYAMLSource,
	JSONSourceType: BuildJSONSource,
}

var sourceTypeByExt = map[string]SourceType{
	".yaml": YAMLSourceType,
	".yml":  YAMLSourceType,
	".json": JSONSourceType,
	".env":  EnvSourceType,
}

// User-registered sources are always attempted before pre-existing sources (hence why they are reversed)
// EnvSource is always attempted last because it always succeeds (unless a .env file is explicitly specified and fails to be read)
var reverseOrderedSources = []string{"env", "json", "yaml"}

type Reader interface {
	ReadConfig(source Source) error
}

func RegisterSource(typ string, builder SourceBuilder) {
	sources[typ] = builder
	sourceTypeByExt["."+typ] = typ
	reverseOrderedSources = append(reverseOrderedSources, typ)
}

type Options struct {
	SourceOptions SourceOptions
	SourceType    SourceType
}

func NewSource(optsSlice ...Options) (Source, error) {
	sourceOptions := SourceOptions{
		FilePath:   os.Getenv("CONFIG_PATH"),
		Convention: os.Getenv("CONFIG_CONVENTION"),
	}

	sourceType := strings.ToLower(os.Getenv("CONFIG_TYPE"))
	subsetSources := reverseOrderedSources

	if len(optsSlice) > 0 {
		sourceOptions.FilePath = stringOrDefault(sourceOptions.FilePath, optsSlice[0].SourceOptions.FilePath)
		sourceOptions.Convention = stringOrDefault(sourceOptions.Convention, optsSlice[0].SourceOptions.Convention)
		sourceType = stringOrDefault(sourceType, optsSlice[0].SourceType)
	}

	if len(sourceType) > 0 {
		subsetSources = []SourceType{sourceType}
	} else if len(sourceOptions.FilePath) > 0 {
		inferredType := inferSourceTypeFromFilePath(sourceOptions.FilePath)

		if len(inferredType) > 0 {
			subsetSources = []SourceType{inferredType}
		}
	}

	var source Source
	var err error

	for i := len(subsetSources) - 1; i >= 0; i-- {
		typ := subsetSources[i]
		builder := sources[typ]
		source, err = builder(sourceOptions)

		if err == nil {
			return source, nil
		}
	}

	return nil, err
}
