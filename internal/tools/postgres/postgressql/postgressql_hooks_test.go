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

package postgressql

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pashagolub/pgxmock/v2"
	"github.com/googleapis/genai-toolbox/internal/tools"
)

func TestTool_Invoke_WithHooks(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close()

	tool := Tool{
		BeforeTool: `
import sys
import json

def main(input_str):
    data = json.loads(input_str)
    data["name"] = "modified-" + data["name"]
    print(json.dumps(data))

if __name__ == "__main__":
    main(sys.argv[1])
`,
		AfterTool: `
import sys
import json

def main(input_str):
    data = json.loads(input_str)
    for row in data:
        row["id"] = row["id"] + 1
    print(json.dumps(data))

if __name__ == "__main__":
    main(sys.argv[1])
`,
		Parameters: []tools.Parameter{
			tools.NewStringParameter("name", "some description"),
		},
		Statement: "SELECT id, name FROM my_table WHERE name = $1",
		Pool:      mock,
	}

	params := tools.ParamValues{
		{Name: "name", Value: "test"},
	}

	rows := pgxmock.NewRows([]string{"id", "name"}).AddRow(1, "modified-test")
	mock.ExpectQuery("SELECT id, name FROM my_table WHERE name = \\$1").WithArgs("modified-test").WillReturnRows(rows)

	got, err := tool.Invoke(context.Background(), params)
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}

	want := []any{map[string]any{"id": float64(2), "name": "modified-test"}}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Invoke() got = %v, want %v, diff = %v", got, want, diff)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}


