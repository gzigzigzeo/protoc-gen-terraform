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

package plugin

import (
	"github.com/gogo/protobuf/protoc-gen-gogo/generator"
	"github.com/gravitational/protoc-gen-terraform/config"
	"github.com/gravitational/trace"
	"github.com/stoewer/go-strcase"
)

var (
	cache map[string]*Message = make(map[string]*Message)
)

// Message represents metadata about protobuf message
type Message struct {
	// Name contains type name
	Name string

	// NameSnake contains type name in snake case (Terraform schema field name)
	NameSnake string

	// GoTypeName contains Go type name for this message with package name
	GoTypeName string

	// Fields contains the collection of fields
	Fields []*Field
}

// BuildMessage builds Message from its protobuf descriptor.
//
// checkValiditiy should be false for nested messages. We do not check them over allowed types,
// otherwise it will be overexplicit. Use excludeFields to skip fields.
//
// It might return nil, nil which means that operation was successful, but message should be skipped.
func BuildMessage(g *generator.Generator, d *generator.Descriptor, checkValidity bool) (*Message, error) {
	typeName := getMessageTypeName(d)

	// Check if message is specified in export type list
	_, ok := config.Types[typeName]
	if !ok && checkValidity {
		// This is not an error, we must just skip this message
		return nil, nil
	}

	c, ok := cache[typeName]
	if ok {
		return c, nil
	}

	for _, field := range d.GetField() {
		if field.OneofIndex != nil {
			return nil, newInvalidMessageError(typeName, "oneOf messages are not supported yet")
		}
	}

	name := d.GetName()

	message := &Message{
		Name:       name,
		NameSnake:  strcase.SnakeCase(name),
		GoTypeName: typeName,
	}

	err := BuildFields(message, g, d)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return message, nil
}

// getMessageTypeName returns full message name, with prepended DefaultPkgName if needed
func getMessageTypeName(d *generator.Descriptor) string {
	if d.GoImportPath() != "." {
		return d.File().GetPackage() + "." + d.GetName()
	}
	if config.DefaultPackageName != "" {
		return config.DefaultPackageName + "." + d.GetName()
	}
	return d.GetName()
}