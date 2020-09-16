package customd

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/iov-one/weave"
	"github.com/iov-one/weave/app"
	"github.com/iov-one/weave/coin"
	"github.com/iov-one/weave/commands/server"
	"github.com/iov-one/weave/crypto"
	"github.com/iov-one/weave/migration"
	"github.com/iov-one/weave/x/cash"
	"github.com/iov-one/weave/x/countdown"
	"github.com/iov-one/weave/x/multisig"
	"github.com/iov-one/weave/x/validators"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
)

// GenInitOptions will produce some basic options for one rich
// account, to use for dev mode
func GenInitOptions(args []string) (json.RawMessage, error) {
	// Your coins ticker code
	ticker := "CSTM"
	if len(args) > 0 {
		ticker = args[0]
		if !coin.IsCC(ticker) {
			return nil, fmt.Errorf("Invalid ticker %s", ticker)
		}
	}

	var addr string
	if len(args) > 1 {
		addr = args[1]
	} else {
		// if no address provided, auto-generate one
		// and print out a recovery phrase
		bz, phrase, err := GenerateCoinKey()
		if err != nil {
			return nil, err
		}
		addr = hex.EncodeToString(bz)
		fmt.Println(phrase)
	}

	type (
		dict  map[string]interface{}
		array []interface{}
	)

	cond1 := weave.NewCondition("sigs", "ed25519", []byte{1, 2, 3})
	// collectorAddr is the address where all tx fee's will be
	// stashed and then distributed to stakeholders
	collectorAddr := cond1.Address()

	return json.Marshal(dict{
		"cash": array{
			dict{
				"address": addr,
				"coins": array{
					dict{
						"whole":  123456789,
						"ticker": ticker,
					},
				},
			},
		},
		"conf": dict{
			"cash": dict{
				"collector_address": collectorAddr,
				"minimal_fee":       coin.Coin{Whole: 0}, // no fee
			},
			"migration": dict{
				// admin is who can change this redistribution address to other address
				"admin": addr,
			},
		},
		"initialize_schema": []dict{
			{"pkg": "migration", "ver": 1},
			{"pkg": "countdown", "ver": 1},
			{"pkg": "custom", "ver": 1},
			{"pkg": "cash", "ver": 1},
			{"pkg": "sigs", "ver": 1},
			{"pkg": "multisig", "ver": 1},
			{"pkg": "utils", "ver": 1},
			{"pkg": "validators", "ver": 1},
		},
	})
}

// GenerateApp is used to create a stub for server/start.go command
func GenerateApp(options *server.Options) (abci.Application, error) {
	// db goes in a subdir, but "" -> "" for memdb
	var dbPath string
	if options.Home != "" {
		dbPath = filepath.Join(options.Home, "custom.db")
	}

	stack := Stack(nil, options.MinFee)
	application, err := Application("customd", stack, TxDecoder, dbPath, options.Debug)
	if err != nil {
		return nil, err
	}

	return DecorateApp(application, options.Logger), nil
}

// DecorateApp adds initializers and Logger to an Application
func DecorateApp(application app.BaseApp, logger log.Logger) app.BaseApp {
	application.WithInit(app.ChainInitializers(
		&migration.Initializer{},
		&cash.Initializer{},
		&countdown.Initializer{},
		&multisig.Initializer{},
		&validators.Initializer{},
	))
	application.WithLogger(logger)
	return application
}

// InlineApp will take a previously prepared CommitStore and return a complete Application
func InlineApp(kv weave.CommitKVStore, logger log.Logger, debug bool) abci.Application {
	minFee := coin.Coin{}
	stack := Stack(nil, minFee)
	ctx := context.Background()
	store := app.NewStoreApp("customd", kv, QueryRouter(), ctx)
	base := app.NewBaseApp(store, TxDecoder, stack, nil, debug)
	return DecorateApp(base, logger)
}

type output struct {
	Pubkey *crypto.PublicKey  `json:"pub_key"`
	Secret *crypto.PrivateKey `json:"secret"`
}

// GenerateCoinKey returns the address of a public key,
// along with the secret phrase to recover the private key.
// You can give coins to this address and return the recovery
// phrase to the user to access them.
func GenerateCoinKey() (weave.Address, string, error) {
	privKey := crypto.GenPrivKeyEd25519()
	pubKey := privKey.PublicKey()
	addr := pubKey.Address()

	out := output{Pubkey: pubKey, Secret: privKey}
	keys, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, "", err
	}

	return addr, string(keys), nil
}
