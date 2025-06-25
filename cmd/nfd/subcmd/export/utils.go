/*
Copyright 2025 The Kubernetes Authors.

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

package export

import (
	"fmt"
	"os"
)

var (
	outputPath string
)

// writeToFile saves string content to a file at the path set by path
func writeToFile(path, content string) (err error) {
	fd, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		closeErr := fd.Close()

		// Note the err is the named return value, and defer wraps end
		if err == nil {
			err = closeErr
		}
	}()

	_, err = fmt.Fprint(fd, content)
	return err
}
