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

// ToMap converts the RawConfig field of the cmd struct into a map of strings.
// It iterates over the exportedParams slice and checks if each parameter exists
// in the RawConfig map. If the parameter exists and its value is a string, it adds
// the parameter to the result map with a "root_" prefix.
//
// Returns:
//
//	A map[string]string containing the parameters from RawConfig with a "root_" prefix.
func (db *cmd) ToMap() map[string]string {
	result := make(map[string]string)
	for _, param := range exportedParams {
		if value, ok := db.RawConfig[param]; ok {
			if strValue, ok := value.(string); ok {
				result["root_"+param] = strValue
			}
		}
	}
	return result
}

var (
	// root configuration parameters to be used in statements
	exportedParams = []string{
		"custom_field", "username", "password", "certificate",
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

	db.AllParams = db.ToMap()

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

	if req.CredentialType != dbplugin.CredentialTypePassword {
		return dbplugin.NewUserResponse{}, fmt.Errorf("only 'password' credential type is supported. Got %s ", req.CredentialType)
	}

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

func (db *cmd) UpdateUser(ctx context.Context, req dbplugin.UpdateUserRequest) (dbplugin.UpdateUserResponse, error) {
	if req.CredentialType != dbplugin.CredentialTypePassword {
		return dbplugin.UpdateUserResponse{}, fmt.Errorf("only 'password' credential type is supported. Got %s", req.CredentialType)
	}

	if req.Username == "" {
		return dbplugin.UpdateUserResponse{}, fmt.Errorf("username is required")
	}

	if req.Password == nil || req.Password.NewPassword == "" {
		return dbplugin.UpdateUserResponse{}, fmt.Errorf("new password is required")
	}

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

	// checking if the password rotated is for the root config user
	if db.AllParams["root_username"] == req.Username {
		db.AllParams["root_password"] = req.Password.NewPassword
	}

	return dbplugin.UpdateUserResponse{}, nil
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
