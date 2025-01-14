# FROM gcr.io/distroless/static-debian12 also works.
FROM alpine:3.21
RUN apk add --no-cache bash
# Copy the plugin binary
USER nonroot
COPY vault/plugins/vault-plugin-database-cmd /bin/vault-plugin-database-cmd
ENTRYPOINT [ "/bin/vault-plugin-database-cmd" ]