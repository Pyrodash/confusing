package confusing

import "fmt"

type SourceType = string

type Source interface {
	Type() SourceType
	Read(target interface{}) error
	ReadKey(key string, target interface{}) error
}

type SourceOptions struct {
	FilePath   string
	Convention string
}

type SourceBuilder = func(opts SourceOptions) (Source, error)

type PrefixedSource struct {
	source Source
	prefix string
}

func (s *PrefixedSource) Read(target interface{}) error {
	return s.source.ReadKey(s.prefix, target)
}

func (s *PrefixedSource) ReadKey(key string, target interface{}) error {
	return s.source.ReadKey(fmt.Sprintf("%s.%s", s.prefix, key), target)
}

func (s *PrefixedSource) Type() SourceType {
	return s.source.Type()
}

func PrefixSourceWith(prefix string, source Source) Source {
	return &PrefixedSource{
		source: source,
		prefix: prefix,
	}
}
