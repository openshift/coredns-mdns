package publisher

import (
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

type CollisionStrategy int

const (
	Inaction CollisionStrategy = iota
	HostName
)

var (
	strategies = []string{
		"Inaction",
		"HostName",
	}
)

type Service struct {
	Name     string `mapstructure:"name"`
	HostName string `mapstructure:"host_name"`
	SvcType  string `mapstructure:"type"`
	Domain   string `mapstructure:"domain"`
	Port     int    `mapstructure:"port"`
	TTL      uint32 `mapstructure:"ttl"`
}

func (strategy CollisionStrategy) String() (string, error) {
	if err := strategy.valid(); err != nil {
		return "", err
	}
	return strategies[strategy], nil
}

func NewCollisionStrategy(strategy string) (CollisionStrategy, error) {
	for i, name := range strategies {
		if strings.EqualFold(strategy, name) {
			return CollisionStrategy(i), nil
		}
	}
	return CollisionStrategy(-1), fmt.Errorf("Unrecognized CollisionStrategy %s", strategy)
}

func CollisionStrategies() []string {
	return strategies
}

func (strategy CollisionStrategy) valid() error {
	if strategy < Inaction || strategy > HostName {
		return fmt.Errorf("Unrecognized CollisionStrategy")
	}
	return nil
}

func (s *Service) AlterName(strategy CollisionStrategy) error {
	if err := strategy.valid(); err != nil {
		return err
	}

	switch strategy {
	case Inaction:
		return nil
	case HostName:
		hostName, err := os.Hostname()
		if err != nil {
			return err
		}
		shortName := strings.Split(hostName, ".")[0]
		originalName := s.Name
		s.Name = originalName + "-" + shortName
		log.WithFields(logrus.Fields{
			"original": originalName,
			"new":      s.Name,
		}).Debug("Changing service name")
		return nil
	}
	return fmt.Errorf("Unimplemented collision avoidance strategy")
}
