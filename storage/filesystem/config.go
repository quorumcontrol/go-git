package filesystem

import (
	"fmt"
	stdioutil "io/ioutil"
	"os"

	"github.com/go-git/go-billy/v5"

	"github.com/go-git/go-git/v5/config"
	format "github.com/go-git/go-git/v5/plumbing/format/config"
	"github.com/go-git/go-git/v5/storage/filesystem/dotgit"
	"github.com/go-git/go-git/v5/utils/ioutil"
)

type ConfigStorage struct {
	dir *dotgit.DotGit
}

func (c *ConfigStorage) Config() (conf *config.Config, err error) {
	systemCfg := config.NewScopedConfig(format.SystemScope)
	userCfg := config.NewScopedConfig(format.UserScope)
	localCfg := config.NewScopedConfig(format.LocalScope)

	var b []byte
	data := []byte("\n")

	if b, err = c.systemConfig(); err != nil {
		return nil, err
	}
	data = append(data, b...)
	data = append(data, []byte("\n")...)
	if err = systemCfg.Unmarshal(data); err != nil {
		return nil, err
	}
	data = []byte("\n")

	if b, err = c.userConfig(); err != nil {
		return nil, err
	}
	data = append(data, b...)
	data = append(data, []byte("\n")...)
	if err = userCfg.Unmarshal(data); err != nil {
		return nil, err
	}
	data = []byte("\n")

	if b, err = c.localConfig(); err != nil {
		return nil, err
	}
	data = append(data, b...)
	data = append(data, []byte("\n")...)
	if err = localCfg.Unmarshal(data); err != nil {
		return nil, err
	}

	return config.NewMergedConfig(config.ScopedConfigs{
		format.LocalScope:  localCfg,
		format.UserScope:   userCfg,
		format.SystemScope: systemCfg,
	})
}

func (c *ConfigStorage) systemConfig() (b []byte, err error) {
	if b, err = c.dir.SystemConfig(); err != nil {
		if os.IsNotExist(err) {
			return []byte{}, nil
		}

		return nil, err
	}

	return b, nil
}

func (c *ConfigStorage) userConfig() (b []byte, err error) {
	if b, err = c.dir.UserConfig(); err != nil {
		fmt.Println("USER ERROR: ", err)
		if os.IsNotExist(err) {
			return []byte{}, nil
		}

		return nil, err
	}

	return b, nil
}

func (c *ConfigStorage) localConfig() (b []byte, err error) {
	f, err := c.dir.Config()
	if err != nil {
		if os.IsNotExist(err) {
			return []byte{}, nil
		}

		return nil, err
	}

	defer ioutil.CheckClose(f, &err)

	b, err = stdioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func (c *ConfigStorage) configFileWriter(cfg *config.Config) (billy.File, error) {
	switch cfg.Scope {
	case format.SystemScope:
		// This is likely to fail due to insufficient permission to write /etc/gitconfig.
		// So let's not blow up when it does.
		f, err := c.dir.SystemConfigWriter()
		if err != nil {
			if pathErr, ok := err.(*os.PathError); ok {
				if pathErr.Err.Error() == os.ErrPermission.Error() {
					return nil, nil
				}
				return nil, err
			}
			return nil, err
		}
		return f, nil
	case format.UserScope:
		return c.dir.UserConfigWriter()
	case format.LocalScope:
		return c.dir.LocalConfigWriter()
	default:
		return nil, fmt.Errorf("cannot get configFileWriter for scope %v", cfg.Scope)
	}
}

func (c *ConfigStorage) writeConfig(cfg *config.Config, file billy.File) error {
	b, err := cfg.Marshal()
	if err != nil {
		return err
	}

	defer ioutil.CheckClose(file, &err)

	_, err = file.Write(b)

	return err
}

func (c *ConfigStorage) SetConfig(cfg *config.Config) (err error) {
	if err = cfg.Validate(); err != nil {
		return
	}

	var file billy.File
	if cfg.Scope == format.MergedScope {
		for _, sc := range cfg.ScopedConfigs() {
			file, err = c.configFileWriter(sc)
			if err != nil {
				return
			}

			if file != nil {
				err = c.writeConfig(sc, file)
			}
		}
	} else {
		file, err = c.configFileWriter(cfg)
		if err != nil {
			return
		}

		if file != nil {
			err = c.writeConfig(cfg, file)
		}
	}

	return err
}
