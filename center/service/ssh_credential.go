package service

import (
	"fmt"

	"github.com/ccfos/nightingale/v6/models"
	"github.com/ccfos/nightingale/v6/pkg/ctx"
	"github.com/pkg/errors"
)

// StoreSSHCredential stores SSH credential in configs table with encryption
func StoreSSHCredential(ctx *ctx.Context, targetIdent, authMethod, credential, username string) error {
	var credentialRef string
	if authMethod == "key" {
		credentialRef = "ssh_key_target_" + targetIdent
	} else if authMethod == "password" {
		credentialRef = "ssh_pass_target_" + targetIdent
	} else {
		return errors.New("invalid auth_method, must be 'key' or 'password'")
	}

	// Check if credential config already exists
	configs, err := models.ConfigsSelectByCkey(ctx, credentialRef)
	if err != nil {
		return errors.Wrap(err, "failed to check existing credential config")
	}

	// Prepare config object
	config := models.Configs{
		Ckey:      credentialRef,
		Cval:      credential,
		Note:      fmt.Sprintf("SSH credential for target %s", targetIdent),
		External:  models.ConfigExternal,
		Encrypted: models.ConfigEncrypted,
		CreateBy:  username,
		UpdateBy:  username,
	}

	// Create or update credential config
	if len(configs) == 0 {
		// Create new config
		err = models.ConfigsUserVariableInsert(ctx, config)
		if err != nil {
			return errors.Wrap(err, "failed to insert credential config")
		}
	} else {
		// Update existing config
		config.Id = configs[0].Id
		err = models.ConfigsUserVariableUpdate(ctx, config)
		if err != nil {
			return errors.Wrap(err, "failed to update credential config")
		}
	}

	return nil
}

// GetSSHCredential retrieves and decrypts SSH credential from configs table
func GetSSHCredential(ctx *ctx.Context, targetIdent, authMethod string) (string, error) {
	var credentialRef string
	if authMethod == "key" {
		credentialRef = "ssh_key_target_" + targetIdent
	} else if authMethod == "password" {
		credentialRef = "ssh_pass_target_" + targetIdent
	} else {
		return "", errors.New("invalid auth_method, must be 'key' or 'password'")
	}

	// Get RSA private key and password for decryption
	privateKeyVal, err := models.ConfigsGet(ctx, models.RSA_PRIVATE_KEY)
	if err != nil {
		return "", errors.Wrap(err, "failed to get RSA private key")
	}

	passwordVal, err := models.ConfigsGet(ctx, models.RSA_PASSWORD)
	if err != nil {
		return "", errors.Wrap(err, "failed to get RSA password")
	}

	// Get decrypted configs map
	decryptMap, err := models.ConfigUserVariableGetDecryptMap(ctx, []byte(privateKeyVal), passwordVal)
	if err != nil {
		return "", errors.Wrap(err, "failed to decrypt configs")
	}

	// Get credential from decrypted map
	credential, exists := decryptMap[credentialRef]
	if !exists {
		return "", errors.New("credential not found")
	}

	return credential, nil
}

// DeleteSSHCredential deletes SSH credential from configs table
func DeleteSSHCredential(ctx *ctx.Context, targetIdent string) error {
	// Try to delete both possible credential keys
	credKeys := []string{
		"ssh_key_target_" + targetIdent,
		"ssh_pass_target_" + targetIdent,
	}

	for _, credKey := range credKeys {
		configs, err := models.ConfigsSelectByCkey(ctx, credKey)
		if err != nil {
			return errors.Wrapf(err, "failed to select config by ckey %s", credKey)
		}

		if len(configs) > 0 {
			ids := make([]int64, len(configs))
			for i, config := range configs {
				ids[i] = config.Id
			}
			err = models.ConfigsDel(ctx, ids)
			if err != nil {
				return errors.Wrapf(err, "failed to delete config with ckey %s", credKey)
			}
		}
	}

	return nil
}