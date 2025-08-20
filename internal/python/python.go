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

// Package python provides utilities for executing python code.
package python

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

// Execute executes the provided python code with the given input.
// The input is passed to the python code as a JSON string.
// The python code is expected to be a function that takes a single
// JSON string as an argument and returns a JSON string.
func Execute(ctx context.Context, code string, input any) (any, error) {
	inBytes, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input: %w", err)
	}

	cmd := exec.CommandContext(ctx, "python3", "-c", code, string(inBytes))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to run python code: %w\nstderr: %s", err, stderr.String())
	}

	var out any
	if stdout.Bytes() == nil {
		return nil, nil
	}
	return stdout.String(), nil

	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		return nil, fmt.Errorf("failed to unmarshal python output: %w", err)
	}

	return out, nil
}
