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

package artifactmanager

import (
	"context"
	"testing"

	"github.com/gojuno/minimock"
	"github.com/insolar/insolar/configuration"
	"github.com/insolar/insolar/core"
	"github.com/insolar/insolar/core/message"
	"github.com/insolar/insolar/instrumentation/inslogger"
	"github.com/insolar/insolar/ledger/storage"
	"github.com/insolar/insolar/ledger/storage/jet"
	"github.com/insolar/insolar/ledger/storage/storagetest"
	"github.com/insolar/insolar/platformpolicy"
	"github.com/insolar/insolar/testutils"
	"github.com/insolar/insolar/testutils/testmessagebus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLedgerArtifactManager_PendingRequest(t *testing.T) {
	t.Parallel()
	ctx := inslogger.TestContext(t)
	mc := minimock.NewController(t)
	db, cleaner := storagetest.TmpDB(ctx, t)
	defer cleaner()
	defer mc.Finish()

	jetID := *jet.NewID(0, nil)

	amPulseStorageMock := testutils.NewPulseStorageMock(t)
	amPulseStorageMock.CurrentFunc = func(p context.Context) (r *core.Pulse, r1 error) {
		pulse, err := db.GetLatestPulse(p)
		require.NoError(t, err)
		return &pulse.Pulse, err
	}

	jcMock := testutils.NewJetCoordinatorMock(t)
	jcMock.LightExecutorForJetMock.Return(&core.RecordRef{}, nil)
	jcMock.MeMock.Return(core.RecordRef{})

	certificate := testutils.NewCertificateMock(t)
	certificate.GetRoleMock.Return(core.StaticRoleLightMaterial)

	cs := testutils.NewPlatformCryptographyScheme()
	mb := testmessagebus.NewTestMessageBus(t)
	mb.PulseStorage = amPulseStorageMock

	am := NewArtifactManger(db)
	am.PulseStorage = amPulseStorageMock
	am.PlatformCryptographyScheme = cs
	am.DefaultBus = mb
	provider := storage.NewRecentStorageProvider(0)

	cryptoScheme := platformpolicy.NewPlatformCryptographyScheme()

	handler := NewMessageHandler(db, &configuration.Ledger{
		LightChainLimit: 10,
	}, certificate)

	handler.PlatformCryptographyScheme = cryptoScheme
	handler.Bus = mb
	// handler.JetCoordinator = jc
	handler.RecentStorageProvider = provider
	handler.JetCoordinator = jcMock
	err := handler.Init(ctx)
	require.NoError(t, err)
	objRef := *genRandomRef(0)
	handler.CloseEarlyRequestCircuitBreakerForJet(ctx, jetID)

	err = db.UpdateJetTree(ctx, core.FirstPulseNumber, true, jetID)
	require.NoError(t, err)
	err = db.UpdateJetTree(ctx, core.FirstPulseNumber+1, true, jetID)
	require.NoError(t, err)

	// Register request
	reqID, err := am.RegisterRequest(ctx, objRef, &message.Parcel{Msg: &message.CallMethod{}, PulseNumber: core.FirstPulseNumber})
	require.NoError(t, err)

	// Change pulse.
	err = db.AddPulse(ctx, core.Pulse{PulseNumber: core.FirstPulseNumber + 1})
	require.NoError(t, err)

	// Should have pending request.
	has, err := am.HasPendingRequests(ctx, objRef)
	require.NoError(t, err)
	assert.True(t, has)

	// Register result.
	reqRef := *core.NewRecordRef(core.DomainID, *reqID)
	_, err = am.RegisterResult(ctx, objRef, reqRef, nil)
	require.NoError(t, err)

	// Should not have pending request.
	has, err = am.HasPendingRequests(ctx, objRef)
	require.NoError(t, err)
	assert.False(t, has)
}
