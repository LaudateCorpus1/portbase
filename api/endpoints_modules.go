package api

import (
	"errors"
	"fmt"
)

func registerModulesEndpoints() error {
	if err := RegisterEndpoint(Endpoint{
		Path:        "modules/{moduleName:.+}/trigger/{eventName:.+}",
		Write:       PermitSelf,
		ActionFunc:  triggerEvent,
		Name:        "Trigger Event",
		Description: "Triggers an event of an internal module.",
	}); err != nil {
		return err
	}

	return nil
}

func triggerEvent(ar *Request) (msg string, err error) {
	// Get parameters.
	moduleName := ar.URLVars["moduleName"]
	eventName := ar.URLVars["eventName"]
	if moduleName == "" || eventName == "" {
		return "", errors.New("invalid parameters")
	}

	// Inject event.
	if err := module.InjectEvent("api event injection", moduleName, eventName, nil); err != nil {
		return "", fmt.Errorf("failed to inject event: %w", err)
	}

	return "event successfully injected", nil
}
