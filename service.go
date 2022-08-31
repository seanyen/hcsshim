package hcsshim

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Microsoft/hcsshim/internal/hcs"
	hcsschema "github.com/Microsoft/hcsshim/internal/schema2"
)

func GetHighestSupportedHcsSchemaVersion() (schemaVersion *hcsschema.Version, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()

	serviceProperties, err := hcs.GetServiceProperties(ctx, hcsschema.PropertyQuery{PropertyTypes: []string{"Basic"}})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve HCS schema version: %s", err)
	}

	basicInfo := &hcsschema.BasicInformation{}
	if err := json.Unmarshal(serviceProperties.Properties[0], &basicInfo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal HCS Schema Version: %s", err)
	}

	var highestVersion hcsschema.Version
	for _, version := range basicInfo.SupportedSchemaVersions {
		if version.Major >= highestVersion.Major {
			if (version.Major > highestVersion.Major) || (version.Minor > highestVersion.Minor) {
				highestVersion.Major = version.Major
				highestVersion.Minor = version.Minor
			}
		}
	}

	schemaVersion = &highestVersion

	return
}
