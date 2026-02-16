package paxos

import (
	"log/slog"
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
		slog.Error("Error opening config file", "error", err)
	}
	defer f.Close()

	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(cfg)
	if err != nil {
		slog.Error("Error parsing yaml object", "error", err)
	}
}
