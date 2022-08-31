//go:build windows

package hcs

import (
	"context"
	"encoding/json"

	"github.com/Microsoft/hcsshim/internal/interop"
	hcsschema "github.com/Microsoft/hcsshim/internal/schema2"
)

// GetServiceProperties returns properties of the host compute service.
func GetServiceProperties(ctx context.Context, q hcsschema.PropertyQuery) (properties *hcsschema.ServiceProperties, err error) {
	operation := "hcs::GetServiceProperties"

	queryb, err := json.Marshal(q)
	if err != nil {
		return nil, err
	}

	var resultp, propertiesp *uint16
	err = hcsGetServiceProperties(string(queryb), &propertiesp, &resultp)
	events := processHcsResult(ctx, resultp)
	if err != nil {
		return nil, &HcsError{Op: operation, Err: err, Events: events}
	}

	if propertiesp == nil {
		return nil, ErrUnexpectedValue
	}
	propertiesRaw := interop.ConvertAndFreeCoTaskMemString(propertiesp)
	properties = &hcsschema.ServiceProperties{}
	if err := json.Unmarshal([]byte(propertiesRaw), properties); err != nil {
		return nil, err
	}

	return properties, nil
}
