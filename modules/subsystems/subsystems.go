package subsystems

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/tevino/abool"

	"github.com/safing/portbase/config"
	"github.com/safing/portbase/modules"
)

var (
	subsystems       []*Subsystem
	subsystemsMap    = make(map[string]*Subsystem)
	subsystemsLock   sync.Mutex
	subsystemsLocked = abool.New()

	handlingConfigChanges = abool.New()
)

// Register registers a new subsystem. The given option must be a bool option. Should be called in the module's prep function. The config option must not yet be registered and will be registered for you.
func Register(name, description string, module *modules.Module, configKeySpace string, option *config.Option) error {
	// lock slice and map
	subsystemsLock.Lock()
	defer subsystemsLock.Unlock()

	// check if registration is closed
	if subsystemsLocked.IsSet() {
		return errors.New("subsystems can only be registered in prep phase")
	}

	// check if already registered
	_, ok := subsystemsMap[name]
	if ok {
		return fmt.Errorf(`subsystem "%s" already registered`, name)
	}

	// create new
	new := &Subsystem{
		Name:        name,
		Description: description,
		module:      module,

		Status: statusFromModule(module),

		toggleOption:   option,
		ConfigKeySpace: configKeySpace,
	}
	if new.toggleOption != nil {
		new.ToggleOptionKey = new.toggleOption.Key
		new.ExpertiseLevel = new.toggleOption.ExpertiseLevel
		new.ReleaseLevel = new.toggleOption.ReleaseLevel
	}

	// register config
	if option != nil {
		err := config.Register(option)
		if err != nil {
			return fmt.Errorf("failed to register config: %s", err)
		}
	}
	new.toggleValue = config.GetAsBool(new.ToggleOptionKey, false)

	// add to lists
	subsystemsMap[name] = new
	subsystems = append(subsystems, new)

	return nil
}

func handleModuleChanges(m *modules.Module) {
	// check if ready
	if !subsystemsLocked.IsSet() {
		return
	}

	// find module status
	var moduleSubsystem *Subsystem
	var moduleStatus *ModuleStatus
subsystemLoop:
	for _, subsystem := range subsystems {
		if m.Name == subsystem.Status.Name {
			moduleSubsystem = subsystem
			moduleStatus = subsystem.Status
			break subsystemLoop
		}
		for _, status := range subsystem.Dependencies {
			if m.Name == status.Name {
				moduleSubsystem = subsystem
				moduleStatus = status
				break subsystemLoop
			}
		}
	}
	// abort if not found
	if moduleSubsystem == nil || moduleStatus == nil {
		return
	}

	// update status
	moduleSubsystem.Lock()
	changed := compareAndUpdateStatus(m, moduleStatus)
	if changed {
		moduleSubsystem.makeSummary()
	}
	moduleSubsystem.Unlock()

	// save
	if changed {
		moduleSubsystem.Save()
	}
}

func handleConfigChanges(ctx context.Context, data interface{}) error {
	// check if ready
	if !subsystemsLocked.IsSet() {
		return nil
	}

	// potentially catch multiple changes
	if handlingConfigChanges.SetToIf(false, true) {
		time.Sleep(100 * time.Millisecond)
		handlingConfigChanges.UnSet()
	} else {
		return nil
	}

	// only run one instance at any time
	subsystemsLock.Lock()
	defer subsystemsLock.Unlock()

	var changed bool
	for _, subsystem := range subsystems {
		if subsystem.module.SetEnabled(subsystem.toggleValue()) {
			// if changed
			changed = true
		}
	}

	// trigger module management if any setting was changed
	if changed {
		err := modules.ManageModules()
		if err != nil {
			module.Error(
				"modulemgmt-failed",
				fmt.Sprintf("The subsystem framework failed to start or stop one or more modules.\nError: %s\nCheck logs for more information.", err),
			)
		} else {
			module.Resolve("modulemgmt-failed")
		}
	}

	return nil
}