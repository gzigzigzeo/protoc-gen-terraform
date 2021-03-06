/*
Copyright 2015-2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package config contains global configuration variables and methods to parse them
package config

import (
	"io/ioutil"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

var (
	// Types is the list of top level types to export. This list must be explicit.
	//
	// Passed from command line (--terraform_out=types=types.UserV2:./_out)
	Types map[string]struct{} = make(map[string]struct{})

	// ExcludeFields is the list of fields to ignore.
	//
	// Passed from command line (--terraform_out=excludeFields=types.UserV2.Expires:./_out)
	ExcludeFields map[string]struct{} = make(map[string]struct{})

	// DurationCustomType this type name will be treated as a custom extendee of time.Duration
	DurationCustomType = ""

	// DefaultPackageName default package name, gets appended to type name if its import
	// path is ".", but the type itself is located in another package
	DefaultPackageName string

	// CustomImports adds imports required in target file
	CustomImports []string

	// TargetPackageName sets the name of the target package
	TargetPackageName string

	// ComputedFields is the list of fields to mark as 'Computed: true'
	//
	// Passed from command line (--terraform_out=computed=types.UserV2.Kind:./_out)
	ComputedFields map[string]struct{} = make(map[string]struct{})

	// RequiredFields is the list of fields to mark as 'Required: true'
	//
	// Passed from command line (--terraform_out=required=types.Metadata.Name:./_out)
	RequiredFields map[string]struct{} = make(map[string]struct{})

	// ConfigModeAttrFields is the list of fields to mark as 'Required: true'
	//
	// Passed from command line (--terraform_out=config_mode_attr=types.Metadata.Name:./_out)
	ConfigModeAttrFields map[string]struct{} = make(map[string]struct{})

	// ConfigModeBlockFields is the list of fields to mark as 'Required: true'
	//
	// Passed from command line (--terraform_out=config_mode_block=types.Metadata.Name:./_out)
	ConfigModeBlockFields map[string]struct{} = make(map[string]struct{})

	// Defaults is the map of default values for a fields
	//
	// Can be set in config file only
	Defaults map[string]interface{} = make(map[string]interface{})

	// ForceNewFields is the list of fields to mark as 'Required: true'
	//
	// Passed from command line (--terraform_out=force=types.Metadata.Name:./_out)
	ForceNewFields map[string]struct{} = make(map[string]struct{})

	// Suffixes represents map of suffixes for custom types
	Suffixes map[string]string = make(map[string]string)

	// StateFunc represents map of StateFunc values for a fields
	StateFunc map[string]string = make(map[string]string)

	// config is yaml config unmarshal struct
	cfg config
)

const (
	paramDelimiter = "+" // Delimiter for types and ignoreFields
)

// config is yaml config unmarshalling helper struct
type config struct {
	Types                 []string               `yaml:"types,omitempty"`
	DurationCustomType    string                 `yaml:"duration_custom_type,omitempty"`
	TargetPackageName     string                 `yaml:"target_package_name,omitempty"`
	DefaultPackageName    string                 `yaml:"default_package_name,omitempty"`
	ExcludeFields         []string               `yaml:"exclude_fields,omitempty"`
	ComputedFields        []string               `yaml:"computed_fields,omitempty"`
	RequiredFields        []string               `yaml:"required_fields,omitempty"`
	ForceNew              []string               `yaml:"force_new_fields,omitempty"`
	ConfigModeAttrFields  []string               `yaml:"config_mode_attr_fields,omitempty"`
	ConfigModeBlockFields []string               `yaml:"config_mode_block_fields,omitempty"`
	CustomImports         []string               `yaml:"custom_imports,omitempty"`
	Defaults              map[string]interface{} `yaml:"defaults,omitempty"`
	Suffixes              map[string]string      `yaml:"suffixes,omitempty"`
	StateFunc             map[string]string      `yaml:"state_func,omitempty"`
}

// Read reads config variables from command line or config file
func Read(p map[string]string) error {
	c := trimArg(p["config"])

	if c != "" {
		err := readConfigFromYaml(c)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		err := setTypes(splitArg(p["types"]))
		if err != nil {
			return trace.Wrap(err)
		}
	}

	setExcludeFields(splitArg(p["exclude_fields"]))
	setComputedFields(splitArg(p["computed"]))
	setRequiredFields(splitArg(p["required"]))
	setCustomImports(splitArg(p["custom_imports"]))
	setForceNewFields(splitArg(p["force"]))
	setConfigModeAttrFields(splitArg(p["config_mode_attr"]))
	setConfigModeBlockFields(splitArg(p["config_mode_block"]))

	setDefaultPackageName(p["pkg"])
	setDurationType(p["custom_duration"])
	setTargetPackageName(p["target_pkg"])

	return nil
}

// readConfigFromYaml reads config from YAML file if specified
func readConfigFromYaml(p string) error {
	if p == "" {
		return nil
	}

	c, err := ioutil.ReadFile(p)
	if err != nil {
		return trace.Wrap(err)
	}

	err = yaml.Unmarshal(c, &cfg)
	if err != nil {
		return trace.Wrap(err)
	}

	return setVarsFromConfig()
}

// setVarsFromConfig sets config variables from parsed yaml
func setVarsFromConfig() error {
	err := setTypes(cfg.Types)
	if err != nil {
		return trace.Wrap(err)
	}

	setDurationType(cfg.DurationCustomType)
	setTargetPackageName(cfg.TargetPackageName)
	setDefaultPackageName(cfg.DefaultPackageName)
	setExcludeFields(cfg.ExcludeFields)
	setComputedFields(cfg.ComputedFields)
	setRequiredFields(cfg.RequiredFields)
	setConfigModeAttrFields(cfg.ConfigModeAttrFields)
	setConfigModeBlockFields(cfg.ConfigModeBlockFields)
	setCustomImports(cfg.CustomImports)
	setForceNewFields(cfg.ForceNew)
	setDefaults(cfg.Defaults)
	setSuffixes(cfg.Suffixes)
	setStateFunc(cfg.StateFunc)

	return nil
}

// setTypes sets Types variable from a string slice, returns error if slice is empty
func setTypes(t []string) error {
	if len(t) == 0 {
		return trace.Errorf("Please, specify explicit top level type list, e.g. --terraform-out=types=UserV2+UserSpecV2:./_out")
	}

	setSet(Types, t)

	log.Printf("Types: %s", t)

	return nil
}

// setExcludeFields parses and sets ExcludeFields
func setExcludeFields(f []string) {
	setSet(ExcludeFields, f)

	if len(f) > 0 {
		log.Printf("Excluded fields: %s", f)
	}
}

// setDefaultPackageName sets the default package name
func setDefaultPackageName(arg string) {
	if trimArg(arg) == "" {
		return
	}

	_, name := filepath.Split(arg)
	DefaultPackageName = name

	log.Printf("Default package name: %v", DefaultPackageName)
}

// setDurationType sets the custom duration type
func setDurationType(arg string) {
	if trimArg(arg) == "" {
		return
	}

	DurationCustomType = arg

	log.Printf("Duration custom type: %s", DurationCustomType)
}

// setCustomImports parses custom import packages
func setCustomImports(i []string) {
	if len(i) == 0 {
		return
	}

	CustomImports = i

	log.Printf("Custom imports: %s", CustomImports)
}

// setTargetPackageName sets the target package name
func setTargetPackageName(arg string) {
	if trimArg(arg) == "" {
		return
	}

	_, name := filepath.Split(arg)
	TargetPackageName = name

	log.Printf("Target package name: %v", TargetPackageName)
}

// setComputedFields parses and sets ExcludeFields
func setComputedFields(f []string) {
	setSet(ComputedFields, f)

	if len(f) > 0 {
		log.Printf("Computed fields: %s", f)
	}
}

// setRequiredFields parses and sets ExcludeFields
func setRequiredFields(f []string) {
	setSet(RequiredFields, f)

	if len(f) > 0 {
		log.Printf("Required fields: %s", f)
	}
}

// setDefaults sets default values for a fields
func setDefaults(m map[string]interface{}) {
	if len(m) == 0 {
		return
	}

	Defaults = m

	log.Printf("Defaults set for: %v", reflect.ValueOf(m).MapKeys())
}

// setSuffixes sets suffixes for a fields
func setSuffixes(m map[string]string) {
	if len(m) == 0 {
		return
	}

	Suffixes = m

	log.Printf("Suffixes set for: %v", reflect.ValueOf(m).MapKeys())
}

// setStateFunc sets suffixes for a fields
func setStateFunc(m map[string]string) {
	if len(m) == 0 {
		return
	}

	StateFunc = m

	log.Printf("State funcs set for: %v", reflect.ValueOf(m).MapKeys())
}

// setForceNew parses and sets ExcludeFields
func setForceNewFields(f []string) {
	setSet(ForceNewFields, f)

	if len(f) > 0 {
		log.Printf("Force new fields: %s", f)
	}
}

// setConfigModeAttrFields parses and sets ExcludeFields
func setConfigModeAttrFields(f []string) {
	setSet(ConfigModeAttrFields, f)

	if len(f) > 0 {
		log.Printf("SchemaConfigModeAttr fields: %s", f)
	}
}

// setConfigModeBlockFields parses and sets ExcludeFields
func setConfigModeBlockFields(f []string) {
	setSet(ConfigModeBlockFields, f)

	if len(f) > 0 {
		log.Printf("SchemaConfigModeBlock fields: %s", f)
	}
}

// trimArg returns argument value without spaces and line breaks
func trimArg(s string) string {
	return strings.TrimSpace(s)
}

// splitArg splits array arg by delimiter
func splitArg(arg string) []string {
	a := trimArg(arg)

	// Prevents returning slice with an empty single element
	if a == "" {
		return []string{}
	}

	return strings.Split(a, paramDelimiter)
}

// setSet sets map[string]struct{} set from a keys array
func setSet(s map[string]struct{}, a []string) {
	for _, n := range a {
		s[n] = struct{}{}
	}
}
