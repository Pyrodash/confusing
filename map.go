package confusing

import (
	"encoding/json"
	"errors"
	"gopkg.in/yaml.v3"
	"os"
	"reflect"
	"strings"
)

const (
	YAMLSourceType SourceType = "yaml"
	JSONSourceType            = "json"
)

// YAML and JSON sources are always attempted first because they are the most specific

type MapSource struct {
	typ        SourceType
	data       map[string]interface{}
	normalizer KeyNormalizer
}

type callbackFunc func()

type mapQueueItem struct {
	source   reflect.Value
	target   reflect.Value
	callback callbackFunc
}

func (i *mapQueueItem) complete() {
	if i.callback != nil {
		i.callback()
	}
}

func (s *MapSource) getKeyFromMap(rootMap map[string]interface{}, key string) interface{} {
	var value interface{}

	value = rootMap

	if key == "" {
		return value
	}

	key = s.normalizer.Normalize(key)
	parts := strings.Split(key, ".")

	for _, part := range parts {
		if m, ok := value.(map[string]interface{}); ok {
			value = m[part]
		} else {
			return nil
		}
	}

	return value
}

func (s *MapSource) readMapPrimitive(rootSourceValue reflect.Value, rootTargetValue reflect.Value) error {
	queue := []mapQueueItem{{source: rootSourceValue, target: rootTargetValue}}

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		if item.source.Kind() == reflect.Invalid {
			continue
		}

		var targetType reflect.Type
		sourceType := item.source.Type()

		for {
			targetType = item.target.Elem().Type()

			if targetType.Kind() != reflect.Ptr {
				break
			} else {
				elemType := targetType.Elem()
				elemPtr := reflect.New(elemType)

				item.target.Elem().Set(elemPtr)
				item.target = elemPtr
			}
		}

		if sourceType.ConvertibleTo(targetType) {
			item.target.Elem().Set(item.source.Convert(targetType))
			item.complete()

			continue
		}

		switch targetType.Kind() {
		case reflect.Bool: // YAML doesn't parse booleans by default
			switch sourceType.Kind() {
			case reflect.String:
				valueBool, err := parseBool(item.source.String())

				if err != nil {
					// invalid boolean value
					// do nothing
					continue
				}

				item.target.Elem().SetBool(valueBool)
			case reflect.Float64:
				item.target.Elem().SetBool(item.source.Float() > 0)
			default:
				// source type can't be converted to bool
				// do nothing
				continue
			}
		case reflect.Slice:
			if sourceType.Kind() == reflect.Slice {
				sourceValueLen := item.source.Len()
				newSlice := reflect.MakeSlice(targetType, sourceValueLen, sourceValueLen)

				for i := 0; i < sourceValueLen; i++ {
					queue = append(
						queue,
						mapQueueItem{
							source: item.source.Index(i).Elem(),
							target: newSlice.Index(i).Addr(),
						},
					)
				}

				item.target.Elem().Set(newSlice)
			} else {
				continue
			}
		case reflect.Map:
			if item.source.Type().Kind() == reflect.Map {
				keyType := targetType.Key()
				valueType := targetType.Elem()

				iter := item.source.MapRange()
				newMap := reflect.MakeMap(targetType)

				for iter.Next() {
					k := iter.Key()
					v := iter.Value()

					keyInitialized := false

					newKeyPtr := reflect.New(keyType)
					newValuePtr := reflect.New(valueType)

					queue = append(queue, mapQueueItem{
						source: k,
						target: newKeyPtr,
						callback: func() {
							keyInitialized = true
						},
					})

					queue = append(queue, mapQueueItem{
						source: reflect.ValueOf(v.Interface()),
						target: newValuePtr,
						callback: func() {
							if keyInitialized {
								newMap.SetMapIndex(newKeyPtr.Elem(), newValuePtr.Elem())
							}
						},
					})
				}

				item.target.Elem().Set(newMap)
			} else {
				continue
			}
		case reflect.Struct:
			if m, ok := item.source.Interface().(map[string]interface{}); ok {
				reader, isReader := item.target.Interface().(Reader)

				if isReader {
					err := reader.ReadConfig(&MapSource{typ: s.typ, data: m})

					if err != nil {
						// errors from custom readers always break execution
						// if you want your custom reader to remain fault-tolerant, do not return the errors you get from calling the configSource
						return err
					}
				} else {
					for i := 0; i < targetType.NumField(); i++ {
						field := targetType.Field(i)
						childKey := processStructField(field)

						if childKey == "" {
							continue
						}

						childSourceValue := s.getKeyFromMap(m, childKey)

						if childSourceValue != nil {
							childValue := reflect.ValueOf(childSourceValue)

							queue = append(queue, mapQueueItem{
								source: childValue,
								target: item.target.Elem().Field(i).Addr(),
							})
						}
					}
				}
			} else {
				continue
			}
		default:
			// unsupported data type
			continue
		}

		item.complete()
	}

	return nil
}

func (s *MapSource) ReadKey(key string, target interface{}) error {
	targetValue := reflect.ValueOf(target)

	if targetValue.Kind() != reflect.Ptr || targetValue.IsNil() {
		return errors.New("target must be a non-nil pointer")
	}

	val := s.getKeyFromMap(s.data, key)
	sourceValue := reflect.ValueOf(val)

	return s.readMapPrimitive(sourceValue, targetValue)
}

func (s *MapSource) Read(target interface{}) error {
	targetValue := reflect.ValueOf(target)

	if targetValue.Kind() != reflect.Ptr || targetValue.IsNil() {
		return errors.New("target must be a non-nil pointer")
	}

	targetType := targetValue.Elem().Type()

	if targetType.Kind() != reflect.Struct {
		return errors.New("target must be a struct")
	}

	return s.readMapPrimitive(reflect.ValueOf(s.data), targetValue)
}

func (s *MapSource) Type() SourceType {
	return s.typ
}

func NewYAMLSource(data map[string]interface{}, convention string) (*MapSource, error) {
	normalizer, err := NormalizerForSourceType(convention, YAMLSourceType)

	if err != nil {
		return nil, err
	}

	return &MapSource{
		typ:        YAMLSourceType,
		data:       data,
		normalizer: normalizer,
	}, nil
}

func BuildYAMLSource(opts SourceOptions) (Source, error) {
	if len(opts.FilePath) == 0 {
		opts.FilePath = "config.yaml"
	}

	file, err := os.Open(opts.FilePath)

	if err != nil {
		return nil, err
	}

	defer file.Close()

	var data map[string]interface{}

	d := yaml.NewDecoder(file)
	err = d.Decode(&data)

	if err != nil {
		return nil, err
	}

	return NewYAMLSource(data, opts.Convention)
}

func NewJSONSource(data map[string]interface{}, convention string) (*MapSource, error) {
	normalizer, err := NormalizerForSourceType(convention, JSONSourceType)

	if err != nil {
		return nil, err
	}

	return &MapSource{
		typ:        JSONSourceType,
		data:       data,
		normalizer: normalizer,
	}, nil
}

func BuildJSONSource(opts SourceOptions) (Source, error) {
	if len(opts.FilePath) == 0 {
		opts.FilePath = "config.json"
	}

	file, err := os.Open(opts.FilePath)

	if err != nil {
		return nil, err
	}

	defer file.Close()

	var data map[string]interface{}

	d := json.NewDecoder(file)
	err = d.Decode(&data)

	if err != nil {
		return nil, err
	}

	return NewJSONSource(data, opts.Convention)
}
