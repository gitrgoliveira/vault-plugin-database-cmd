FROM gcr.io/distroless/static-debian12
COPY vault/plugins/vault-plugin-database-cmd /bin/vault-plugin-database-cmd
ENTRYPOINT [ "/bin/vault-plugin-database-cmd" ]