package helper

import (
	"errors"
	"github.com/bassbeaver/gioc"
	"github.com/spf13/viper"
	"os"
	"path/filepath"
	"strings"
)

const (
	configServicesPrefix = "services"
)

func StringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}

	return false
}

func GetStringPart(source, separator string, part int) string {
	parts := strings.Split(source, separator)

	return parts[part]
}

func RegisterService(
	configObj *viper.Viper,
	containerObj *gioc.Container,
	serviceAlias string,
	factoryMethod interface{},
	enableCaching bool,
) error {
	configServicePath := configServicesPrefix + "." + serviceAlias
	configServiceArgumentsPath := configServicesPrefix + "." + serviceAlias + ".arguments"
	if !configObj.IsSet(configServicePath) {
		return errors.New(serviceAlias + " service configuration not found")
	}

	var arguments []string
	if configObj.IsSet(configServiceArgumentsPath) {
		arguments = configObj.GetStringSlice(configServiceArgumentsPath)
	} else {
		arguments = make([]string, 0)
	}

	containerObj.RegisterServiceFactoryByAlias(
		serviceAlias,
		gioc.Factory{
			Create:    factoryMethod,
			Arguments: arguments,
		},
		enableCaching,
	)

	return nil
}

func BuildConfigFromDir(configPath string) (*viper.Viper, error) {
	configObj := viper.New()

	var configDir string
	configPathStat, configPathStatError := os.Stat(configPath)
	if nil != configPathStatError {
		return nil, errors.New("failed to read configs: " + configPathStatError.Error())
	}
	if configPathStat.IsDir() {
		configDir = configPath
	} else {
		configDir = filepath.Dir(configPath)
	}

	firstConfigFile := true
	pathWalkError := filepath.Walk(
		configDir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return errors.New("failed to read config file " + path + ", error: " + err.Error())
			}

			if info.IsDir() {
				return nil
			}

			configFilePath := filepath.Dir(path)
			configFileExt := filepath.Ext(info.Name())
			// if extension is not allowed - take next file
			if !StringInSlice(configFileExt[1:], viper.SupportedExts) {
				return nil
			}

			configFileName := info.Name()[0 : len(info.Name())-len(configFileExt)]

			configObj.AddConfigPath(configFilePath)
			configObj.SetConfigName(configFileName)

			if firstConfigFile {
				if configError := configObj.ReadInConfig(); nil != configError {
					return configError
				}

				firstConfigFile = false
			} else {
				if configError := configObj.MergeInConfig(); nil != configError {
					return configError
				}
			}

			return nil
		},
	)
	if nil != pathWalkError {
		return nil, errors.New("failed to read configs: " + pathWalkError.Error())
	}

	return configObj, nil
}
