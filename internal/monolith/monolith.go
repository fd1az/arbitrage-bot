// Package monolith provides the application container and module interface.
package monolith

import (
	"context"

	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/fd1az/arbitrage-bot/internal/asset"
	"github.com/fd1az/arbitrage-bot/internal/config"
	"github.com/fd1az/arbitrage-bot/internal/di"
	"github.com/fd1az/arbitrage-bot/internal/logger"
)

// Monolith is the main application container providing access to shared infrastructure.
type Monolith interface {
	Config() *config.Config
	Logger() logger.LoggerInterface
	EthClient() *ethclient.Client
	AssetRegistry() *asset.Registry
	Services() di.ServiceRegistry
}

// Module represents a bounded context module that can register services and start up.
type Module interface {
	RegisterServices(di.Container) error
	Startup(context.Context, Monolith) error
}

// app implements the Monolith interface.
type app struct {
	config        *config.Config
	logger        logger.LoggerInterface
	ethClient     *ethclient.Client
	assetRegistry *asset.Registry
	container     di.Container
}

// New creates a new Monolith instance.
func New(cfg *config.Config, log logger.LoggerInterface) (*app, error) {
	// Create Ethereum client
	ethClient, err := ethclient.Dial(cfg.Ethereum.HTTPURL)
	if err != nil {
		return nil, err
	}

	// Use default asset registry (pre-populated with common assets)
	assetRegistry := asset.DefaultRegistry()

	container := di.NewContainer()

	// Register global services
	container.Register("config", cfg)
	container.Register("logger", log)
	container.Register("ethClient", ethClient)
	container.Register("assetRegistry", assetRegistry)

	return &app{
		config:        cfg,
		logger:        log,
		ethClient:     ethClient,
		assetRegistry: assetRegistry,
		container:     container,
	}, nil
}

func (a *app) Config() *config.Config {
	return a.config
}

func (a *app) Logger() logger.LoggerInterface {
	return a.logger
}

func (a *app) EthClient() *ethclient.Client {
	return a.ethClient
}

func (a *app) AssetRegistry() *asset.Registry {
	return a.assetRegistry
}

func (a *app) Services() di.ServiceRegistry {
	return a.container
}

// Container returns the DI container for module registration.
func (a *app) Container() di.Container {
	return a.container
}

// RegisterModules registers all provided modules.
func (a *app) RegisterModules(modules ...Module) error {
	for _, m := range modules {
		if err := m.RegisterServices(a.container); err != nil {
			return err
		}
	}
	return nil
}

// StartModules starts all provided modules.
func (a *app) StartModules(ctx context.Context, modules ...Module) error {
	for _, m := range modules {
		if err := m.Startup(ctx, a); err != nil {
			return err
		}
	}
	return nil
}

// Close closes all resources.
func (a *app) Close() error {
	if a.ethClient != nil {
		a.ethClient.Close()
	}
	return nil
}
