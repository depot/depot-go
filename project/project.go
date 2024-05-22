package project

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

// Returns the project ID from the environment or config file.
// Searches from the directory of each of the files.
func ResolveProjectID(id string, files ...string) string {
	if id != "" {
		return id
	}

	id = os.Getenv("DEPOT_PROJECT_ID")
	if id != "" {
		return id
	}

	dirs, err := WorkingDirectories(files...)
	if err != nil {
		return ""
	}

	// Only a single project ID is allowed.
	uniqueIDs := make(map[string]struct{})

	for _, dir := range dirs {
		cwd, _ := filepath.Abs(dir)
		config, _, err := ReadConfig(cwd)
		if err == nil {
			id = config.ID
			uniqueIDs[id] = struct{}{}
		}
	}

	return id
}

// Returns all directories for any files.  If no files are specified then
// the current working directory is returned.  Special handling for stdin
// is also included by assuming the current working directory.
func WorkingDirectories(files ...string) ([]string, error) {
	directories := []string{}
	if len(files) == 0 {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		directories = append(directories, cwd)
	}

	for _, file := range files {
		if file == "-" || file == "" {
			cwd, err := os.Getwd()
			if err != nil {
				return nil, err
			}
			directories = append(directories, cwd)
			continue
		}

		if fi, err := os.Stat(file); err == nil && fi.IsDir() {
			directories = append(directories, file)
		} else {
			directories = append(directories, filepath.Dir(file))
		}
	}

	return directories, nil
}

type ProjectConfig struct {
	ID string `json:"id" yaml:"id"`
}

func ReadConfig(cwd string) (*ProjectConfig, string, error) {
	filename, err := FindConfigFileUp(cwd)
	if err != nil {
		return nil, "", err
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, "", err
	}

	var config ProjectConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, "", err
	}

	return &config, filename, nil
}

func FindConfigFileUp(current string) (string, error) {
	for {
		path := filepath.Join(current, "depot.json")
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
		path = filepath.Join(current, "depot.yml")
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
		path = filepath.Join(current, "depot.yaml")
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
		next := filepath.Dir(current)
		if next == current {
			break
		}
		current = next
	}
	return "", fmt.Errorf("no project config found")
}
