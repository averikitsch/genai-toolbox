// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"slices"

	yaml "github.com/goccy/go-yaml"
	"github.com/googleapis/genai-toolbox/internal/sources"
	python "github.com/tliron/py4go"
)

// ToolConfigFactory defines the signature for a function that creates and
// decodes a specific tool's configuration. It takes the context, the tool's
// name, and a YAML decoder to parse the config.
type ToolConfigFactory func(ctx context.Context, name string, decoder *yaml.Decoder) (ToolConfig, error)

var toolRegistry = make(map[string]ToolConfigFactory)

// Register allows individual tool packages to register their configuration
// factory function. This is typically called from an init() function in the
// tool's package. It associates a 'kind' string with a function that can
// produce the specific ToolConfig type. It returns true if the registration was
// successful, and false if a tool with the same kind was already registered.
func Register(kind string, factory ToolConfigFactory) bool {
	if _, exists := toolRegistry[kind]; exists {
		// Tool with this kind already exists, do not overwrite.
		return false
	}
	toolRegistry[kind] = factory
	return true
}

// DecodeConfig looks up the registered factory for the given kind and uses it
// to decode the tool configuration.
func DecodeConfig(ctx context.Context, kind string, name string, decoder *yaml.Decoder) (ToolConfig, error) {
	factory, found := toolRegistry[kind]
	if !found {
		return nil, fmt.Errorf("unknown tool kind: %q", kind)
	}
	toolConfig, err := factory(ctx, name, decoder)
	if err != nil {
		return nil, fmt.Errorf("unable to parse tool %q as kind %q: %w", name, kind, err)
	}
	return toolConfig, nil
}

type ToolConfig interface {
	ToolConfigKind() string
	Initialize(map[string]sources.Source) (Tool, error)
}

type Tool interface {
	Invoke(context.Context, ParamValues) (any, error)
	ParseParams(map[string]any, map[string]map[string]any) (ParamValues, error)
	Manifest() Manifest
	McpManifest() McpManifest
	Authorized([]string) bool
	InvokeBeforeTool(context.Context, ParamValues) (ParamValues, error)
}

// Manifest is the representation of tools sent to Client SDKs.
type Manifest struct {
	Description  string              `json:"description"`
	Parameters   []ParameterManifest `json:"parameters"`
	AuthRequired []string            `json:"authRequired"`
}

// Definition for a tool the MCP client can call.
type McpManifest struct {
	// The name of the tool.
	Name string `json:"name"`
	// A human-readable description of the tool.
	Description string `json:"description,omitempty"`
	// A JSON Schema object defining the expected parameters for the tool.
	InputSchema McpToolsSchema `json:"inputSchema,omitempty"`
}

// Helper function that returns if a tool invocation request is authorized
func IsAuthorized(authRequiredSources []string, verifiedAuthServices []string) bool {
	if len(authRequiredSources) == 0 {
		// no authorization requirement
		return true
	}
	for _, a := range authRequiredSources {
		if slices.Contains(verifiedAuthServices, a) {
			return true
		}
	}
	return false
}

// Execute executes the provided python code with the given input.
// The input is passed to the python code as a JSON string.
// The python code is expected to be a function that takes a single
// JSON string as an argument and returns a JSON string.
func Execute(ctx context.Context, pythonCode string, input ParamValues) (ParamValues, error) {
	// code := `
	// def preprocess(tool_params: dict) -> dict:
	// 	print("Hello from before_tool")
	// 	for key, value in tool_params.items():
	// 		tool_params[key] = value.upper()
	// 	print(tool_params)
	// 	return tool_params
	// 	`

	// Initialize Python runtime
	python.Initialize()
	// defer python.Finalize()

	// Import the module containing your preprocess function
	// Assuming it's in a file called "mymodule.py"
	// python.SetPythonPath("/home/user/genai-toolbox/internal/")
	module, err := python.Import("mymodule")
	if err != nil {
		panic(err)
	}
	// defer module.Release()
	// Execute the Python code to define the function
	// _, err := python.RunString(code)
	// _, err := python.NewUnicode(code)
	// if err != nil {
	// 	panic(err)
	// }

	// Get the main module to access the defined function
	// module, err := python.Import("__main__")
	// if err != nil {
	// 	panic(err)
	// }
	// defer module.Release()

	// Get the preprocess function
	preprocessFunc, err := module.GetAttr("preprocess")
	if err != nil {
		panic(err)
	}
	defer preprocessFunc.Release()

	// Create the dictionary input {hello: world, bye: moon}
	inputDict, err := python.NewDict()
	if err != nil {
		panic(err)
	}
	defer inputDict.Release()

	// Add key-value pairs to the dictionary
	helloKey, _ := python.NewUnicode("name")
	defer helloKey.Release()
	worldValue, _ := python.NewUnicode("Running")
	defer worldValue.Release()
	inputDict.SetDictItem(helloKey, worldValue)

	byeKey, _ := python.NewUnicode("bye")
	defer byeKey.Release()
	moonValue, _ := python.NewUnicode("moon")
	defer moonValue.Release()
	inputDict.SetDictItem(byeKey, moonValue)

	// Call the preprocess function with the dictionary
	result, err := preprocessFunc.Call(inputDict)
	if err != nil {
		panic(err)
	}
	defer result.Release()
	// fmt.Printf(strconv.FormatBool(result.IsDict()))
	// Process the result (example: convert to string)
	// resultStr := result.String()
	// fmt.Printf("Function result: %s\n", resultStr)
	params := make([]ParamValue, 0, 2)
	if result.IsDict() {
		// jsonData, _ := result.ToBytes()
		jsonData := []byte("{\"name\": \"Running\"}")
		var resultMap map[string]interface{}

		err := json.Unmarshal(jsonData, &resultMap)
		if err != nil {
			log.Fatalf("Error unmarshaling JSON: %v", err)
		}

		for name, newV := range resultMap {
			params = append(params, ParamValue{Name: name, Value: newV})
		}
	}

	return params, nil
}
