package confusing

import (
	"encoding/json"
	"errors"
	"github.com/joho/godotenv"
	"os"
	"reflect"
	"strconv"
	"strings"
)

const EnvSourceType SourceType = "env"

type EnvSource struct {
	normalizer KeyNormalizer
}

type envQueueItem struct {
	key    string
	target reflect.Value
}

func (s *EnvSource) readEnvKey(key string) string {
	key = s.normalizer.Normalize(key)

	return os.Getenv(key)
}

// NOTE: Maps and slices of structs/slices don't make sense in environment variables
// Maps are always parsed as JSON strings
// By default, slices are parsed as comma-separated items
// When a slice of structs/slices is encountered, the whole slice is parsed as a JSON string
func (s *EnvSource) readEnvPrimitive(value string, targetValue reflect.Value) error {
	targetType := targetValue.Elem().Type()

	switch targetType.Kind() {
	case reflect.String:
		targetValue.Elem().SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		valueInt, err := strconv.Atoi(value)

		if err != nil {
			return err
		}

		targetValue.Elem().SetInt(int64(valueInt))
	case reflect.Float32, reflect.Float64:
		valueFloat, err := strconv.ParseFloat(value, 64)

		if err != nil {
			return err
		}

		targetValue.Elem().SetFloat(valueFloat)
	case reflect.Bool:
		valueBool, err := parseBool(value)

		if err != nil {
			return err
		}

		targetValue.Elem().SetBool(valueBool)
	case reflect.Map:
		var data map[interface{}]interface{}
		var source *MapSource
		err := json.Unmarshal([]byte(value), &data)

		if err != nil {
			return err
		}

		source, err = NewJSONSource(nil, "")

		if err != nil {
			return err
		}

		return source.readMapPrimitive(reflect.ValueOf(data), targetValue)
	default:
		return errors.New("unsupported target type")
	}

	return nil
}

func (s *EnvSource) readEnvSlice(value string, targetPtr reflect.Value) error {
	sliceType := targetPtr.Elem().Type()

	if len(value) > 0 {
		switch sliceType.Elem().Kind() {
		case reflect.Struct:
			fallthrough
		case reflect.Slice:
			var data []interface{}
			var source *MapSource
			err := json.Unmarshal([]byte(value), &data)

			if err != nil {
				return err
			}

			source, err = NewJSONSource(nil, "")

			if err != nil {
				return err
			}

			return source.readMapPrimitive(reflect.ValueOf(data), targetPtr)
		default:
			valueSlice := strings.Split(value, ",")
			valueSliceLen := len(valueSlice)

			newSlice := reflect.MakeSlice(sliceType, valueSliceLen, valueSliceLen)

			for i := range valueSlice {
				valueSlice[i] = strings.TrimSpace(valueSlice[i])

				elemType := sliceType.Elem()
				elemPtr := reflect.New(elemType)

				if err := s.readEnvPrimitive(valueSlice[i], elemPtr); err != nil {
					elemPtr.Elem().SetZero()
				}

				newSlice.Index(i).Set(elemPtr.Elem())
			}

			targetPtr.Elem().Set(newSlice)
		}
	} else {
		newSlice := reflect.MakeSlice(sliceType, 0, 0)

		targetPtr.Elem().Set(newSlice)
	}

	return nil
}

func (s *EnvSource) readKey(rootKey string, rootTargetValue reflect.Value) error {
	queue := []envQueueItem{{rootKey, rootTargetValue}}

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		targetElemType := item.target.Elem().Type()
		targetPtr := item.target

		// deference target ptr to the deepest pointer which is pointing at an actual value when the target is a multi-level pointer
		for {
			if targetElemType.Kind() != reflect.Ptr {
				break
			} else {
				targetElemType = targetElemType.Elem()
				elemPtr := reflect.New(targetElemType)

				targetPtr.Elem().Set(elemPtr)
				targetPtr = elemPtr
			}
		}

		switch targetElemType.Kind() {
		case reflect.Slice:
			value := strings.TrimSpace(s.readEnvKey(item.key))

			if err := s.readEnvSlice(value, targetPtr); err != nil {
				return err
			}
		case reflect.Struct:
			reader, isReader := targetPtr.Interface().(Reader)

			if isReader {
				err := reader.ReadConfig(PrefixSourceWith(item.key, s))

				if err != nil {
					// errors from custom readers always break execution
					// if you want your custom reader to remain fault-tolerant, do not return the errors you get from calling the configSource
					return err
				}
			} else {
				for i := 0; i < targetElemType.NumField(); i++ {
					field := targetElemType.Field(i)
					childKey := processStructField(field)

					if childKey == "" {
						continue
					}

					var absoluteKey string

					if len(item.key) > 0 {
						absoluteKey = concatenateKeys(item.key, childKey)
					} else {
						absoluteKey = childKey
					}

					queue = append(queue, envQueueItem{absoluteKey, targetPtr.Elem().Field(i).Addr()})
				}
			}
		default:
			if err := s.readEnvPrimitive(s.readEnvKey(item.key), targetPtr); err != nil {
				// targetPtr.Elem().SetZero()
				continue
			}
		}
	}

	return nil
}

func (s *EnvSource) ReadKey(key string, target interface{}) error {
	targetValue := reflect.ValueOf(target)

	if targetValue.Kind() != reflect.Ptr || targetValue.IsNil() {
		return errors.New("target must be a non-nil pointer")
	}

	return s.readKey(key, targetValue)
}

func (s *EnvSource) Read(target interface{}) error {
	targetValue := reflect.ValueOf(target)

	if targetValue.Kind() != reflect.Ptr || targetValue.IsNil() {
		return errors.New("target must be a non-nil pointer")
	}

	targetType := targetValue.Elem().Type()

	if targetType.Kind() != reflect.Struct {
		return errors.New("target must be a struct")
	}

	return s.readKey("", targetValue)
}

func (s *EnvSource) Type() string {
	return EnvSourceType
}

func NewEnvSource(convention string) (*EnvSource, error) {
	normalizer, err := NormalizerForSourceType(convention, EnvSourceType)

	if err != nil {
		return nil, err
	}

	return &EnvSource{normalizer: normalizer}, nil
}

// BuildEnvSource This function only fails if the .env file path is explicitly provided and doesn't exist
func BuildEnvSource(opts SourceOptions) (Source, error) {
	var err error
	var verifyPath bool

	if len(opts.FilePath) > 0 {
		err = godotenv.Load(opts.FilePath)
		verifyPath = true
	} else {
		err = godotenv.Load()
		verifyPath = false
	}

	isFileExistsErr := errors.Is(err, os.ErrNotExist)

	if err != nil {
		if (isFileExistsErr && verifyPath) || !isFileExistsErr {
			return nil, err
		}
	}

	return NewEnvSource(opts.Convention)
}
