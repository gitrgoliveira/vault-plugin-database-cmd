package cmd

import (
	"context"
	"crypto/tls"
	"fmt"
	"os/exec"
	"strings"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	dbplugin "github.com/hashicorp/vault/sdk/database/dbplugin/v5"
	"github.com/hashicorp/vault/sdk/database/helper/credsutil"
	identity "github.com/hashicorp/vault/sdk/helper/pluginidentityutil"
	"github.com/mitchellh/mapstructure"
)

const (
	defaultTimeout = 20000 * time.Millisecond
	maxKeyLength   = 64
)

var _ dbplugin.Database = (*cmd)(nil)

type cmd struct {
	Logger hclog.Logger

	Config map[string]string
	credsutil.CredentialsProducer

	RawConfig map[string]interface{}

	RootTLSConfig *tls.Config

	RootUsername    string `json:"username"`
	RootPassword    string `json:"password"`
	RootCertificate string `json:"certificate"`

	identity.PluginIdentityTokenParams
}

func New() (interface{}, error) {
	db := new()

	// This middleware isn't strictly required, but highly recommended to prevent accidentally exposing
	// values such as passwords in error messages.
	dbType := dbplugin.NewDatabaseErrorSanitizerMiddleware(db, db.secretValues)
	return dbType, nil
}

func new() *cmd {
	db := &cmd{}
	db.Logger = hclog.New(&hclog.LoggerOptions{})
	return db
}

func (db *cmd) secretValues() map[string]string {
	return map[string]string{
		"password": "[password]",
	}
}

func (db *cmd) Initialize(ctx context.Context, req dbplugin.InitializeRequest) (dbplugin.InitializeResponse, error) {
	db.Logger = hclog.New(&hclog.LoggerOptions{})

	db.Logger.Info("Initialize", "config", req.Config)

	db.RawConfig = req.Config
	decoderConfig := &mapstructure.DecoderConfig{
		Result:           db,
		WeaklyTypedInput: true,
		TagName:          "json",
	}

	decoder, err := mapstructure.NewDecoder(decoderConfig)
	if err != nil {
		return dbplugin.InitializeResponse{}, err
	}

	err = decoder.Decode(req.Config)
	if err != nil {
		return dbplugin.InitializeResponse{}, err
	}

	resp := dbplugin.InitializeResponse{
		Config: req.Config,
	}

	resp.SetSupportedCredentialTypes([]dbplugin.CredentialType{
		dbplugin.CredentialTypePassword,
	})

	return resp, nil
}

func (db *cmd) NewUser(ctx context.Context, req dbplugin.NewUserRequest) (dbplugin.NewUserResponse, error) {
	db.Logger.Info("NewUser", "statements", req.Statements.Commands)

	// These statements are not used. They are only for the purpose of this example.
	db.Logger.Info("NewUser", "rollback_statements", req.RollbackStatements.Commands)

	if req.CredentialType == dbplugin.CredentialTypePassword {

		username, err := credsutil.GenerateUsername(
			credsutil.DisplayName(req.UsernameConfig.DisplayName, maxKeyLength),
			credsutil.RoleName(req.UsernameConfig.RoleName, maxKeyLength))

		if err != nil {
			return dbplugin.NewUserResponse{}, fmt.Errorf("failed to generate username: %w", err)
		}

		db.Logger.Info("NewUser", "future_username", username)

		// Assemble rollback statements into a single script
		script := strings.Join(req.Statements.Commands, "\n")
		params := map[string]string{
			"name":     username,
			"username": username,
			"password": req.Password,
		}

		renderedScript := replaceVars(params, script)

		// Execute the script
		cmd := exec.Command("/bin/bash", "-c", renderedScript)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return dbplugin.NewUserResponse{}, fmt.Errorf("failed to execute rollback script: %s, error: %w", script, err)
		}
		db.Logger.Info("Executed creation script", "script", script, "output", string(output))

		return dbplugin.NewUserResponse{
			Username: username,
		}, nil

	}

	return dbplugin.NewUserResponse{}, fmt.Errorf("only 'password' credential type is supported. Got %s ", req.CredentialType)
}

func (db *cmd) UpdateUser(ctx context.Context, req dbplugin.UpdateUserRequest) (dbplugin.UpdateUserResponse, error) {
	if req.CredentialType == dbplugin.CredentialTypePassword {
		db.Logger.Info("UpdateUser", "username", req.Username)
		db.Logger.Info("UpdateUser", "password_statements", req.Password.Statements.Commands)

		// These statements are not used. They are only for the purpose of this example.
		if req.Expiration != nil && req.Expiration.Statements.Commands != nil {
			db.Logger.Info("UpdateUser", "expiration_statements", req.Expiration.Statements.Commands)
		}

		// Assemble password change statements into a single script
		script := strings.Join(req.Password.Statements.Commands, "\n")
		params := map[string]string{
			"name":     req.Username,
			"username": req.Username,
			"password": req.Password.NewPassword,
		}
		renderedScript := replaceVars(params, script)

		// Execute the script
		cmd := exec.Command("/bin/bash", "-c", renderedScript)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return dbplugin.UpdateUserResponse{}, fmt.Errorf("failed to execute password change script: %s, error: %w", script, err)
		}
		db.Logger.Info("Executed password change script", "script", script, "output", string(output))

		return dbplugin.UpdateUserResponse{}, nil
	}

	return dbplugin.UpdateUserResponse{}, fmt.Errorf("only 'password' credential type is supported. Got %s ", req.CredentialType)
}

func (db *cmd) DeleteUser(ctx context.Context, req dbplugin.DeleteUserRequest) (dbplugin.DeleteUserResponse, error) {
	db.Logger.Info("DeleteUser", "username", req.Username)
	db.Logger.Info("DeleteUser", "statements_commands", req.Statements.Commands)

	// Assemble delete statements into a single script
	script := strings.Join(req.Statements.Commands, "\n")
	params := map[string]string{
		"name":     req.Username,
		"username": req.Username,
	}
	renderedScript := replaceVars(params, script)

	// Execute the script
	cmd := exec.Command("/bin/bash", "-c", renderedScript)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return dbplugin.DeleteUserResponse{}, fmt.Errorf("failed to execute delete script: %s, error: %w", script, err)
	}
	db.Logger.Info("Executed delete script", "script", script, "output", string(output))

	return dbplugin.DeleteUserResponse{}, nil
}

func (db *cmd) Type() (string, error) {
	return "cmd", nil
}

func (db *cmd) Close() error {
	db.Logger.Info("Close", "closing", "cmd")
	return nil
}

func replaceVars(m map[string]string, tpl string) string {
	if m == nil || len(m) <= 0 {
		return tpl
	}

	for k, v := range m {
		tpl = strings.ReplaceAll(tpl, fmt.Sprintf("{{%s}}", k), v)
	}
	return tpl
}
