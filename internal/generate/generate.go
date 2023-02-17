package generate

import (
	"bytes"
	"fmt"
	"httportmap-server/internal/config"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"github.com/jwping/logger"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"gopkg.in/yaml.v2"
)

func GenerateConfig(mPath, ifs string, logger *logger.Logger) error {
	data, err := ioutil.ReadFile(mPath)
	if err != nil {
		logger.Error("文件读取失败", err)
		return err
	}

	portMap := map[string]*config.DestAddr{}

	buffer := bytes.NewBuffer(data)
	for {
		line, readLinErr := buffer.ReadString(byte('\n'))
		if readLinErr != nil && readLinErr != io.EOF {
			logger.Error("行读取失败", err)
			return err
		}

		line = strings.Replace(line, "\n", "", -1)

		array := strings.Split(line, ifs)

		nanoid, err := gonanoid.New()
		if err != nil {
			logger.Debug("nanoid生成失败", array[0], err)
			continue
		}
		portMap[array[0]] = &config.DestAddr{
			EnableAuth: true,
			Authorization: map[string]string{
				"ops": nanoid,
			},
			Dest: array[1] + ":" + array[2],
		}

		if readLinErr == io.EOF {
			break
		}
	}

	httPortMap := config.HttPortMap{
		System: config.System{
			Listen:     ":8080",
			SessionTTL: 24,
		},
		PortMap: portMap,
	}

	outData, err := yaml.Marshal(httPortMap)
	if err != nil {
		logger.Error("序列化yaml失败", err)
		return err
	}

	outPath := filepath.Join(filepath.Dir(mPath), fmt.Sprintf("config_%d.yaml", time.Now().Unix()))
	if err = ioutil.WriteFile(outPath, outData, 0660); err != nil {
		logger.Error("写入文件失败", err)
		return err
	}

	logger.Info("输出文件路径", outPath)

	return nil
}
