# Vault Plugin for Universal Credential Management - `vault-plugin-database-cmd`
This repository contains a **custom Vault database plugin** designed to manage any type credential by executing system commands.

The ecosystem of applications and APIs that Vault needs to support is always growing and becoming more diverse, and DevOps engineers often find themselves having to create bespoke overprivileged pipelines to rotate secrets and store them in Vault KV for third-party party consumption.

By moving those rotation jobs into Vault you get:
 - **Secure Code Execution**: Utilizes Docker Rootless with gVisor for enhanced security.
 - **Controlled Access**: Only Vault administrators can enable, disable or update the plugin.
 - **Secure Secret Introduction**: Instant support by Vault Agent, Vault secrets operator, etc
 - **Automated Rotation Workflows**: Instant support for periodic password rotation or on a schedule
 - **Just-in-time Password Delivery**: Unique passwords and usernames can be generated on-demand, according to your policies.
 - **Root Credential Rotation**: Rotation of root credential to a secure and unknown value

> **Note:** For static usernames, if rotation is enabled, the plugin will return the previous password if it hasn't been rotated yet.

> [!CAUTION]
> **This plugin is dangerous if misconfigured!** Any 3rd-party plugin can be dangerous and should be run in rootless docker with gVisor. Due to the nature of what this plugin allows, its configuration and setup must be handled with care. It is your responsibility to ensure that the commands you run are safe and do not expose your system to security risks.

## What can I do with this plugin?
- Generate dynamic credentials for anything by executing custom commands for credential management
- Migrate your current jobs/scripts into HashiCorp Vault by embedding the scripts and any other  binaries into the Dockerfile!

## Preparation
This plugin has been tested using [Rootless Docker](https://docs.docker.com/engine/security/rootless/) running with [gVisor](https://gvisor.dev/). You can see examples of how to configure your Vault nodes in the following scripts - [setup_as_root.sh](setup_as_root.sh) and [setup_as_user.sh](setup_as_user.sh).

> **Note:**  For Rootless Docker, please ensure the `DOCKER_HOST` environment variable is configured to the user socket, e.g., `unix:///run/user/1000/docker.sock`.

For detailed registration and usage instructions, please check the `test` section in the [`Makefile`](Makefile).


1. Register plugin runtime to work with Rootless Docker and gVisor:
    ```sh
	vault plugin runtime register -type=container -rootless=true -oci_runtime=runsc runsc
    ```

2. Register the plugin as a `database` plugin, using semantic versioning:
    ```sh
	vault plugin register \
		-sha256=$(SHA256) \
		-oci_image=$(DOCKER_IMAGE) \
		-runtime=runsc \
		-version=$(DOCKER_IMAGE_TAG) \
		database $(PLUGIN_NAME)
    ```

## Using the custom database plugin
1. Enable the database backend:
    ```sh
    vault secrets enable -path=database-cmd database
    ```

2. Write the plugin root configuration. Username and password are mandatory for *password* credential type. Root rotation workflow uses the `UpdateUser` go function call, like all other user password rotations:

    ```sh
    vault write database-cmd/config/my-database \
		plugin_name="$(PLUGIN_NAME)" \
		plugin_version="$(DOCKER_IMAGE_TAG)" \
		allowed_roles="*" \
		username="mandatory" \
		password="mandatory" \
		custom_field="anything" \
		root_rotation_statements="echo 'Root rotation statements'" \
		root_rotation_statements="echo 'Second line {{root_custom_field}}'"
    ```
> **Note 1:** To add more lines to the bash script, repeat the statement parameter.
> **Note 2:** To add further custom fields, you'll need to edit the code. If you'd like to use these in your scripts, then all you need to do is use something like `{{root_custom_field}}` in your script.



3. Create roles to manage credentials, for example:
    *   New user on demand:
    ```sh
    vault write database-cmd/roles/dynamic-role \
		db_name=my-database \
		creation_statements="echo 'Dynamic creation statements'" \
		creation_statements="ping -c3 www.google.com" \
        default_ttl="1h" \
        max_ttl="24h"
    ```

    * Static user with a password rotation schedule. The same password will be returned until it reaches the rotation time or it's forced:
    ```sh
    vault write database-cmd/static-roles/static-role \
		db_name=my-database \
		credential_type="password" \
		username="static-username" \
		rotation_window="1h" \
		self_managed_password="true" \
		rotation_schedule="0 * * * SAT" \
		rotation_statements="echo 'Rotate static'"
    ```


## Building your own
1. Fork the repository.
2. Edit the code.
3. Run `vagrant up`.
4. Go to `/vagrant` and run `make release`.

### Testing
1. Ensure the container image exists with `make build-container`
2. Modify the `Makefile` `test` target with the Vault commands you need
3. Run `make start` to launch Vault in development mode
4. Run `make test` to register the plugin, enable it and run the defined test commands

## License

This project is licensed under the Mozilla Public License 2.0. See the [LICENSE.md](LICENSE.md) file for details.

## Acknowledgements

Special thanks to my customers and the UK & Ireland Solutions Engineering Team for their valuable input.
