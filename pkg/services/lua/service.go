package lua

import (
	"sync"

	ee "github.com/gravestench/eventemitter"
	"github.com/gravestench/runtime"
	"github.com/gravestench/runtime/pkg/events"
	"github.com/rs/zerolog"
	"github.com/yuin/gopher-lua"

	"github.com/gravestench/dark-magic/pkg/services/configFile"
)

type Service struct {
	cfg    configFile.Dependency
	logger *zerolog.Logger
	state  *lua.LState
	events *ee.EventEmitter
	mux    sync.Mutex
}

func (s *Service) Init(rt runtime.R) {
	s.state = lua.NewState()
	s.bindLoggerToLuaEnvironment()

	rt.Events().On(events.EventServiceAdded, s.tryToExportToLuaEnvironment)

	for _, service := range rt.Services() {
		if candidate, ok := service.(UsesLuaEnvironment); ok {
			s.tryToExportToLuaEnvironment(candidate)
		}
	}

	// wait for all siblings to be ready before we launch scripts
	for _, service := range rt.Services() {
		if candidate, ok := service.(runtime.HasDependencies); ok {
			if !candidate.DependenciesResolved() {
				continue
			}
		}

		break // all deps resolved for all siblings
	}

	cfg, err := s.cfg.GetConfigByFileName(s.ConfigFileName())
	if err != nil {
		s.logger.Fatal().Msgf("getting config: %v", err)
	}

	initScriptPath := cfg.Group(s.Name()).GetString("init script")

	s.runScript(initScriptPath)

	go s.watchFileAndCallOnChange(initScriptPath, func() {
		s.runScript(initScriptPath)
	})
}

func (s *Service) Name() string {
	return "Lua Environment"
}

func (s *Service) BindLogger(logger *zerolog.Logger) {
	s.logger = logger
}

func (s *Service) Logger() *zerolog.Logger {
	return s.logger
}
