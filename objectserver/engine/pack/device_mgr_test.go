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

package pack

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iqiyi/falcon/common/ring"
)

func TestNewPackDeviceMgr(t *testing.T) {
	root, err := ioutil.TempDir("", "")
	require.Nil(t, err)
	defer os.RemoveAll(root)

	// Any way to get rid of the magic number?
	mgr := NewPackDeviceMgr(6000, root, PACK_POLICY_INDEX)
	mgr.testMode = true
	require.NotNil(t, mgr)

	devs, err := ring.ListLocalDevices(
		"object", mgr.hashPrefix, mgr.hashSuffix, mgr.Policy, mgr.Port)

	require.Nil(t, err)
	require.Equal(t, len(devs), len(mgr.devices))
}

func TestGetPackDevice1(t *testing.T) {
	root, err := ioutil.TempDir("", "")
	require.Nil(t, err)
	defer os.RemoveAll(root)

	// Any way to get rid of the magic number?
	mgr := NewPackDeviceMgr(6000, root, PACK_POLICY_INDEX)
	mgr.testMode = true
	require.NotNil(t, mgr)

	require.NotNil(t, mgr.GetPackDevice(PACK_DEVICE))
}

func TestGetPackDevice2(t *testing.T) {
	root, err := ioutil.TempDir("", "")
	require.Nil(t, err)
	defer os.RemoveAll(root)

	// Any way to get rid of the magic number?
	mgr := NewPackDeviceMgr(6000, root, PACK_POLICY_INDEX)
	mgr.testMode = false
	require.NotNil(t, mgr)

	require.Nil(t, mgr.GetPackDevice(PACK_DEVICE))
}
