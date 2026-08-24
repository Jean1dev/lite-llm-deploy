FROM docker.litellm.ai/berriai/litellm-database:latest

COPY config.yaml /app/config.yaml

EXPOSE 4000

# Heroku defines PORT only when the dyno starts. Keep its expansion at runtime
# and retain LiteLLM's production entrypoint (including database migrations).
ENTRYPOINT []
CMD ["/bin/sh", "-c", "exec /app/docker/prod_entrypoint.sh --config /app/config.yaml --port \"${PORT:-4000}\""]
