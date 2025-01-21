package cmd

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	dbplugin "github.com/hashicorp/vault/sdk/database/dbplugin/v5"
	"github.com/hashicorp/vault/sdk/database/helper/credsutil"
	"github.com/mitchellh/mapstructure"
)

const (
	defaultTimeout = 20000 * time.Millisecond
	maxKeyLength   = 64
	scriptCommand  = "/bin/bash"
)

var _ dbplugin.Database = (*cmd)(nil)

type cmd struct {
	Logger hclog.Logger

	credsutil.CredentialsProducer

	RawConfig map[string]interface{}

	RootUsername    string `json:"username"`
	RootPassword    string `json:"password"`
	RootCertificate string `json:"certificate"`

	CustomField string `json:"custom_field"`

	AllParams map[string]string
}

// ToMap converts the exported parameters in RawConfig to a map[string]string.
func (db *cmd) ToMap() map[string]string {
	result := make(map[string]string)
	for _, param := range exported_params {
		if value, ok := db.RawConfig[param]; ok {
			if strValue, ok := value.(string); ok {
				result[param] = strValue
			}
		}
	}
	return result
}

func (db *cmd) convertParams() {
	db.AllParams = db.ToMap()
}

var (
	// exported parameters
	exported_params = []string{
		"RootUsername", "RootPassword", "RootCertificate", "CustomField",
	}
)

func New() (interface{}, error) {
	db := newCmd()

	// This middleware isn't strictly required, but highly recommended to prevent accidentally exposing
	// values such as passwords in error messages.
	dbType := dbplugin.NewDatabaseErrorSanitizerMiddleware(db, db.secretValues)
	return dbType, nil
}

func newCmd() *cmd {
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

	db.convertParams()

	resp := dbplugin.InitializeResponse{
		Config: req.Config,
	}

	resp.SetSupportedCredentialTypes([]dbplugin.CredentialType{
		dbplugin.CredentialTypePassword,
	})

	return resp, nil
}

func (db *cmd) NewUser(ctx context.Context, req dbplugin.NewUserRequest) (dbplugin.NewUserResponse, error) {
	db.Logger.Info("NewUser", "Config custom_field", db.CustomField)
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

		if err := db.executeScript(script, params); err != nil {
			return dbplugin.NewUserResponse{}, fmt.Errorf("failed to execute creation script: %w", err)
		}

		return dbplugin.NewUserResponse{
			Username: username,
		}, nil

	}

	return dbplugin.NewUserResponse{}, fmt.Errorf("only 'password' credential type is supported. Got %s ", req.CredentialType)
}

func (db *cmd) UpdateUser(ctx context.Context, req dbplugin.UpdateUserRequest) (dbplugin.UpdateUserResponse, error) {
	db.Logger.Info("UpdateUser", "Config custom_field", db.CustomField)

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

		if err := db.executeScript(script, params); err != nil {
			return dbplugin.UpdateUserResponse{}, fmt.Errorf("failed to execute password change script: %w", err)
		}

		return dbplugin.UpdateUserResponse{}, nil
	}

	return dbplugin.UpdateUserResponse{}, fmt.Errorf("only 'password' credential type is supported. Got %s ", req.CredentialType)
}

func (db *cmd) DeleteUser(ctx context.Context, req dbplugin.DeleteUserRequest) (dbplugin.DeleteUserResponse, error) {
	db.Logger.Info("DeleteUser", "Config custom_field", db.CustomField)
	db.Logger.Info("DeleteUser", "username", req.Username)
	db.Logger.Info("DeleteUser", "statements_commands", req.Statements.Commands)

	// Assemble delete statements into a single script
	script := strings.Join(req.Statements.Commands, "\n")
	params := map[string]string{
		"name":     req.Username,
		"username": req.Username,
	}

	if err := db.executeScript(script, params); err != nil {
		return dbplugin.DeleteUserResponse{}, fmt.Errorf("failed to execute delete script: %w", err)
	}

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

func (db *cmd) executeScript(script string, params map[string]string) error {

	//to add all the root config params to the script
	for k, v := range db.AllParams {
		params[k] = v
	}

	renderedScript := replaceVars(params, script)

	cmd := exec.Command(scriptCommand, "-c", renderedScript)
	output, err := cmd.CombinedOutput()
	if err != nil {
		db.Logger.Error("Failed to execute script", "script", script, "output", string(output), "error", err)
		return fmt.Errorf("script execution failed: %s, error: %w", script, err)
	}
	db.Logger.Info("Executed script", "script", script, "output", string(output))
	return nil
}
