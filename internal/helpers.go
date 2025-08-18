package internal

import (
	"fmt"
	"os"
	"path/filepath"
)

// checks whether a string is part of an array of strings
func arrayContains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}

func varFileExtensionAllowed(file string) bool {
	ext := filepath.Ext(file)

	return ext == ".tfvars" || ext == ".tf" || ext == ".json"
}

// filters files on tfvar files
func filterTfVarsFiles(files []string) []string {
	fileNames := []string{}

	for _, file := range files {
		if varFileExtensionAllowed(file) {
			fileNames = append(fileNames, file)
		}
	}

	return fileNames
}

// lists all files in a directory and its sub directories
func listDirRecursive(basePath string) ([]string, error) {
	all, err := os.ReadDir(basePath)
	if err != nil {
		return nil, err
	}

	files := []string{}

	for _, f := range all {
		if f.Type().IsRegular() {
			files = append(files, fmt.Sprintf("%s/%s", basePath, f.Name()))
			continue
		}

		if !f.Type().IsDir() {
			continue
		}

		nestedDirPath := filepath.Join(basePath, f.Name())

		listed, err := listDirRecursive(nestedDirPath)
		if err != nil {
			return nil, err
		}

		files = append(files, listed...)
	}

	return files, nil
}

// return a list of tfvar files
func getTfVarFilesPaths(path string) ([]string, error) {
	files, err := listDirRecursive(path)
	if err != nil {
		return nil, err
	}

	return filterTfVarsFiles(files), nil
}

// checks whether if file exist
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
