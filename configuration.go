package paxos

import (
	"github.com/sirupsen/logrus"
	"os"
	"github.com/go-yaml/yaml"
)

type Config struct {
	NumAcceptor int  `yaml:"numbers.acceptor"`
	NumReplicas int  `yaml:"numbers.replicas"`
	NumLeaders  int  `yaml:"numbers.leaders"`
}

// Function reads the config file. Pass a Config object as reference
func ReadFile(cfg *Config){
	f, err:= os.Open("config.yaml")
	if err != nil {
		logrus.WithError(err).Error("Error opening config file")
	}
	defer f.Close()

	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(cfg)
	if err != nil {
		logrus.WithError(err).Error("Error parsing yaml object")
	}
}