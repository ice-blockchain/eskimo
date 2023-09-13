// SPDX-License-Identifier: ice License 1.0

package fixture

import (
	"fmt"
	"testing"

	"github.com/testcontainers/testcontainers-go"

	connectorsfixture "github.com/ice-blockchain/wintr/connectors/fixture"
	messagebrokerfixture "github.com/ice-blockchain/wintr/connectors/message_broker/fixture"
	storagefixture "github.com/ice-blockchain/wintr/connectors/storage/fixture"
)

func StartLocalTestEnvironment() {
	connectorsfixture.
		NewTestRunner(applicationYAMLKey, nil, WTestConnectors()...).
		StartConnectorsIndefinitely()
}

//nolint:gocritic // Because that's exactly what we want.
func RunTests(
	m *testing.M,
	dbConnector *storagefixture.TestConnector,
	mbConnector *messagebrokerfixture.TestConnector,
	lifeCycleHooks ...*connectorsfixture.ConnectorLifecycleHooks,
) {
	*dbConnector = newDBConnector()
	*mbConnector = newMBConnector()

	var connectorLifecycleHooks *connectorsfixture.ConnectorLifecycleHooks
	if len(lifeCycleHooks) == 1 {
		connectorLifecycleHooks = lifeCycleHooks[0]
	}

	connectorsfixture.
		NewTestRunner(applicationYAMLKey, connectorLifecycleHooks, *dbConnector, *mbConnector).
		RunTests(m)
}

func WTestConnectors() []connectorsfixture.TestConnector {
	return []connectorsfixture.TestConnector{newMBConnector()}
}

func RTestConnectors() []connectorsfixture.TestConnector {
	return []connectorsfixture.TestConnector{newDBConnector()}
}

func newDBConnector() storagefixture.TestConnector {
	return storagefixture.NewTestConnector(applicationYAMLKey, TestConnectorsOrder)
}

func newMBConnector() messagebrokerfixture.TestConnector {
	return messagebrokerfixture.NewTestConnector(applicationYAMLKey, TestConnectorsOrder)
}

func RContainerMounts() []func(projectRoot string) testcontainers.ContainerMount {
	return nil
}

func WContainerMounts() []func(projectRoot string) testcontainers.ContainerMount {
	ip2LocBINName := "IP-COUNTRY-REGION-CITY-LATITUDE-LONGITUDE-ZIPCODE-TIMEZONE-ISP-DOMAIN-NETSPEED-AREACODE-WEATHER-MOBILE-ELEVATION-USAGETYPE-SAMPLE.BIN"

	return []func(projectRoot string) testcontainers.ContainerMount{func(projectRoot string) testcontainers.ContainerMount {
		ip2LocBINPath := fmt.Sprintf(`%vusers/internal/device/metadata/.testdata/%v`, projectRoot, ip2LocBINName)
		containerIP2LocBINPath := testcontainers.ContainerMountTarget(fmt.Sprintf(`/users/internal/device/metadata/.testdata/%v`, ip2LocBINName))

		return testcontainers.BindMount(ip2LocBINPath, containerIP2LocBINPath)
	}}
}
