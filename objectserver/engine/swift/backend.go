// Copyright (c) 2015 Rackspace
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

package swift

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/iqiyi/falcon/common"
	"github.com/iqiyi/falcon/common/fs"
	"github.com/iqiyi/falcon/common/pickle"
)

const (
	ONE_WEEK                = 604800
	METADATA_CHUNK_SIZE     = 65536
	HASH_FILE               = "hashes.pkl"
	HASH_INVALIDATIONS_FILE = "hashes.invalid"
)

func PolicyDir(policy int) string {
	if policy == 0 {
		return "objects"
	}
	return fmt.Sprintf("objects-%d", policy)
}

func UnPolicyDir(dir string) (int, error) {
	if dir == "objects" {
		return 0, nil
	}
	var policy int
	if n, err := fmt.Sscanf(dir, "objects-%d", &policy); n == 1 && err == nil {
		return policy, nil
	}
	return 0, fmt.Errorf("Unable to parse policy from dir")
}

func RawReadMetadata(fileNameOrFd interface{}) ([]byte, error) {
	var pickledMetadata []byte
	offset := 0
	for index := 0; ; index += 1 {
		var metadataName string
		// get name of next xattr
		if index == 0 {
			metadataName = "user.swift.metadata"
		} else {
			metadataName = "user.swift.metadata" + strconv.Itoa(index)
		}
		// get size of xattr
		length, err := fs.Getxattr(fileNameOrFd, metadataName, nil)
		if err != nil || length <= 0 {
			break
		}
		// grow buffer to hold xattr
		for cap(pickledMetadata) < offset+length {
			pickledMetadata = append(pickledMetadata, 0)
		}
		pickledMetadata = pickledMetadata[0 : offset+length]
		if _, err := fs.Getxattr(fileNameOrFd, metadataName, pickledMetadata[offset:]); err != nil {
			return nil, err
		}
		offset += length
	}
	return pickledMetadata, nil
}

func ReadMetadata(fileNameOrFd interface{}) (map[string]string, error) {
	pickledMetadata, err := RawReadMetadata(fileNameOrFd)
	if err != nil {
		return nil, err
	}
	v, err := pickle.PickleLoads(pickledMetadata)
	if err != nil {
		return nil, err
	}
	if v, ok := v.(map[interface{}]interface{}); ok {
		metadata := make(map[string]string, len(v))
		for mk, mv := range v {
			var mks, mvs string
			if mks, ok = mk.(string); !ok {
				return nil, fmt.Errorf("Metadata key not string: %v", mk)
			} else if mvs, ok = mv.(string); !ok {
				return nil, fmt.Errorf("Metadata value not string: %v", mv)
			}
			metadata[mks] = mvs
		}
		return metadata, nil
	}
	return nil, fmt.Errorf("Unpickled metadata not correct type")
}

func RawWriteMetadata(fd uintptr, buf []byte) error {
	for index := 0; len(buf) > 0; index++ {
		var metadataName string
		if index == 0 {
			metadataName = "user.swift.metadata"
		} else {
			metadataName = "user.swift.metadata" + strconv.Itoa(index)
		}
		writelen := METADATA_CHUNK_SIZE
		if len(buf) < writelen {
			writelen = len(buf)
		}
		if _, err := fs.Setxattr(fd, metadataName, buf[0:writelen]); err != nil {
			return err
		}
		buf = buf[writelen:]
	}
	return nil
}

func WriteMetadata(fd uintptr, v map[string]string) error {
	return RawWriteMetadata(fd, pickle.PickleDumps(v))
}

func QuarantineHash(hashDir string) error {
	// FYI- this does not invalidate the hash like swift's version. Please
	// do that yourself
	//          objects      partition    suffix       hash
	objsDir := filepath.Dir(filepath.Dir(filepath.Dir(hashDir)))
	driveDir := filepath.Dir(objsDir)
	quarantineDir := filepath.Join(driveDir, "quarantined", filepath.Base(objsDir))
	if err := os.MkdirAll(quarantineDir, 0755); err != nil {
		return err
	}
	hash := filepath.Base(hashDir)
	destDir := filepath.Join(quarantineDir, hash+"-"+common.UUID())
	if err := os.Rename(hashDir, destDir); err != nil {
		return err
	}
	return nil
}

// InvalidateHash invalidates the hashdir's suffix hash, indicating it needs to be recalculated.
func InvalidateHash(hashDir string) error {
	suffDir := filepath.Dir(hashDir)
	partitionDir := filepath.Dir(suffDir)

	if partitionLock, err := fs.LockPath(partitionDir, 10*time.Second); err != nil {
		return err
	} else {
		defer partitionLock.Close()
	}
	fp, err := os.OpenFile(filepath.Join(partitionDir, "hashes.invalid"), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0660)
	if err != nil {
		return err
	}
	defer fp.Close()
	_, err = fmt.Fprintf(fp, "%s\n", filepath.Base(suffDir))
	return err
}

func HashCleanupListDir(hashDir string, reclaimAge int64) ([]string, error) {
	fileList, err := fs.ReadDirNames(hashDir)
	returnList := []string{}
	if err != nil {
		if os.IsNotExist(err) {
			return returnList, nil
		}
		if fs.IsNotDir(err) {
			return returnList, ErrPathNotDir
		}
		return returnList, err
	}
	deleteRest := false
	deleteRestMeta := false
	if len(fileList) == 1 {
		filename := fileList[0]
		if strings.HasSuffix(filename, ".ts") {
			withoutSuffix := strings.Split(filename, ".")[0]
			if strings.Contains(withoutSuffix, "_") {
				withoutSuffix = strings.Split(withoutSuffix, "_")[0]
			}
			timestamp, _ := strconv.ParseFloat(withoutSuffix, 64)
			if time.Now().Unix()-int64(timestamp) > reclaimAge {
				os.RemoveAll(hashDir + "/" + filename)
				return returnList, nil
			}
		}
		returnList = append(returnList, filename)
	} else {
		for index := len(fileList) - 1; index >= 0; index-- {
			filename := fileList[index]
			if deleteRest {
				os.RemoveAll(hashDir + "/" + filename)
			} else {
				if strings.HasSuffix(filename, ".meta") {
					if deleteRestMeta {
						os.RemoveAll(hashDir + "/" + filename)
						continue
					}
					deleteRestMeta = true
				}
				if strings.HasSuffix(filename, ".ts") || strings.HasSuffix(filename, ".data") {
					// TODO: check .ts time for expiration
					deleteRest = true
				}
				returnList = append(returnList, filename)
			}
		}
	}
	return returnList, nil
}

func RecalculateSuffixHash(suffixDir string, reclaimAge int64) (string, error) {
	// the is hash_suffix in swift
	h := md5.New()

	hashList, err := fs.ReadDirNames(suffixDir)
	if err != nil {
		if fs.IsNotDir(err) {
			return "", ErrPathNotDir
		}
		return "", err
	}
	for _, fullHash := range hashList {
		hashPath := suffixDir + "/" + fullHash
		fileList, err := HashCleanupListDir(hashPath, reclaimAge)
		if err != nil {
			if err == ErrPathNotDir {
				if QuarantineHash(hashPath) == nil {
					InvalidateHash(hashPath)
				}
				continue
			}
			return "", err
		}
		if len(fileList) > 0 {
			for _, fileName := range fileList {
				io.WriteString(h, fileName)
			}
		} else {
			os.Remove(hashPath) // leaves the suffix (swift removes it but who cares)
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func ObjectFiles(directory string) (string, string) {
	fileList, err := fs.ReadDirNames(directory)
	metaFile := ""
	if err != nil {
		return "", ""
	}
	for index := len(fileList) - 1; index >= 0; index-- {
		filename := fileList[index]
		if strings.HasSuffix(filename, ".meta") {
			metaFile = filename
		}
		if strings.HasSuffix(filename, ".ts") || strings.HasSuffix(filename, ".data") {
			if metaFile != "" {
				return filepath.Join(directory, filename), filepath.Join(directory, metaFile)
			} else {
				return filepath.Join(directory, filename), ""
			}
		}
	}
	return "", ""
}

func applyMetaFile(metaFile string, datafileMetadata map[string]string) (map[string]string, error) {
	if metadata, err := ReadMetadata(metaFile); err != nil {
		return nil, err
	} else {
		for k, v := range datafileMetadata {
			if k == "Content-Length" ||
				k == "Content-Type" ||
				k == "deleted" ||
				k == "ETag" ||
				strings.HasPrefix(k, "X-Object-Sysmeta-") {
				metadata[k] = v
			}
		}
		return metadata, nil
	}
}

func OpenObjectMetadata(fd uintptr, metaFile string) (map[string]string, error) {
	datafileMetadata, err := ReadMetadata(fd)
	if err != nil {
		return nil, err
	}
	if metaFile != "" {
		return applyMetaFile(metaFile, datafileMetadata)
	}
	return datafileMetadata, nil
}

func ObjectMetadata(dataFile string, metaFile string) (map[string]string, error) {
	datafileMetadata, err := ReadMetadata(dataFile)
	if err != nil {
		return nil, err
	}
	if metaFile != "" {
		return applyMetaFile(metaFile, datafileMetadata)
	}
	return datafileMetadata, nil
}
