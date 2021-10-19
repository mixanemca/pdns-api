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

package forwardzone

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/mixanemca/pdns-api/internal/infrastructure/errors"
	"github.com/mixanemca/pdns-api/internal/infrastructure/network"
)

var re = regexp.MustCompile(`([a-zA-Z0-9.-]+)\s?=\s?([a-zA-Z0-9.:, ]+)`)

// ForwardZone represent a zones and it nameservers from forward-zones-file
type ForwardZone struct {
	Name        string   `json:"name"`
	Nameservers []string `json:"nameservers"`
}

// ForwardZones represent list of zones and its nameservers from forward-zones-file
type ForwardZones []ForwardZone

// Inplenet sort.Interface
func (fzs ForwardZones) Len() int           { return len(fzs) }
func (fzs ForwardZones) Less(i, j int) bool { return fzs[i].Name < fzs[j].Name }
func (fzs ForwardZones) Swap(i, j int)      { fzs[i], fzs[j] = fzs[j], fzs[i] }

// String implements fmt.Stringer interface
func (fz ForwardZone) String() string {
	return fmt.Sprintf("%s=%s\n", network.Canonicalize(fz.Name), strings.Join(fz.Nameservers, ","))
}

// ParseForwardZoneLine parse string to ForwardZone
func ParseForwardZoneLine(s string) (*ForwardZone, error) {
	fz := new(ForwardZone)
	match := re.FindStringSubmatch(s)
	if len(match) != 3 {
		return nil, fmt.Errorf("failed parse forward-zones-line: %s", s)
	}
	fz.Name = match[1]
	fz.Nameservers = strings.Split(match[2], ",")
	// Remove spaces
	for i := range fz.Nameservers {
		fz.Nameservers[i] = strings.TrimSpace(fz.Nameservers[i])
	}

	return fz, nil
}

func ParseForwardZoneFile(r io.Reader) (ForwardZones, error) {
	fzs := make(ForwardZones, 0)
	scanner := bufio.NewScanner(r)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		s := scanner.Text()
		fz, err := ParseForwardZoneLine(s)
		if err != nil {
			return fzs, err
		}
		if fz != nil {
			fzs = append(fzs, *fz)
		}
	}
	if err := scanner.Err(); err != nil {
		return fzs, fmt.Errorf("failed to parse forward-zones-file: %v", err)
	}

	return fzs, nil
}

func ForwardZoneIsExist(fzs ForwardZones, searchName string) bool {
	sort.Sort(fzs)
	idx := sort.Search(len(fzs), func(i int) bool { return fzs[i].Name == network.Canonicalize(searchName) })
	return idx < len(fzs) && fzs[idx].Name == network.Canonicalize(searchName)
}

func UpdateForwardZone(fzs ForwardZones, fz ForwardZone) (ForwardZones, error) {
	for i := range fzs {
		if fzs[i].Name == network.Canonicalize(fz.Name) {
			fzs[i] = fz
			return fzs, nil
		}
	}
	return fzs, errors.Newf("forward-zone %s not found", fz.Name)
}

func DeleteForwardZone(fzs []ForwardZone, deleteName string) ([]ForwardZone, error) {
	for i := range fzs {
		if fzs[i].Name == network.Canonicalize(deleteName) {
			fzs = append(fzs[:i], fzs[i+1:]...)
			return fzs, nil
		}
	}
	return nil, fmt.Errorf("forwarding zone %s not found", deleteName)
}
