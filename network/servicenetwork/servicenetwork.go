/*
 *    Copyright 2018 Insolar
 *
 *    Licensed under the Apache License, Version 2.0 (the "License");
 *    you may not use this file except in compliance with the License.
 *    You may obtain a copy of the License at
 *
 *        http://www.apache.org/licenses/LICENSE-2.0
 *
 *    Unless required by applicable law or agreed to in writing, software
 *    distributed under the License is distributed on an "AS IS" BASIS,
 *    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *    See the License for the specific language governing permissions and
 *    limitations under the License.
 */

package servicenetwork

import (
	"context"
	"strconv"
	"strings"

	"github.com/insolar/insolar/component"
	"github.com/insolar/insolar/configuration"
	"github.com/insolar/insolar/consensus/phases"
	"github.com/insolar/insolar/core"
	"github.com/insolar/insolar/instrumentation/inslogger"
	"github.com/insolar/insolar/log"
	"github.com/insolar/insolar/network"
	"github.com/insolar/insolar/network/controller"
	"github.com/insolar/insolar/network/controller/bootstrap"
	"github.com/insolar/insolar/network/hostnetwork"
	"github.com/insolar/insolar/network/merkle"
	"github.com/insolar/insolar/network/routing"
	"github.com/pkg/errors"
)

// ServiceNetwork is facade for network.
type ServiceNetwork struct {
	cfg configuration.Configuration
	cm  *component.Manager

	hostNetwork  network.HostNetwork  // TODO: should be injected
	routingTable network.RoutingTable // TODO: should be injected

	// dependencies
	CertificateManager  core.CertificateManager         `inject:""`
	PulseManager        core.PulseManager               `inject:""`
	PulseStorage        core.PulseStorage               `inject:""`
	CryptographyService core.CryptographyService        `inject:""`
	NetworkCoordinator  core.NetworkCoordinator         `inject:""`
	CryptographyScheme  core.PlatformCryptographyScheme `inject:""`
	NodeKeeper          network.NodeKeeper              `inject:""`
	NetworkSwitcher     core.NetworkSwitcher            `inject:""`

	// subcomponents
	PhaseManager phases.PhaseManager `inject:"subcomponent"`
	Controller   network.Controller  `inject:"subcomponent"`

	// fakePulsar *fakepulsar.FakePulsar
	isGenesis bool
	skip      int
}

// NewServiceNetwork returns a new ServiceNetwork.
func NewServiceNetwork(conf configuration.Configuration, scheme core.PlatformCryptographyScheme, rootCm *component.Manager, isGenesis bool) (*ServiceNetwork, error) {
	serviceNetwork := &ServiceNetwork{cm: component.NewManager(rootCm), cfg: conf, CryptographyScheme: scheme, isGenesis: isGenesis, skip: conf.Service.Skip}
	return serviceNetwork, nil
}

// SendMessage sends a message from MessageBus.
func (n *ServiceNetwork) SendMessage(nodeID core.RecordRef, method string, msg core.Parcel) ([]byte, error) {
	return n.Controller.SendMessage(nodeID, method, msg)
}

// SendCascadeMessage sends a message from MessageBus to a cascade of nodes
func (n *ServiceNetwork) SendCascadeMessage(data core.Cascade, method string, msg core.Parcel) error {
	return n.Controller.SendCascadeMessage(data, method, msg)
}

// RemoteProcedureRegister registers procedure for remote call on this host.
func (n *ServiceNetwork) RemoteProcedureRegister(name string, method core.RemoteProcedure) {
	n.Controller.RemoteProcedureRegister(name, method)
}

// incrementPort increments port number if it not equals 0
func incrementPort(address string) (string, error) {
	parts := strings.Split(address, ":")
	if len(parts) < 2 {
		return address, errors.New("failed to get port from address")
	}
	port, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return address, err
	}

	if port != 0 {
		port++
	}

	parts = append(parts[:len(parts)-1], strconv.Itoa(port))
	return strings.Join(parts, ":"), nil
}

// Start implements component.Initer
func (n *ServiceNetwork) Init(ctx context.Context) error {
	n.routingTable = &routing.Table{}
	internalTransport, err := hostnetwork.NewInternalTransport(n.cfg, n.CertificateManager.GetCertificate().GetNodeRef().String())
	if err != nil {
		return errors.Wrap(err, "Failed to create internal transport")
	}

	// workaround for Consensus transport, port+=1 of default transport
	n.cfg.Host.Transport.Address, err = incrementPort(n.cfg.Host.Transport.Address)
	if err != nil {
		return errors.Wrap(err, "failed to increment port.")
	}

	consensusNetwork, err := hostnetwork.NewConsensusNetwork(
		n.cfg.Host.Transport.Address,
		n.CertificateManager.GetCertificate().GetNodeRef().String(),
		n.NodeKeeper.GetOrigin().ShortID(),
		n.routingTable,
	)
	if err != nil {
		return errors.Wrap(err, "Failed to create consensus network.")
	}

	n.hostNetwork = hostnetwork.NewHostTransport(internalTransport, n.routingTable)
	options := controller.ConfigureOptions(n.cfg.Host)

	n.cm.Inject(n,
		n.CertificateManager.GetCertificate(),
		n.NodeKeeper,
		merkle.NewCalculator(),
		consensusNetwork,
		phases.NewNaiveCommunicator(),
		phases.NewFirstPhase(),
		phases.NewSecondPhase(),
		phases.NewThirdPhase(),
		phases.NewPhaseManager(),
		bootstrap.NewSessionManager(),
		controller.NewNetworkController(n.hostNetwork),
		controller.NewRPCController(options, n.hostNetwork),
		controller.NewPulseController(n.hostNetwork, n.routingTable),
		bootstrap.NewBootstrapper(options, internalTransport),
		bootstrap.NewAuthorizationController(options, internalTransport),
		bootstrap.NewChallengeResponseController(options, internalTransport),
		bootstrap.NewNetworkBootstrapper(),
	)

	// n.fakePulsar = fakepulsar.NewFakePulsar(n.HandlePulse, n.cfg.Pulsar.PulseTime)
	return nil
}

// Start implements component.Starter
func (n *ServiceNetwork) Start(ctx context.Context) error {
	log.Infoln("Network starts listening...")
	n.hostNetwork.Start(ctx)
	n.routingTable.Inject(n.NodeKeeper)

	log.Info("Starting network component manager...")
	err := n.cm.Start(ctx)
	if err != nil {
		return errors.Wrap(err, "Failed to bootstrap network")
	}

	log.Infoln("Bootstrapping network...")
	err = n.Controller.Bootstrap(ctx)
	if err != nil {
		return errors.Wrap(err, "Failed to bootstrap network")
	}

	// n.fakePulsar.Start(ctx)

	return nil
}

// Stop implements core.Component
func (n *ServiceNetwork) Stop(ctx context.Context) error {
	logger := inslogger.FromContext(ctx)

	logger.Info("Stopping network components")
	if err := n.cm.Stop(ctx); err != nil {
		log.Errorf("Error while stopping network components: %s", err.Error())
	}
	logger.Info("Stopping host network")
	n.hostNetwork.Stop()
	return nil
}

func (n *ServiceNetwork) HandlePulse(ctx context.Context, pulse core.Pulse) {
	// if !n.isFakePulse(&pulse) {
	// 	n.fakePulsar.Stop(ctx)
	// }
	if n.isGenesis {
		return
	}

	traceID := "pulse_" + strconv.FormatUint(uint64(pulse.PulseNumber), 10)

	ctx, logger := inslogger.WithTraceField(ctx, traceID)
	logger.Infof("Got new pulse number: %d", pulse.PulseNumber)
	if n.PulseManager == nil {
		logger.Error("PulseManager is not initialized")
		return
	}
	if !n.NodeKeeper.IsBootstrapped() {
		n.Controller.SetLastIgnoredPulse(pulse.NextPulseNumber)
		return
	}
	if pulse.PulseNumber <= n.Controller.GetLastIgnoredPulse()+core.PulseNumber(n.skip) {
		log.Infof("Ignore pulse %d: network is not yet initialized", pulse.PulseNumber)
		return
	}
	currentPulse, err := n.PulseStorage.Current(ctx)
	if err != nil {
		logger.Error(errors.Wrap(err, "Could not get current pulse"))
		return
	}
	if (pulse.PulseNumber > currentPulse.PulseNumber) &&
		(pulse.PulseNumber >= currentPulse.NextPulseNumber) {

		err = n.NetworkSwitcher.OnPulse(ctx, pulse)
		if err != nil {
			logger.Error(errors.Wrap(err, "Failed to call OnPulse on NetworkSwitcher"))
			return
		}

		err = n.PulseManager.Set(ctx, pulse, n.NetworkSwitcher.GetState() == core.CompleteNetworkState)
		if err != nil {
			logger.Error(errors.Wrap(err, "Failed to set pulse"))
			return
		}

		logger.Infof("Set new current pulse number: %d", pulse.PulseNumber)
		// go func(logger core.Logger, network *ServiceNetwork) {
		// 	TODO: make PhaseManager works and uncomment this (after NETD18-75)
		// 	err = n.PhaseManager.OnPulse(ctx, &pulse)
		// 	if err != nil {
		// 		logger.Warn("phase manager fail: " + err.Error())
		// 	}
		// }(logger, n)
	} else {
		logger.Infof("Incorrect pulse number. Current: %d. New: %d", currentPulse.PulseNumber, pulse.PulseNumber)
	}
}

// func (n *ServiceNetwork) isFakePulse(pulse *core.Pulse) bool {
// 	return (pulse.NextPulseNumber == 0) && (pulse.PulseNumber == 0)
// }
