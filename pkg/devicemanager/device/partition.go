/*
   Copyright @ 2021 bocloud <fushaosong@beyondcent.com>.

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

package device

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	carinav1 "github.com/carina-io/carina/api/v1"
	"github.com/carina-io/carina/utils/exec"
	"github.com/carina-io/carina/utils/log"
)

const (
	// DiskType is a disk type
	DiskType = "disk"
	// SSDType is an sdd type
	SSDType = "ssd"
	// PartType is a partition type
	PartType = "part"
	// CryptType is an encrypted type
	CryptType = "crypt"
	// LVMType is an LVM type
	LVMType = "lvm"
	// MultiPath is for multipath devices
	MultiPath = "mpath"
	// LinearType is a linear type
	LinearType = "linear"
	sgdiskCmd  = "sgdisk"

	LoopType = "loop"
)

type LocalPartition interface {
	ListPartitions() (partitions []carinav1.Partition, err error)
	ListDevicePartitions() (rawDevices []carinav1.RawDevice, err error)
	AddDevicePartition(device string, name, start, end string) (partition carinav1.Partition, err error)
	DelDevicePartition(device, partitionNumber string) (bool, error)
	GetDevicePartitions(device string) (partitions []carinav1.Partition, err error)
	GetDeviceUnUsePartitions(device string) (partitions []carinav1.Partition, unusedSpace uint64, err error)
	IsPartType(device string) (bool, error)
	GetUdevInfo(device string) (map[string]string, error)
}

type LocalPartitionImplement struct {
	LocalDeviceImplement LocalDeviceImplement
	Executor             exec.Executor
}

//list all device partitions
func (ld *LocalPartitionImplement) ListPartitions() (partitions []carinav1.Partition, err error) {
	divices, err := ld.LocalDeviceImplement.ListDevices()
	if err != nil {
		return partitions, fmt.Errorf("failed to list all devices: %+v", err)
	}
	if len(divices) < 1 {
		return partitions, fmt.Errorf("unable to get one devices: %+v", err)
	}
	for _, v := range divices {
		log.Infof("list device %s", v)
		parttiontype, err := ld.LocalDeviceImplement.GetDiskPartitionType(v)
		if err != nil {
			log.Infof("failed to get  devices Partition Table Type: %+v", err)
		}
		if parttiontype == " " || parttiontype == "unknown" {
			continue
		}
		partition, err := ld.GetDevicePartitions(v)
		if err != nil {
			log.Errorf("failed to list all devices: %+v", err)
			continue
		}
		for _, item := range partition {
			partitions = append(partitions, item)
		}
	}
	return partitions, nil

}

//list all device partitions
func (ld *LocalPartitionImplement) ListDevicePartitions() (rawDevices []carinav1.RawDevice, err error) {
	divices, err := ld.LocalDeviceImplement.ListDevicesDetail("")
	if err != nil {
		return rawDevices, fmt.Errorf("failed to list all devices: %+v", err)
	}
	if len(divices) < 1 {
		return rawDevices, fmt.Errorf("unable to get one devices: %+v", err)
	}
	for _, v := range divices {
		if v.Type != PartType {
			continue
		}
		log.Infof("list device %s", v)
		parttiontype, err := ld.LocalDeviceImplement.GetDiskPartitionType(v.ParentName)
		if err != nil {
			log.Infof("failed to get  devices Partition Table Type: %s,%+v", v.Name, err)
		}
		if parttiontype == " " || parttiontype == "unknown" {
			continue
		}
		rawDevicesItem := new(carinav1.RawDevice)
		tmp, err := json.Marshal(v)
		if err != nil {
			log.Infof("failed to marshal devices Partition %s,%+v", v.Name, err)
		}
		json.Unmarshal(tmp, &rawDevicesItem)
		rawDevicesItem.Partition, err = ld.GetDevicePartitions(v.Name)
		if err != nil {
			log.Errorf("failed to list all devices: %+v", err)
			continue
		}

		rawDevices = append(rawDevices, *rawDevicesItem)

	}
	return rawDevices, nil

}

// add partition to give device
// parted -s /dev/sdX -- mklabel msdos
// mkpart primary fat32 64s 4MiB \
// mkpart primary fat32 4MiB -1s
//ext2,fat16, fat32,hfs, hfs+, hfsx,linux-swap,NTFS,reiserfs,ufs,btrfs
//name 2 'carina.io/pods-name-volume/pvc-1'
func (ld *LocalPartitionImplement) AddDevicePartition(device string, name, start, end string) (partition carinav1.Partition, err error) {
	parttiontype, err := ld.LocalDeviceImplement.GetDiskPartitionType(device)
	if err != nil {
		return partition, err
	}
	if parttiontype == " " || parttiontype == "unknown" {
		//rebuild parttion
		_, err := ld.Executor.ExecuteCommandWithOutput("parted", "-s", fmt.Sprintf("/dev/%s", device), "mklable", "gpt")
		if err != nil {
			return partition, err
		}
	}

	_, err = ld.Executor.ExecuteCommandWithOutput("parted", "-s", fmt.Sprintf("/dev/%s", device), "mkpart", "primary", start, end)
	if err != nil {
		log.Error("exec parted -s", fmt.Sprintf("/dev/%s", device), "mkpart", "primary", start, end, "failed"+err.Error())
		return partition, err
	}

	output, err := ld.Executor.ExecuteCommandWithOutput("parted", "-s", fmt.Sprintf("/dev/%s", device), "p")
	if err != nil {
		return partition, err
	}

	partitionString := strings.ReplaceAll(output, "\"", "")
	partitionsList := strings.Split(partitionString, "\n")
	locationNum := 0

	for i, partitions := range partitionsList {

		if strings.Contains(partitions, "Number") {
			locationNum = i
		}
		if locationNum == 0 || i <= locationNum {
			continue
		}
		log.Infof("found partition in line %s", i)
		tmp := strings.Split(partitions, " ")
		partition.Number = tmp[0]
		partition.Start = tmp[1]
		partition.End = tmp[2]
		partition.Size = tmp[3]
		partition.Filesystem = tmp[4]
		partition.Name = tmp[5]
		partition.Flags = tmp[6]
		if partition.Start == start && partition.End == end {
			//set partition name
			_, err = ld.Executor.ExecuteCommandWithOutput("parted", "-s", fmt.Sprintf("/dev/%s", device), "name", partition.Number, name)
			if err != nil {
				log.Error("exec parted -s", fmt.Sprintf("/dev/%s", device), "name", partition.Number, name, "failed"+err.Error())
				return partition, err
			}
			return partition, nil
		}
	}

	return partition, nil
}

// delete a partition on a given device
func (ld *LocalPartitionImplement) DelDevicePartition(device, partitionNumber string) (bool, error) {
	_, err := ld.Executor.ExecuteCommandWithOutput("parted", fmt.Sprintf("/dev/%s", device), "rm", partitionNumber)
	if err != nil {
		log.Error("exec parted -s", fmt.Sprintf("/dev/%s", device), "rm", partitionNumber, "failed"+err.Error())
		return false, err
	}

	return true, nil
}

// GetDevicePartitions gets partitions on a given device
func (ld *LocalPartitionImplement) GetDevicePartitions(device string) (partitions []carinav1.Partition, err error) {

	var devicePath string
	splitDevicePath := strings.Split(device, "/")
	if len(splitDevicePath) == 1 {
		devicePath = fmt.Sprintf("/dev/%s", device) //device path for OSD on devices.
	} else {
		devicePath = device //use the exact device path (like /mnt/<pvc-name>) in case of PVC block device
	}

	output, err := ld.Executor.ExecuteCommandWithOutput("parted", "-s", fmt.Sprintf("/dev/%s", devicePath), "p")
	log.Infof("Output: %+v", output)
	if err != nil {
		return partitions, fmt.Errorf("failed to get device %s partitions. %+v", device, err)
	}
	partitions = parsePartitionString(output)
	return partitions, nil
}

//
func (ld *LocalPartitionImplement) GetDeviceUnUsePartitions(device string) (partitions []carinav1.Partition, unusedSpace uint64, err error) {
	var devicePath string
	splitDevicePath := strings.Split(device, "/")
	if len(splitDevicePath) == 1 {
		devicePath = fmt.Sprintf("/dev/%s", device) //device path for OSD on devices.
	} else {
		devicePath = device //use the exact device path (like /mnt/<pvc-name>) in case of PVC block device
	}
	output, err := ld.Executor.ExecuteCommandWithOutput("parted", "-s", fmt.Sprintf("/dev/%s", devicePath), "p", "free")
	log.Infof("Output: %+v", output)
	if err != nil {
		return partitions, 0, fmt.Errorf("failed to get device %s partitions. %+v", device, err)
	}
	partitions, unusedSpace = parsePartitionUnUseString(output)
	return partitions, unusedSpace, nil
}

// GetUdevInfo gets udev information
func (ld *LocalPartitionImplement) GetUdevInfo(device string) (map[string]string, error) {
	output, err := ld.Executor.ExecuteCommandWithOutput("udevadm", "info", "--query=property", fmt.Sprintf("/dev/%s", device))
	if err != nil {
		return nil, err
	}

	return parseUdevInfo(output), nil
}

// IsPartType returns if a device is owned by lvm or partition
func (ld *LocalPartitionImplement) IsPartType(device string) (bool, error) {
	devProps, err := ld.LocalDeviceImplement.ListDevicesDetail(device)
	if err != nil {
		return false, fmt.Errorf("failed to get device properties for %q: %+v", device, err)
	}
	return devProps[0].Type == PartType, nil
}

func parsePartitionString(partitionString string) []carinav1.Partition {
	resp := []carinav1.Partition{}

	if partitionString == "" {
		return resp
	}
	partitionString = strings.ReplaceAll(partitionString, "\"", "")
	partitionsList := strings.Split(partitionString, "\n")
	locationNum := 0
	for i, partitions := range partitionsList {
		partition := carinav1.Partition{}
		if strings.Contains(partitions, "Number") {
			locationNum = i
		}
		if locationNum == 0 || i <= locationNum {
			continue
		}
		log.Infof("found partition in line %s", i)
		tmp := strings.Split(partitions, " ")
		partition.Number = tmp[0]
		partition.Start = tmp[1]
		partition.End = tmp[2]
		partition.Size = tmp[3]
		partition.Filesystem = tmp[4]
		partition.Name = tmp[5]
		partition.Flags = tmp[6]
		resp = append(resp, partition)

	}
	return resp

}

func parsePartitionUnUseString(partitionString string) (partitions []carinav1.Partition, unusedSpace uint64) {
	resp := []carinav1.Partition{}
	if partitionString == "" {
		return resp, 0
	}
	partitionString = strings.ReplaceAll(partitionString, "\"", "")
	partitionsList := strings.Split(partitionString, "\n")
	for i, partitions := range partitionsList {
		partition := carinav1.Partition{}
		if !strings.Contains(partitions, "Free Space") {
			continue
		}

		log.Infof("found partition Free Space in line %s %s", i, partitions)
		tmp := strings.Split(partitions, " ")
		partition.Number = tmp[0]
		partition.Start = tmp[1]
		partition.End = tmp[2]
		partition.Size = tmp[3]
		partition.Filesystem = tmp[4]
		partition.Name = tmp[5]
		partition.Flags = tmp[6]
		resp = append(resp, partition)
		size, _ := strconv.Atoi(partition.Size)
		unusedSpace += uint64(size)

	}
	return resp, unusedSpace

}

func parseUdevInfo(output string) map[string]string {
	lines := strings.Split(output, "\n")
	result := make(map[string]string, len(lines))
	for _, v := range lines {
		pairs := strings.Split(v, "=")
		if len(pairs) > 1 {
			result[pairs[0]] = pairs[1]
		}
	}
	return result
}
