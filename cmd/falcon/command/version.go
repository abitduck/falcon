// Copyright (c) 2016-2018 iQIYI.com.  All rights reserved.
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
// 

package command

import (
	"fmt"
	"strings"

	"github.com/iqiyi/falcon/common"
)

type VersionCommand struct {
}

func (c *VersionCommand) Help() string {
	helpText := `
Usage: falcon version 
`
	return strings.TrimSpace(helpText)
}

func (c *VersionCommand) Run(args []string) int {
	fmt.Println(common.Version)
	return EXIT_OK
}

func (c *VersionCommand) Synopsis() string {
	return "show the version information"
}
