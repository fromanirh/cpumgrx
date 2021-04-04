// Copyright 2014 Google Inc. All Rights Reserved.
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

// derived from https://github.com/google/cadvisor/blob/master/utils/sysfs/sysfs.go @ ef7e64f9

package machineinformer

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"k8s.io/klog/v2"

	"github.com/google/cadvisor/utils/sysfs"
)

const (
	blockDir     = "/sys/block"
	cacheDir     = "/sys/devices/system/cpu/cpu"
	netDir       = "/sys/class/net"
	dmiDir       = "/sys/class/dmi"
	ppcDevTree   = "/proc/device-tree"
	s390xDevTree = "/etc" // s390/s390x changes

	meminfoFile = "meminfo"

	sysFsCPUTopology = "topology"

	// CPUPhysicalPackageID is a physical package id of cpu#. Typically corresponds to a physical socket number,
	// but the actual value is architecture and platform dependent.
	CPUPhysicalPackageID = "physical_package_id"
	// CPUCoreID is the CPU core ID of cpu#. Typically it is the hardware platform's identifier
	// (rather than the kernel's). The actual value is architecture and platform dependent.
	CPUCoreID = "core_id"

	coreIDFilePath    = "/" + sysFsCPUTopology + "/core_id"
	packageIDFilePath = "/" + sysFsCPUTopology + "/physical_package_id"

	// memory size calculations

	cpuDirPattern  = "cpu*[0-9]"
	nodeDirPattern = "node*[0-9]"

	//HugePagesNrFile name of nr_hugepages file in sysfs
	HugePagesNrFile = "nr_hugepages"
)

var (
	nodeDir = "/sys/devices/system/node/"
)

type relocatableSysFs struct {
	root string
}

func NewRelocatableSysFs(root string) sysfs.SysFs {
	return &relocatableSysFs{
		root: root,
	}
}

func NewRealSysFs(root string) sysfs.SysFs {
	return NewRelocatableSysFs("")
}

func (fs *relocatableSysFs) GetNodesPaths() ([]string, error) {
	pathPattern := filepath.Join(fs.root, nodeDir, nodeDirPattern)
	return filepath.Glob(pathPattern)
}

func (fs *relocatableSysFs) GetCPUsPaths(cpusPath string) ([]string, error) {
	pathPattern := filepath.Join(fs.root, cpusPath, cpuDirPattern)
	return filepath.Glob(pathPattern)
}

func (fs *relocatableSysFs) GetCoreID(cpuPath string) (string, error) {
	coreIDFilePath := filepath.Join(fs.root, cpuPath, coreIDFilePath)
	coreID, err := ioutil.ReadFile(coreIDFilePath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(coreID)), err
}

func (fs *relocatableSysFs) GetCPUPhysicalPackageID(cpuPath string) (string, error) {
	packageIDFilePath := filepath.Join(fs.root, cpuPath, packageIDFilePath)
	packageID, err := ioutil.ReadFile(packageIDFilePath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(packageID)), err
}

func (fs *relocatableSysFs) GetMemInfo(nodePath string) (string, error) {
	meminfoPath := filepath.Join(fs.root, nodePath, meminfoFile)
	meminfo, err := ioutil.ReadFile(meminfoPath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(meminfo)), err
}

func (fs *relocatableSysFs) GetHugePagesInfo(hugePagesDirectory string) ([]os.FileInfo, error) {
	return ioutil.ReadDir(filepath.Join(fs.root, hugePagesDirectory))
}

func (fs *relocatableSysFs) GetHugePagesNr(hugepagesDirectory string, hugePageName string) (string, error) {
	hugePageFilePath := filepath.Join(fs.root, hugepagesDirectory, hugePageName, HugePagesNrFile)
	hugePageFile, err := ioutil.ReadFile(hugePageFilePath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(hugePageFile)), err
}

func (fs *relocatableSysFs) GetBlockDevices() ([]os.FileInfo, error) {
	return ioutil.ReadDir(filepath.Join(fs.root, blockDir))
}

func (fs *relocatableSysFs) GetBlockDeviceNumbers(name string) (string, error) {
	dev, err := ioutil.ReadFile(filepath.Join(fs.root, blockDir, name, "/dev"))
	if err != nil {
		return "", err
	}
	return string(dev), nil
}

func (fs *relocatableSysFs) GetBlockDeviceScheduler(name string) (string, error) {
	sched, err := ioutil.ReadFile(filepath.Join(fs.root, blockDir, name, "/queue/scheduler"))
	if err != nil {
		return "", err
	}
	return string(sched), nil
}

func (fs *relocatableSysFs) GetBlockDeviceSize(name string) (string, error) {
	size, err := ioutil.ReadFile(filepath.Join(fs.root, blockDir, name, "/size"))
	if err != nil {
		return "", err
	}
	return string(size), nil
}

func (fs *relocatableSysFs) GetNetworkDevices() ([]os.FileInfo, error) {
	files, err := ioutil.ReadDir(filepath.Join(fs.root, netDir))
	if err != nil {
		return nil, err
	}

	// Filter out non-directory & non-symlink files
	var dirs []os.FileInfo
	for _, f := range files {
		if f.Mode()|os.ModeSymlink != 0 {
			f, err = os.Stat(filepath.Join(fs.root, netDir, f.Name()))
			if err != nil {
				continue
			}
		}
		if f.IsDir() {
			dirs = append(dirs, f)
		}
	}
	return dirs, nil
}

func (fs *relocatableSysFs) GetNetworkAddress(name string) (string, error) {
	address, err := ioutil.ReadFile(filepath.Join(fs.root, netDir, name, "/address"))
	if err != nil {
		return "", err
	}
	return string(address), nil
}

func (fs *relocatableSysFs) GetNetworkMtu(name string) (string, error) {
	mtu, err := ioutil.ReadFile(filepath.Join(fs.root, netDir, name, "/mtu"))
	if err != nil {
		return "", err
	}
	return string(mtu), nil
}

func (fs *relocatableSysFs) GetNetworkSpeed(name string) (string, error) {
	speed, err := ioutil.ReadFile(filepath.Join(fs.root, netDir, name, "/speed"))
	if err != nil {
		return "", err
	}
	return string(speed), nil
}

func (fs *relocatableSysFs) GetNetworkStatValue(dev string, stat string) (uint64, error) {
	statPath := filepath.Join(fs.root, netDir, dev, "/statistics", stat)
	out, err := ioutil.ReadFile(statPath)
	if err != nil {
		return 0, fmt.Errorf("failed to read stat from %q for device %q", statPath, dev)
	}
	var s uint64
	n, err := fmt.Sscanf(string(out), "%d", &s)
	if err != nil || n != 1 {
		return 0, fmt.Errorf("could not parse value from %q for file %s", string(out), statPath)
	}
	return s, nil
}

func (fs *relocatableSysFs) GetCaches(id int) ([]os.FileInfo, error) {
	cpuPath := filepath.Join(fs.root, fmt.Sprintf("%s%d/cache", cacheDir, id))
	return ioutil.ReadDir(cpuPath)
}

func bitCount(i uint64) (count int) {
	for i != 0 {
		if i&1 == 1 {
			count++
		}
		i >>= 1
	}
	return
}

func getCPUCount(cache string) (count int, err error) {
	out, err := ioutil.ReadFile(filepath.Join(cache, "/shared_cpu_map"))
	if err != nil {
		return 0, err
	}
	masks := strings.Split(string(out), ",")
	for _, mask := range masks {
		// convert hex string to uint64
		m, err := strconv.ParseUint(strings.TrimSpace(mask), 16, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse cpu map %q: %v", string(out), err)
		}
		count += bitCount(m)
	}
	return
}

func (fs *relocatableSysFs) GetCacheInfo(id int, name string) (sysfs.CacheInfo, error) {
	cachePath := filepath.Join(fs.root, fmt.Sprintf("%s%d/cache/%s", cacheDir, id, name))
	out, err := ioutil.ReadFile(filepath.Join(cachePath, "/size"))
	if err != nil {
		return sysfs.CacheInfo{}, err
	}
	var size uint64
	n, err := fmt.Sscanf(string(out), "%dK", &size)
	if err != nil || n != 1 {
		return sysfs.CacheInfo{}, err
	}
	// convert to bytes
	size = size * 1024
	out, err = ioutil.ReadFile(filepath.Join(cachePath, "/level"))
	if err != nil {
		return sysfs.CacheInfo{}, err
	}
	var level int
	n, err = fmt.Sscanf(string(out), "%d", &level)
	if err != nil || n != 1 {
		return sysfs.CacheInfo{}, err
	}

	out, err = ioutil.ReadFile(filepath.Join(cachePath, "/type"))
	if err != nil {
		return sysfs.CacheInfo{}, err
	}
	cacheType := strings.TrimSpace(string(out))
	cpuCount, err := getCPUCount(cachePath)
	if err != nil {
		return sysfs.CacheInfo{}, err
	}
	return sysfs.CacheInfo{
		Size:  size,
		Level: level,
		Type:  cacheType,
		Cpus:  cpuCount,
	}, nil
}

func (fs *relocatableSysFs) GetSystemUUID() (string, error) {
	if id, err := ioutil.ReadFile(filepath.Join(fs.root, dmiDir, "id", "product_uuid")); err == nil {
		return strings.TrimSpace(string(id)), nil
	} else if id, err = ioutil.ReadFile(filepath.Join(fs.root, ppcDevTree, "system-id")); err == nil {
		return strings.TrimSpace(strings.TrimRight(string(id), "\000")), nil
	} else if id, err = ioutil.ReadFile(filepath.Join(fs.root, ppcDevTree, "vm,uuid")); err == nil {
		return strings.TrimSpace(strings.TrimRight(string(id), "\000")), nil
	} else if id, err = ioutil.ReadFile(filepath.Join(fs.root, s390xDevTree, "machine-id")); err == nil {
		return strings.TrimSpace(string(id)), nil
	} else {
		return "", err
	}
}

func (fs *relocatableSysFs) IsCPUOnline(cpuPath string) bool {
	onlinePath, err := filepath.Abs(filepath.Join(fs.root, cpuPath+"/../online"))
	if err != nil {
		klog.V(1).Infof("Unable to get absolute path for %s", cpuPath)
		return false
	}

	// Quick check to determine if file exists: if it does not then kernel CPU hotplug is disabled and all CPUs are online.
	_, err = os.Stat(onlinePath)
	if err != nil && os.IsNotExist(err) {
		return true
	}
	if err != nil {
		klog.V(1).Infof("Unable to stat %s: %s", onlinePath, err)
	}

	cpuID, err := getCPUID(cpuPath)
	if err != nil {
		klog.V(1).Infof("Unable to get CPU ID from path %s: %s", cpuPath, err)
		return false
	}

	isOnline, err := isCPUOnline(onlinePath, cpuID)
	if err != nil {
		klog.V(1).Infof("Unable to get online CPUs list: %s", err)
		return false
	}
	return isOnline
}

func getCPUID(dir string) (uint16, error) {
	regex := regexp.MustCompile("cpu([0-9]+)")
	matches := regex.FindStringSubmatch(dir)
	if len(matches) == 2 {
		id, err := strconv.Atoi(matches[1])
		if err != nil {
			return 0, err
		}
		return uint16(id), nil
	}
	return 0, fmt.Errorf("can't get CPU ID from %s", dir)
}

// isCPUOnline is copied from github.com/opencontainers/runc/libcontainer/cgroups/fs and modified to suite cAdvisor
// needs as Apache 2.0 license allows.
// It parses CPU list (such as: 0,3-5,10) into a struct that allows to determine quickly if CPU or particular ID is online.
// see: https://github.com/opencontainers/runc/blob/ab27e12cebf148aa5d1ee3ad13d9fc7ae12bf0b6/libcontainer/cgroups/fs/cpuset.go#L45
func isCPUOnline(path string, cpuID uint16) (bool, error) {
	fileContent, err := ioutil.ReadFile(path)
	if err != nil {
		return false, err
	}
	if len(fileContent) == 0 {
		return false, fmt.Errorf("%s found to be empty", path)
	}

	cpuList := strings.TrimSpace(string(fileContent))
	for _, s := range strings.Split(cpuList, ",") {
		splitted := strings.SplitN(s, "-", 3)
		switch len(splitted) {
		case 3:
			return false, fmt.Errorf("invalid values in %s", path)
		case 2:
			min, err := strconv.ParseUint(splitted[0], 10, 16)
			if err != nil {
				return false, err
			}
			max, err := strconv.ParseUint(splitted[1], 10, 16)
			if err != nil {
				return false, err
			}
			if min > max {
				return false, fmt.Errorf("invalid values in %s", path)
			}
			for i := min; i <= max; i++ {
				if uint16(i) == cpuID {
					return true, nil
				}
			}
		case 1:
			value, err := strconv.ParseUint(s, 10, 16)
			if err != nil {
				return false, err
			}
			if uint16(value) == cpuID {
				return true, nil
			}
		}
	}

	return false, nil
}

// Looks for sysfs cpu path containing given CPU property, e.g. core_id or physical_package_id
// and returns number of unique values of given property, exemplary usage: getting number of CPU physical cores
func GetUniqueCPUPropertyCount(cpuBusPath string, propertyName string) int {
	absCPUBusPath, err := filepath.Abs(cpuBusPath)
	if err != nil {
		klog.Errorf("Cannot make %s absolute", cpuBusPath)
		return 0
	}
	pathPattern := absCPUBusPath + "/cpu*[0-9]"
	sysCPUPaths, err := filepath.Glob(pathPattern)
	if err != nil {
		klog.Errorf("Cannot find files matching pattern (pathPattern: %s),  number of unique %s set to 0", pathPattern, propertyName)
		return 0
	}
	onlinePath, err := filepath.Abs(cpuBusPath + "/online")
	if err != nil {
		klog.V(1).Infof("Unable to get absolute path for %s", cpuBusPath+"/../online")
		return 0
	}

	if err != nil {
		klog.V(1).Infof("Unable to get online CPUs list: %s", err)
		return 0
	}
	uniques := make(map[string]bool)
	for _, sysCPUPath := range sysCPUPaths {
		cpuID, err := getCPUID(sysCPUPath)
		if err != nil {
			klog.V(1).Infof("Unable to get CPU ID from path %s: %s", sysCPUPath, err)
			return 0
		}
		isOnline, err := isCPUOnline(onlinePath, cpuID)
		if err != nil && !os.IsNotExist(err) {
			klog.V(1).Infof("Unable to determine CPU online state: %s", err)
			continue
		}
		if !isOnline && !os.IsNotExist(err) {
			continue
		}
		propertyPath := filepath.Join(sysCPUPath, sysFsCPUTopology, propertyName)
		propertyVal, err := ioutil.ReadFile(propertyPath)
		if err != nil {
			klog.Warningf("Cannot open %s, assuming 0 for %s of CPU %d", propertyPath, propertyName, cpuID)
			propertyVal = []byte("0")
		}
		packagePath := filepath.Join(sysCPUPath, sysFsCPUTopology, CPUPhysicalPackageID)
		packageVal, err := ioutil.ReadFile(packagePath)
		if err != nil {
			klog.Warningf("Cannot open %s, assuming 0 %s of CPU %d", packagePath, CPUPhysicalPackageID, cpuID)
			packageVal = []byte("0")

		}
		uniques[fmt.Sprintf("%s_%s", bytes.TrimSpace(propertyVal), bytes.TrimSpace(packageVal))] = true
	}
	return len(uniques)
}
