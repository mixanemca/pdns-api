/*
Copyright © 2021 Michael Bruskov <mixanemca@yandex.ru>

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

package infrastructure

import "os"

func GetHostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}

// Canonicalize returns canonicalized string
func Canonicalize(name string) string {
	if name != "" && name[len(name)-1:] != "." {
		return name + "."
	}
	return name
}

// DeCanonicalize returns not canonicalized string
func DeCanonicalize(name string) string {
	if name != "" && name[len(name)-1:] == "." {
		return name[:len(name)-1]
	}
	return name
}
