# confusing

confusing is a Go library that provides a unified interface for parsing configurations from a variety of different formats such as: environment variables, YAML or JSON.

## Installation
```shell
go get github.com/Pyrodash/confusing
```

## Sources
A `Source` is an object with a defined interface for reading values stored at a specific keys in the config, and parsing them into the expected type.

Three sources are provided out of the box: `"env"`, `"yaml"`, and `"json"`.

It's possible to explicitly specify a config source type, and/or config file path, by setting any of the following environment variables:
```
CONFIG_PATH="path/to/config"
CONFIG_TYPE="json"
```

### Env
By default, the EnvSource will attempt to load the `.env` file into the environment. If the file does not exist, no error will be returned (unless a config path is explicitly defined), because using a .env file is optional.

#### Arrays
When using an EnvSource, arrays are read as a list of comma-separated items. However, when an array of structs or slices is encountered, the whole array will be parsed as a JSON string.

## Keys
Configurations are indexed by keys which use the dot notation as a universal standard for nested object access. Each source is responsible for translating a key to the standard key-naming convention of the target format.

### Env
The enforced key-naming convention is `UPPER_SNAKE_CASE`.

For example, to read `"Hello world"` from this environment variable:
```
CONFUSING_WELCOME_MESSAGE="Hello world"
```

We can do the following:
```go
var myString string

source.ReadKey("confusing.welcomeMessage", &myString) // source is an env source
fmt.Println(myString) // "Hello world"
```

### JSON
The enforced key-naming convention is `camelCase`.

For example, the same code displayed above can be used to read `"Hello world"` this JSON config:
```json
{
  "confusing": {
    "welcomeMessage": "Hello world"
  }
}
```

### YAML
The enforced key-naming convention is `snake_case`.

Likewise, the example code displayed previously can be used to read `"Hello world"` from the following YAML config:
```yaml
confusing:
  welcome_message: Hello world
```


It's possible to override the enforced convention for YAML/JSON by setting the following environment variables:
```
JSON_CONVENTION="snake" # or camel
YAML_CONVENTION="camel" # or snake

# to set a global convention (not recommended)
CONFIG_CONVENTION="snake"
```
Note that these should be set manually in the terminal (or through Docker/Kubernetes) because they will not be automatically read from a `.env` file, since you are not using one. You can use a `.env` file to set these options if you load it into the environment yourself.

## Acquiring a Source
A factory function is provided to create a source of any type. It iterates over all possible source types, attempting to locate the source whose configuration file exists. If there are no config files found, the default source is an EnvSource (even if there is no `.env` file).
```go
source, err := confusing.NewSource()

if err != nil {
	// handle error
}

fmt.Println(source.Type())
```
It's also possible to pass some options to the factory function. All of them are optional, and if there are env variable counterparts, they are prioritized.
```go
source, err := confusing.NewSource(confusing.Options{
	SourceType: "json",
	SourceOptions: confusing.SourceOptions{
		FilePath: "path/to/config.json",
		Convention: confusing.SnakeCaseConvention,
	},
}
})
```
Note that when you specify a convention this way, it's going to be enforced upon whatever type of source is selected. There's a better way to programmatically change the default convention for a specific source type:
```go
confusing.SetConventionForSourceType(confusing.JSONSourceType, confusing.SnakeCaseConvention)

source, err := confusing.NewSource(confusing.Options{
    SourceOptions: confusing.SourceOptions{
        FilePath: "path/to/config.json", // source type will be automatically inferred to json
    },
})
```

## Reading Configurations
Example:

```go
// main.go
package main

import (
	"fmt"
	"github.com/Pyrodash/confusing"
)

type DatabaseConfig struct {
	Host     string
	Port     int
	Username string
	Database string `config:"name"`
}

type OAuth2Provider struct {
	Key    string
	Secret string
}

type MyConfig struct {
	Port            int
	Database        DatabaseConfig
	OAuth2Providers []OAuth2Provider `config:"oauth2"`
}

func main() {
	source, err := confusing.NewSource()

	if err != nil {
		panic(err)
	}

	var myConfig MyConfig

	err = source.Read(&myConfig)

	if err != nil {
		panic(err)
	}

	fmt.Printf("%+v\n", myConfig)
}

```
```yaml
# config.yaml
port: 3000
database:
  host: 127.0.0.1
  port: 3306
  username: jackie
  name: confusing
oauth2:
  - key: discord
    secret: some_secret
  - key: facebook
    secret: some_secret
```
The same code can also read the following `.env` file:
```
PORT=3000

DATABASE_HOST="127.0.0.1"
DATABASE_PORT="3306"
DATABASE_USERNAME="jackie"
DATABASE_NAME="confusing"

OAUTH2='[{"key":"discord","secret":"some_secret"},{"key":"facebook","secret":"some_secret"}]'
```