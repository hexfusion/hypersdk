package vm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/ava-labs/avalanchego/api/metrics"
	"github.com/ava-labs/avalanchego/database/manager"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	avago_version "github.com/ava-labs/avalanchego/version"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/pebble"
	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/vm/controller"
	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/vm/genesis"
	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/vm/rpc"
)

const (
	dbFolder = ".simulator/db"
)

func TestController(t *testing.T) {
	require := require.New(t)
	homeDir, err := os.UserHomeDir()
	require.NoError(err)
	dbPath := path.Join(homeDir, dbFolder)

	nodeID := ids.GenerateTestNodeID()
	networkID := uint32(1)
	subnetID := ids.GenerateTestID()
	chainID := ids.GenerateTestID()
	app := &appSender{}

	loggingConfig := logging.Config{}
	loggingConfig.LogLevel = logging.Debug
	logFactory := logging.NewFactory(loggingConfig)
	log, err := logFactory.Make(nodeID.String())
	require.NoError(err)
	defer logFactory.Close()

	sk, err := bls.NewSecretKey()
	require.NoError(err)

	pdb, _, err := pebble.New(dbPath, pebble.NewDefaultConfig())
	require.NoError(err)
	db, err := manager.NewManagerFromDBs([]*manager.VersionedDatabase{
		{
			Database: pdb,
			Version:  avago_version.CurrentDatabase,
		},
	})
	require.NoError(err)

	genesisBytes, err := json.Marshal(genesis.Default())
	require.NoError(err)

	snowCtx := &snow.Context{
		NetworkID:    networkID,
		SubnetID:     subnetID,
		ChainID:      chainID,
		NodeID:       nodeID,
		Log:          log,
		ChainDataDir: dbPath,
		Metrics:      metrics.NewOptionalGatherer(),
		PublicKey:    bls.PublicFromSecretKey(sk),
	}

	toEngine := make(chan common.Message, 1)
	c := controller.New()
	defer c.Shutdown(context.Background())
	err = c.Initialize(
		context.TODO(),
		snowCtx,
		db,
		genesisBytes,
		nil,
		nil,
		toEngine,
		nil,
		app,
	)
	require.NoError(err)

	handlers, err := c.CreateHandlers(context.Background())
	require.NoError(err)

	client := rpc.NewJSONRPCClient("http://localhost:9650", networkID, chainID, handlers)

	resp, err := client.Genesis(context.Background())
	require.NoError(err)

	c.GetTransaction(context.Background(), resp.Genesis.Txs[0].ID())

	fmt.Printf("resp: %v\n", resp)

}

var _ common.AppSender = &appSender{}

type appSender struct{}

func (app *appSender) SendAppGossip(ctx context.Context, appGossipBytes []byte) error {
	return nil
}

func (*appSender) SendAppRequest(context.Context, set.Set[ids.NodeID], uint32, []byte) error {
	return nil
}

func (*appSender) SendAppResponse(context.Context, ids.NodeID, uint32, []byte) error {
	return nil
}

func (*appSender) SendAppGossipSpecific(context.Context, set.Set[ids.NodeID], []byte) error {
	return nil
}

func (*appSender) SendCrossChainAppRequest(context.Context, ids.ID, uint32, []byte) error {
	return nil
}

func (*appSender) SendCrossChainAppResponse(context.Context, ids.ID, uint32, []byte) error {
	return nil
}
