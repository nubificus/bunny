// Copyright (c) 2023-2025, Nubificus LTD
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

package hops

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-version"
)

const (
	Version = "v0.1"
)

// CheckBunnyfileVersion checks if the version of the user's input file
// is compatible with the supported version.
func CheckBunnyfileVersion(fileVersion string) error {
	if fileVersion == "" {
		return fmt.Errorf("The version field is necessary")
	}
	hVersion, err := version.NewVersion(Version)
	if err != nil {
		return fmt.Errorf("Internal error parsing hops API version %s: %v", Version, err)
	}
	userFileVer, err := version.NewVersion(fileVersion)
	if err != nil {
		return fmt.Errorf("Could not parse version in user bunnyfile %s: %v", fileVersion, err)
	}
	if hVersion.LessThan(userFileVer) {
		return fmt.Errorf("Unsupported version %s. Please use %s or earlier", fileVersion, Version)
	}

	return nil
}

// ValidatePlatform checks if user input meets all conditions regarding the platforms
// field. The conditions are:
// 1) framework can not be empty or not set
// 2) monitor can not be empty or not set
func ValidatePlatform(plat Platform) error {
	if plat.Framework == "" {
		return fmt.Errorf("The framework field of platforms is necessary")
	}
	if plat.Monitor == "" {
		return fmt.Errorf("The monitor field of platforms is necessary")
	}

	return nil
}

// ValidateRootfs checks if user input meets all conditions regarding the rootfs
// field. The conditions are:
// 1) if from is empty/scratch then path should also be empty
// 2) if path is empty then from should also be empty
// 3) if from is not scratch or empty, include should not be set
// 4) An entry in include can not have the first part (before ":" empty
func ValidateRootfs(rootfs Rootfs) error {
	if (rootfs.From == "scratch") && rootfs.Path != "" {
		return fmt.Errorf("The from field of rootfs can not be empty or scratch, if path is set")
	}
	if rootfs.Path != "" && rootfs.Type == "raw" {
		return fmt.Errorf("The path field in rootfs can not be combined with a raw rootfs")
	}
	if rootfs.From == "local" && rootfs.Type == "raw" {
		return fmt.Errorf("If type of rootfs is raw, then from can not be local")
	}
	if len(rootfs.Includes) > 0 && rootfs.From != "scratch" {
		return fmt.Errorf("Adding files to an existing rootfs is not yet supported")
	}

	for _, file := range rootfs.Includes {
		parts := strings.Split(file, ":")
		if len(parts) < 1 || len(parts[0]) == 0 {
			return fmt.Errorf("Invalid syntax in rootf's include. An entry can not have its first part empty")
		}
	}

	return nil
}

// ValidateKernel checks if user input meets all conditions regarding the kernel
// field. The conditions are:
// 1) from can not be empty or not set
// 2) path not be empty or not set
func ValidateKernel(kernel Kernel) error {
	if kernel.From == "" {
		return fmt.Errorf("The from field of kernel is necessary")
	}
	if kernel.Path == "" {
		return fmt.Errorf("The path field of kernel is necessary")
	}

	return nil
}
