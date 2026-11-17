package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Configuration struct {
	hosts []Host
}

// Initializes the configuration
func NewConfiguration(configPath string) (*Configuration, error) {
	confBytes, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var hosts []Host
	err = json.Unmarshal(confBytes, &hosts)
	if err != nil {
		return nil, err
	}

	this := new(Configuration)

	for i := range hosts {
		if err := hosts[i].Validate(); err != nil {
			fmt.Printf("Record %d (%q) has errors and is not loaded: %v\n", i, hosts[i].FQDN(), err)
			continue
		}

		if err := hosts[i].Populate(); err != nil {
			fmt.Printf("Record %d (%q) has errors and is not loaded: %v\n", i, hosts[i].FQDN(), err)
			continue
		}
		this.hosts = append(this.hosts, hosts[i])
	}

	newBytes, err := json.MarshalIndent(hosts, "", "\t")
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(newBytes, confBytes) {
		if err := writeFileAtomic(configPath, newBytes); err != nil {
			return nil, err
		}
	}

	if len(this.hosts) == 0 {
		return nil, fmt.Errorf("no records were loaded")
	}

	return this, nil
}

func (c *Configuration) GetHost(fqdn string) (*Host, error) {
	for i := range c.hosts {
		if c.hosts[i].FQDN() == fqdn {
			return &c.hosts[i], nil
		}
	}

	return nil, fmt.Errorf("unknown fqdn %q", fqdn)
}

// atomic write (temp file + rename).
func writeFileAtomic(path string, data []byte) error {
	// do not overwrite a symlink, resolve it
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	dir := filepath.Dir(resolved)

	// some folder = same filesystem = move is atomic
	tmp, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // useless in case of success, but cleans up otherwise

	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpName, resolved); err != nil {
		return err
	}

	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
