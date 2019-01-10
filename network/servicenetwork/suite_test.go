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
	"crypto"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/insolar/insolar/certificate"
	"github.com/insolar/insolar/component"
	"github.com/insolar/insolar/configuration"
	"github.com/insolar/insolar/consensus/packets"
	"github.com/insolar/insolar/core"
	"github.com/insolar/insolar/cryptography"
	"github.com/insolar/insolar/log"
	"github.com/insolar/insolar/network"
	"github.com/insolar/insolar/network/nodenetwork"
	"github.com/insolar/insolar/network/utils"
	"github.com/insolar/insolar/platformpolicy"
	"github.com/insolar/insolar/testutils"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

var testNetworkPort = 10010

type fixture struct {
	ctx            context.Context
	bootstrapNodes []*networkNode
	networkNodes   []*networkNode
	testNode       *networkNode
}

func newFixture() *fixture {
	return &fixture{
		ctx:            context.Background(),
		bootstrapNodes: make([]*networkNode, 0),
		networkNodes:   make([]*networkNode, 0),
	}
}

type testSuite struct {
	suite.Suite
	fixtureMap     map[string]*fixture
	bootstrapCount int
	nodesCount     int
}

func NewTestSuite(bootstrapCount, nodesCount int) *testSuite {
	return &testSuite{
		Suite:          suite.Suite{},
		fixtureMap:     make(map[string]*fixture, 0),
		bootstrapCount: bootstrapCount,
		nodesCount:     nodesCount,
	}
}

func (s *testSuite) fixture() *fixture {
	return s.fixtureMap[s.T().Name()]
}

// SetupSuite creates and run network with bootstrap and common nodes once before run all tests in the suite
func (s *testSuite) SetupTest() {
	s.fixtureMap[s.T().Name()] = newFixture()

	log.Infoln("SetupTest")

	for i := 0; i < s.bootstrapCount; i++ {
		s.fixture().bootstrapNodes = append(s.fixture().bootstrapNodes, newNetworkNode())
	}

	for i := 0; i < s.nodesCount; i++ {
		s.fixture().networkNodes = append(s.fixture().networkNodes, newNetworkNode())
	}

	s.fixture().testNode = newNetworkNode()

	log.Infoln("Setup bootstrap nodes")
	s.SetupNodesNetwork(s.fixture().bootstrapNodes)

	<-time.After(time.Second * 2)
	activeNodes := s.fixture().bootstrapNodes[0].serviceNetwork.NodeKeeper.GetActiveNodes()
	require.Equal(s.T(), len(s.fixture().bootstrapNodes), len(activeNodes))

	if len(s.fixture().networkNodes) > 0 {
		log.Infoln("Setup network nodes")
		s.SetupNodesNetwork(s.fixture().networkNodes)
		s.waitForConsensus(2)

		// active nodes count verification
		activeNodes1 := s.fixture().networkNodes[0].serviceNetwork.NodeKeeper.GetActiveNodes()
		activeNodes2 := s.fixture().networkNodes[0].serviceNetwork.NodeKeeper.GetActiveNodes()

		require.Equal(s.T(), s.getNodesCount(), len(activeNodes1))
		require.Equal(s.T(), s.getNodesCount(), len(activeNodes2))
	}
	fmt.Println("=================== SetupTest() Done")
}

func (s *testSuite) SetupNodesNetwork(nodes []*networkNode) {
	for _, node := range nodes {
		s.preInitNode(node)
	}

	results := make(chan error, len(nodes))
	initNode := func(node *networkNode) {
		err := node.init(s.fixture().ctx)
		results <- err
	}
	startNode := func(node *networkNode) {
		err := node.componentManager.Start(s.fixture().ctx)
		results <- err
	}

	waitResults := func(results chan error, expected int) error {
		count := 0
		for {
			select {
			case err := <-results:
				count++
				s.NoError(err)
				if count == expected {
					return nil
				}
			case <-time.After(time.Second * 5):
				return errors.New("timeout")
			}
		}
	}

	log.Infoln("Init nodes")
	for _, node := range nodes {
		go initNode(node)
	}

	err := waitResults(results, len(nodes))
	s.NoError(err)

	log.Infoln("Start nodes")
	for _, node := range nodes {
		go startNode(node)
	}

	err = waitResults(results, len(nodes))
	s.NoError(err)
}

// TearDownSuite shutdowns all nodes in network, calls once after all tests in suite finished
func (s *testSuite) TearDownTest() {
	log.Info("=================== TearDownTest()")
	log.Infoln("Stop network nodes")
	for _, n := range s.fixture().networkNodes {
		err := n.componentManager.Stop(s.fixture().ctx)
		s.NoError(err)
	}
	log.Infoln("Stop bootstrap nodes")
	for _, n := range s.fixture().bootstrapNodes {
		err := n.componentManager.Stop(s.fixture().ctx)
		s.NoError(err)

	}
}

func (s *testSuite) waitForConsensus(consensusCount int) {
	for i := 0; i < consensusCount; i++ {
		for _, n := range s.fixture().bootstrapNodes {
			err := <-n.consensusResult
			s.NoError(err)
		}

		for _, n := range s.fixture().networkNodes {
			err := <-n.consensusResult
			s.NoError(err)
		}
	}
}

func (s *testSuite) waitForConsensusExcept(consensusCount int, exception core.RecordRef) {
	for i := 0; i < consensusCount; i++ {
		for _, n := range s.fixture().bootstrapNodes {
			if n.id.Equal(exception) {
				continue
			}
			err := <-n.consensusResult
			s.NoError(err)
		}

		for _, n := range s.fixture().networkNodes {
			if n.id.Equal(exception) {
				continue
			}
			err := <-n.consensusResult
			s.NoError(err)
		}
	}
}

// nodesCount returns count of nodes in network without testNode
func (s *testSuite) getNodesCount() int {
	return len(s.fixture().bootstrapNodes) + len(s.fixture().networkNodes)
}

type PhaseTimeOut uint8

const (
	Disable = PhaseTimeOut(iota + 1)
	Partial
	Full
)

func (s *testSuite) InitTestNode() {
	if s.fixture().testNode.componentManager != nil {
		err := s.fixture().testNode.init(s.fixture().ctx)
		s.NoError(err)
	}
}

func (s *testSuite) StartTestNode() {
	if s.fixture().testNode.componentManager != nil {
		err := s.fixture().testNode.componentManager.Start(s.fixture().ctx)
		s.NoError(err)
	}
}

func (s *testSuite) StopTestNode() {
	if s.fixture().testNode.componentManager != nil {
		err := s.fixture().testNode.componentManager.Stop(s.fixture().ctx)
		s.NoError(err)
	}
}

type networkNode struct {
	id                  core.RecordRef
	role                core.StaticRole
	privateKey          crypto.PrivateKey
	cryptographyService core.CryptographyService
	host                string

	componentManager *component.Manager
	serviceNetwork   *ServiceNetwork
	consensusResult  chan error
}

// newNetworkNode returns networkNode initialized only with id, host address and key pair
func newNetworkNode() *networkNode {
	key, err := platformpolicy.NewKeyProcessor().GeneratePrivateKey()
	if err != nil {
		panic(err.Error())
	}
	address := "127.0.0.1:" + strconv.Itoa(testNetworkPort)
	testNetworkPort += 2 // coz consensus transport port+=1

	return &networkNode{
		id:                  testutils.RandomRef(),
		role:                RandomRole(),
		privateKey:          key,
		cryptographyService: cryptography.NewKeyBoundCryptographyService(key),
		host:                address,
		consensusResult:     make(chan error, 30),
	}
}

// init calls Init for node component manager and wraps PhaseManager
func (n *networkNode) init(ctx context.Context) error {
	err := n.componentManager.Init(ctx)
	n.serviceNetwork.PhaseManager = &phaseManagerWrapper{original: n.serviceNetwork.PhaseManager, result: n.consensusResult}
	n.serviceNetwork.NodeKeeper = &nodeKeeperWrapper{original: n.serviceNetwork.NodeKeeper}
	return err
}

func (s *testSuite) initCrypto(node *networkNode) (*certificate.CertificateManager, core.CryptographyService) {
	pubKey, err := node.cryptographyService.GetPublicKey()
	s.NoError(err)

	// init certificate

	proc := platformpolicy.NewKeyProcessor()
	publicKey, err := proc.ExportPublicKeyPEM(pubKey)
	s.NoError(err)

	cert := &certificate.Certificate{}
	cert.PublicKey = string(publicKey[:])
	cert.Reference = node.id.String()
	cert.Role = node.role.String()
	cert.BootstrapNodes = make([]certificate.BootstrapNode, 0)

	for _, b := range s.fixture().bootstrapNodes {
		pubKey, _ := b.cryptographyService.GetPublicKey()
		pubKeyBuf, err := proc.ExportPublicKeyPEM(pubKey)
		s.NoError(err)

		bootstrapNode := certificate.NewBootstrapNode(
			pubKey,
			string(pubKeyBuf[:]),
			b.host,
			b.id.String())

		cert.BootstrapNodes = append(cert.BootstrapNodes, *bootstrapNode)
	}

	// dump cert and read it again from json for correct private files initialization
	jsonCert, err := cert.Dump()
	s.NoError(err)
	log.Infof("cert: %s", jsonCert)

	cert, err = certificate.ReadCertificateFromReader(pubKey, proc, strings.NewReader(jsonCert))
	s.NoError(err)
	return certificate.NewCertificateManager(cert), node.cryptographyService
}

func RandomRole() core.StaticRole {
	i := rand.Int()%3 + 1
	return core.StaticRole(i)
}

// preInitNode inits previously created node with mocks and external dependencies
func (s *testSuite) preInitNode(node *networkNode) {
	cfg := configuration.NewConfiguration()
	cfg.Pulsar.PulseTime = 5000 // pulse 5 sec for faster tests
	cfg.Host.Transport.Address = node.host

	scheme := platformpolicy.NewPlatformCryptographyScheme()
	serviceNetwork, err := NewServiceNetwork(cfg, scheme)
	s.NoError(err)

	pulseStorageMock := testutils.NewPulseStorageMock(s.T())
	pulseStorageMock.CurrentMock.Set(func(p context.Context) (r *core.Pulse, r1 error) {
		return &core.Pulse{PulseNumber: 0}, nil

	})

	pulseManagerMock := testutils.NewPulseManagerMock(s.T())

	netCoordinator := testutils.NewNetworkCoordinatorMock(s.T())
	netCoordinator.ValidateCertMock.Set(func(p context.Context, p1 core.AuthorizationCertificate) (bool, error) {
		return true, nil
	})
	netCoordinator.WriteActiveNodesMock.Set(func(p context.Context, p1 core.PulseNumber, p2 []core.Node) (r error) {
		return nil
	})

	netCoordinator.IsStartedMock.Set(func() (r bool) {
		return true
	})

	amMock := testutils.NewArtifactManagerMock(s.T())
	amMock.StateMock.Set(func() (r []byte, r1 error) {
		return make([]byte, packets.HashLength), nil
	})

	pubKey, _ := node.cryptographyService.GetPublicKey()

	origin := nodenetwork.NewNode(node.id, node.role, pubKey, node.host, "")
	certManager, cryptographyService := s.initCrypto(node)
	netSwitcher := testutils.NewNetworkSwitcherMock(s.T())
	netSwitcher.GetStateMock.Set(func() (r core.NetworkState) {
		return core.VoidNetworkState
	})

	realKeeper := nodenetwork.NewNodeKeeper(origin)

	realKeeper.SetState(network.Waiting)
	if len(certManager.GetCertificate().GetDiscoveryNodes()) == 0 || utils.OriginIsDiscovery(certManager.GetCertificate()) {
		realKeeper.SetState(network.Ready)
		realKeeper.AddActiveNodes([]core.Node{origin})
	}
	realKeeper.SetExitHandler(func() {
		log.Info("node exited, bye bye")
	})

	node.componentManager = &component.Manager{}
	node.componentManager.Register(realKeeper, pulseManagerMock, pulseStorageMock, netCoordinator, amMock)
	node.componentManager.Register(certManager, cryptographyService)
	node.componentManager.Inject(serviceNetwork, netSwitcher)
	node.serviceNetwork = serviceNetwork

	pulseManagerMock.SetMock.Set(func(p context.Context, p1 core.Pulse, p2 bool) (r error) {
		if serviceNetwork.NodeKeeper == nil {
			panic("NodeKeeper == nil")
		}
		return serviceNetwork.NodeKeeper.MoveSyncToActive()
	})
}