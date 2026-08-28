build-app:
	docker compose build app

up-dev: build-app
	docker compose up

token-dev:
	docker run --rm livekit/livekit-cli token create \
		--api-key devkey --api-secret secret \
		--join --room test --identity alice --valid-for 24h
