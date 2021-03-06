// Copyright (C) 2017 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gapidapk

import (
	"github.com/golang/protobuf/jsonpb"
	"github.com/google/gapid/core/fault/cause"
	"github.com/google/gapid/core/log"
	"github.com/google/gapid/core/os/android"
	"github.com/google/gapid/core/os/android/adb"
	"github.com/google/gapid/gapidapk/pkginfo"
)

const (
	sendPkgInfoAction                = "com.google.android.gapid.action.SEND_PKG_INFO"
	sendPkgInfoService               = "com.google.android.gapid.PackageInfoService"
	sendPkgInfoOnlyDebugExtra        = "com.google.android.gapid.extra.ONLY_DEBUG"
	sendPkgInfoIncludeIconsExtra     = "com.google.android.gapid.extra.INCLUDE_ICONS"
	sendPkgInfoIconDensityScaleExtra = "com.google.android.gapid.extra.ICON_DENSITY_SCALE"
	sendPkgInfoPort                  = "gapid-pkginfo"
)

// PackageList returns the list of packages installed on the device.
func PackageList(ctx log.Context, d adb.Device, includeIcons bool, iconDensityScale float32) (*pkginfo.PackageList, error) {
	apk, err := EnsureInstalled(ctx, d, nil)
	if err != nil {
		return nil, err
	}

	action := apk.ServiceActions.FindByName(sendPkgInfoAction, sendPkgInfoService)
	if action == nil {
		return nil, cause.Explain(ctx, nil, "Service intent was not found")
	}

	onlyDebug := d.Root(ctx) == adb.ErrDeviceNotRooted

	if err := d.StartService(ctx, *action,
		android.BoolExtra{Key: sendPkgInfoOnlyDebugExtra, Value: onlyDebug},
		android.BoolExtra{Key: sendPkgInfoIncludeIconsExtra, Value: includeIcons},
		android.FloatExtra{Key: sendPkgInfoIconDensityScaleExtra, Value: iconDensityScale},
	); err != nil {
		return nil, cause.Explain(ctx, err, "Starting service")
	}

	sock, err := adb.ForwardAndConnect(ctx, d, sendPkgInfoPort)
	if err != nil {
		return nil, cause.Explain(ctx, err, "Connecting to service port")
	}

	defer sock.Close()

	out := &pkginfo.PackageList{}
	if err := jsonpb.Unmarshal(sock, out); err != nil {
		return nil, cause.Explain(ctx, err, "unmarshal json data")
	}
	return out, nil
}
