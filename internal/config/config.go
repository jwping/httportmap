package config

import (
	"context"
	"io/ioutil"

	"github.com/jwping/logger"
	"gopkg.in/yaml.v2"
)

type DestAddr struct {
	// Enable        bool              `yaml:"enable"`
	EnableAuth    bool              `yaml:"enable_auth"`
	Authorization map[string]string `yaml:"authorization"`
	Dest          string            `yaml:"dest"`

	SrcPort string `yaml:"-"`
	// server  *http.Server
	ShutdownCancel context.CancelFunc `yaml:"-"`
}

func (d *DestAddr) Update(dest DestAddr) {
	d.SrcPort = dest.SrcPort
	d.EnableAuth = dest.EnableAuth
	d.Authorization = dest.Authorization
	d.Dest = dest.Dest
	// d.Enable = dest.Enable

}

type System struct {
	Listen     string `yaml:"listen"`
	SessionTTL int    `yaml:"session_ttl"`
}

type HttPortMap struct {
	System  System               `yaml:"system"`
	PortMap map[string]*DestAddr `yaml:"portmap"`
}

func ReadConfig(cPath string, clog *logger.Logger) (*HttPortMap, error) {
	cData, err := ioutil.ReadFile(cPath)
	if err != nil {
		clog.Error("读取配置文件异常", err)
		return nil, err
	}

	httPortMap := &HttPortMap{}
	if err := yaml.Unmarshal(cData, &httPortMap); err != nil {
		clog.Error("yaml反序列化失败", err)
		return nil, err
	}

	return httPortMap, nil
}
