package flagutil

import (
	"encoding/json"
	"flag"
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
	"strings"
)

var (
	configFile  = flag.String("config_file.path", "", "reads configuration from config file and overrides flags and env values")
	printConfig = flag.String("config_file.print_format", "", "prints formatted config to stdout and exits")
)

// PrintConfig prints
//  default config
// with formatter defined at flag
// --config_file_print_format
// exits with 0 after printing
func PrintConfig() error {
	format := *printConfig
	switch format {
	case "":
		return nil
	case "json", "yaml", "yml":
	default:
		return fmt.Errorf("unknown output format: %s", format)
	}
	flagMap := map[string]interface{}{}
	flag.VisitAll(func(f *flag.Flag) {
		splitFlagName := strings.Split(f.Name, ".")
		// special hack
		// rule is deprecated
		// use rule.path instead
		if f.Name == "rule" {
			splitFlagName = []string{"rule", "path"}
		}
		// promscrape.config is bad also
		// use promscrape.config.path instead
		if f.Name == "promscrape.config" {
			splitFlagName = []string{"promscrape", "config", "path"}
		}
		currentMapRoot := flagMap
		for i := 0; i < len(splitFlagName); i++ {
			v := splitFlagName[i]
			if currentMapRoot[v] == nil {
				if i == len(splitFlagName)-1 {
					currentMapRoot[v] = f.DefValue
				} else {
					currentMapRoot[v] = map[string]interface{}{}
					currentMapRoot = currentMapRoot[v].(map[string]interface{})
				}
			} else {
				switch currentMapRoot[v].(type) {
				case map[string]interface{}:
					currentMapRoot = currentMapRoot[v].(map[string]interface{})
				default:
					log.Fatalf("bad flag value, it overlaps with existing path: %v\n", f.Name)
				}
			}

		}

	})
	var configContent []byte
	var err error
	if format == "json" {
		configContent, err = json.Marshal(&flagMap)
		if err != nil {
			return fmt.Errorf("cannot marshal example config to json: %v", err)
		}
	} else {
		configContent, err = yaml.Marshal(&flagMap)
		if err != nil {
			return fmt.Errorf("cannot marshal config to yaml: %v", err)
		}
	}
	fmt.Printf("\n%s\n", string(configContent))
	os.Exit(0)
	return nil
}

// ParseConfig implements
// reading flags from config file
// it supports json and yaml formats
// parser doesnt validate provided config
// and only looking for matched flag names
// for instance flag -http.listen.port
// can be readed from json config: {"http": {"listen" :  {"port" : 8085 }}}
// array values are combined to single value
func ParseConfig() error {
	if *configFile == "" {
		return nil
	}

	return parseConfig(*configFile)
}

func parseConfig(configPath string) error {

	configContent := map[string]interface{}{}
	err := readFileContentToMap(*configFile, &configContent)
	if err != nil {
		return fmt.Errorf("cannot read config file from file: %s, err: %w", *configFile, err)
	}
	flattenConfig := map[string]string{}
	for k, v := range configContent {
		flatten(k, v, flattenConfig)
	}

	//special hack for rule
	if v, ok := flattenConfig["rule.path"]; ok && v != "" {
		flattenConfig["rule"] = v
	}
	// also we have to apply this fix for promscrape.config
	if v, ok := flattenConfig["promsrapce.config.path"]; ok && v != "" {
		flattenConfig["promscrape.config"] = v
	}
	flag.VisitAll(func(f *flag.Flag) {

		if v, ok := flattenConfig[f.Name]; ok {
			if v != "" {
				err = f.Value.Set(v)
				if err != nil {
					fmt.Printf("cannot set flag: %s, err: %v \n", f.Name, err)
				}
			}
		}
	})
	return nil
}

// parse map to flatten
// all array keys
// will be combined to single value
func flatten(prefix string, value interface{}, flatmap map[string]string) {

	switch value := value.(type) {
	case map[interface{}]interface{}:
		for k, v := range value {
			flatten(prefix+"."+k.(string), v, flatmap)
		}

	case []interface{}:
		for _, v := range value {
			flatten(prefix, v, flatmap)
		}

	case []string:
		for _, v := range value {
			if _, ok := flatmap[prefix]; ok {
				flatmap[prefix] += "," + v
			} else {
				flatmap[prefix] = v
			}

		}

	default:
		// if we already have such prefix
		// it means, that its an array
		// and we should combine its values
		flagValue := fmt.Sprintf("%v", value)
		if v, ok := flatmap[prefix]; ok {
			flatmap[prefix] = v + "," + flagValue
		} else {
			flatmap[prefix] = flagValue
		}
	}
}

// reads file, parses it with predefined parsers
// json and yaml is supported
func readFileContentToMap(filePath string, contentMap *map[string]interface{}) error {
	if strings.HasSuffix(filePath, ".yaml") || strings.HasSuffix(filePath, ".yml") {
		fileContent, err := ioutil.ReadFile(filePath)
		if err != nil {
			return err
		}
		return yaml.Unmarshal(fileContent, contentMap)
	}
	if strings.HasSuffix(filePath, ".json") {
		fileContent, err := ioutil.ReadFile(filePath)
		if err != nil {
			return err
		}
		return json.Unmarshal(fileContent, contentMap)
	}

	return fmt.Errorf("file type is not supported, provide yaml or json")
}
