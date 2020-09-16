package customd

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/iov-one/weave/x/countdown"

	"github.com/iov-one/weave"
	"github.com/iov-one/weave-starter-kit/x/custom"
	"github.com/iov-one/weave/app"
	"github.com/iov-one/weave/coin"
	"github.com/iov-one/weave/errors"
	"github.com/iov-one/weave/migration"
	"github.com/iov-one/weave/orm"
	"github.com/iov-one/weave/store/iavl"
	"github.com/iov-one/weave/x"
	"github.com/iov-one/weave/x/batch"
	"github.com/iov-one/weave/x/cash"
	"github.com/iov-one/weave/x/cron"
	"github.com/iov-one/weave/x/multisig"
	"github.com/iov-one/weave/x/sigs"
	"github.com/iov-one/weave/x/utils"
	"github.com/iov-one/weave/x/validators"
)

// Authenticator returns authentication with multisigs
// and public key signatues
func Authenticator() x.Authenticator {
	return x.ChainAuth(sigs.Authenticate{}, multisig.Authenticate{})
}

// CashControl returns a controller for cash functions
func CashControl() cash.Controller {
	return cash.NewController(cash.NewBucket())
}

// Chain returns a chain of decorators, to handle authentication,
// fees, logging, and recovery
func Chain(authFn x.Authenticator, minFee coin.Coin) app.Decorators {

	return app.ChainDecorators(
		utils.NewLogging(),
		utils.NewRecovery(),
		utils.NewKeyTagger(),
		// on CheckTx, bad tx don't affect state
		utils.NewSavepoint().OnCheck(),
		sigs.NewDecorator(),
		multisig.NewDecorator(authFn),
		cash.NewFeeDecorator(authFn, CashControl()),
		batch.NewDecorator(),
		utils.NewSavepoint().OnDeliver(),
	)
}

// Router returns a default router
func Router(authFn x.Authenticator, issuer weave.Address) *app.Router {
	r := app.NewRouter()
	scheduler := cron.NewScheduler(CronTaskMarshaler)

	cash.RegisterRoutes(r, authFn, CashControl())
	countdown.RegisterRoutes(r, authFn, scheduler)
	sigs.RegisterRoutes(r, authFn)
	multisig.RegisterRoutes(r, authFn)
	migration.RegisterRoutes(r, authFn)
	validators.RegisterRoutes(r, authFn)
	custom.RegisterRoutes(r, authFn, scheduler)
	return r
}

// QueryRouter returns a default query router,
// allowing access to "/custom", "/auth", "/contracts", "/wallets", "/validators" and "/"
func QueryRouter() weave.QueryRouter {
	r := weave.NewQueryRouter()
	r.RegisterAll(
		cash.RegisterQuery,
		countdown.RegisterQuery,
		sigs.RegisterQuery,
		multisig.RegisterQuery,
		migration.RegisterQuery,
		orm.RegisterQuery,
		validators.RegisterQuery,
		custom.RegisterQuery,
	)
	return r
}

// Stack wires up a standard router with a standard decorator
// chain. This can be passed into BaseApp.
func Stack(issuer weave.Address, minFee coin.Coin) weave.Handler {
	authFn := Authenticator()
	return Chain(authFn, minFee).WithHandler(Router(authFn, issuer))
}

// CronStack wires up a standard router with a cron specific decorator chain.
// This can be passed into BaseApp.
// Cron stack configuration is a subset of the main stack. It is using the same
// components but not all functionalities are needed or expected (ie no message
// fee).
func CronStack() weave.Handler {
	rt := app.NewRouter()

	authFn := cron.Authenticator{}

	// Cron is using custom router as not the same handlers are registered.
	custom.RegisterCronRoutes(rt, authFn)
	countdown.RegisterCronRoutes(rt, authFn)

	decorators := app.ChainDecorators(
		utils.NewLogging(),
		utils.NewRecovery(),
		utils.NewKeyTagger(),
		utils.NewActionTagger(),
		// No fee decorators.
	)
	return decorators.WithHandler(rt)
}

// CommitKVStore returns an initialized KVStore that persists
// the data to the named path.
func CommitKVStore(dbPath string) (weave.CommitKVStore, error) {
	// memory backed case, just for testing
	if dbPath == "" {
		return iavl.MockCommitStore(), nil
	}

	// Expand the path fully
	path, err := filepath.Abs(dbPath)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrDatabase, "invalid database name: %s", path)
	}

	// Some external calls accidentally add a ".db", which is now removed
	path = strings.TrimSuffix(path, filepath.Ext(path))

	// Split the database name into it's components (dir, name)
	dir := filepath.Dir(path)
	name := filepath.Base(path)
	return iavl.NewCommitStore(dir, name), nil
}

// Application constructs a basic ABCI application with
// the given arguments.
func Application(name string, h weave.Handler,
	tx weave.TxDecoder, dbPath string, debug bool) (app.BaseApp, error) {

	ctx := context.Background()
	kv, err := CommitKVStore(dbPath)
	if err != nil {
		return app.BaseApp{}, errors.Wrap(err, "cannot create database instance")
	}
	store := app.NewStoreApp(name, kv, QueryRouter(), ctx)
	ticker := cron.NewTicker(CronStack(), CronTaskMarshaler)
	base := app.NewBaseApp(store, tx, h, ticker, debug)
	return base, nil
}
