build-app:
	docker compose build livekit-app

up-dev: build-app
	USERS_FILE_PATH=./users.json LIVEKIT_SERVER_DEV_FLAG=--dev ENV_FILE=.env.dev docker compose up

up-prod:
	ENV_FILE=.env docker compose up -d

token-dev:
	docker run --rm livekit/livekit-cli token create \
		--api-key devkey --api-secret secret \
		--join --room test --identity alice --valid-for 24h

generate-keys:
	docker run --rm livekit/livekit-server generate-keys