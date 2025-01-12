package cmd

import (
	"context"

	dbplugin "github.com/hashicorp/vault/sdk/database/dbplugin/v5"
)

var _ dbplugin.Database = (*cmd)(nil)

type cmd struct {
	// sync.RWMutex

	// usernameProducer template.StringTemplate
	// credsutil.CredentialsProducer
}

func New() (interface{}, error) {
	db := new()

	// This middleware isn't strictly required, but highly recommended to prevent accidentally exposing
	// values such as passwords in error messages. An example of this is included below
	dbType := dbplugin.NewDatabaseErrorSanitizerMiddleware(db, db.secretValues)
	return dbType, nil
}

func new() *cmd {
	// ...
	db := &cmd{
		// ...
	}
	return db
}

func (db *cmd) secretValues() map[string]string {
	return map[string]string{
		"db.password": "[password]",
	}
}

func (db *cmd) Initialize(ctx context.Context, req dbplugin.InitializeRequest) (dbplugin.InitializeResponse, error) {
	return dbplugin.InitializeResponse{}, nil
}

func (db *cmd) NewUser(ctx context.Context, req dbplugin.NewUserRequest) (dbplugin.NewUserResponse, error) {
	return dbplugin.NewUserResponse{}, nil
}

func (db *cmd) UpdateUser(ctx context.Context, req dbplugin.UpdateUserRequest) (dbplugin.UpdateUserResponse, error) {
	return dbplugin.UpdateUserResponse{}, nil
}

func (db *cmd) DeleteUser(ctx context.Context, req dbplugin.DeleteUserRequest) (dbplugin.DeleteUserResponse, error) {
	return dbplugin.DeleteUserResponse{}, nil
}

func (db *cmd) Type() (string, error) {
	return "cmd", nil
}

func (db *cmd) Close() error {
	return nil
}
